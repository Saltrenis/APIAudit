package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FlaskScanner extracts routes from Python Flask projects.
type FlaskScanner struct{}

// Name implements Scanner.
func (s *FlaskScanner) Name() string { return "flask" }

// Flask route patterns:
//
//	@app.route("/path", methods=["GET", "POST"])
//	@blueprint.route("/path")
//	bp = Blueprint('name', __name__, url_prefix='/prefix')
//	app.register_blueprint(bp, url_prefix='/prefix')
var (
	// Captures decorator variable name and route path (and optional methods).
	flaskRouteRe = regexp.MustCompile(
		`@(\w+)\.route\s*\(\s*["']([^"']+)["'](?:[^)]*methods\s*=\s*\[([^\]]+)\])?`,
	)

	// Matches Blueprint constructor when url_prefix is passed inline:
	//   auth = Blueprint('auth', __name__, url_prefix='/auth')
	flaskBlueprintCtorRe = regexp.MustCompile(
		`(\w+)\s*=\s*Blueprint\s*\(\s*['"](\w+)['"]\s*,\s*__name__[^)]*url_prefix\s*=\s*['"]([^'"]+)['"]`,
	)

	// Matches Blueprint constructor without url_prefix — used to map variable name to blueprint name:
	//   auth = Blueprint('auth', __name__)
	flaskBlueprintNameRe = regexp.MustCompile(
		`(\w+)\s*=\s*Blueprint\s*\(\s*['"](\w+)['"]\s*,\s*__name__`,
	)

	// Matches register_blueprint calls with url_prefix:
	//   app.register_blueprint(auth_blueprint, url_prefix='/auth')
	//   app.register_blueprint(main, url_prefix='/api/v1')
	flaskRegisterBlueprintRe = regexp.MustCompile(
		`register_blueprint\s*\(\s*(\w+)\s*(?:,\s*url_prefix\s*=\s*['"]([^'"]+)['"])?`,
	)

	flaskDefRe = regexp.MustCompile(`def\s+(\w+)\s*\(`)
)

// Scan implements Scanner.
func (s *FlaskScanner) Scan(dir string) ([]Route, error) {
	// First pass: collect all blueprint prefixes project-wide by reading every
	// Python file for Blueprint constructor and register_blueprint calls.
	prefixByVarName, prefixByBPName := collectBlueprintPrefixes(dir)

	var routes []Route

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}
		if !isPythonFile(path) {
			return nil
		}

		found, ferr := scanFlaskFile(path, prefixByVarName, prefixByBPName)
		if ferr != nil {
			return nil
		}
		routes = append(routes, found...)
		return nil
	})

	return routes, err
}

// collectBlueprintPrefixes walks the project and returns two maps:
//
//   - prefixByVarName: Python variable name → url_prefix
//     e.g. "auth_blueprint" → "/auth"  (from register_blueprint calls)
//
//   - prefixByBPName: Blueprint logical name → url_prefix
//     e.g. "auth" → "/auth"
//
// Two passes are performed so that register_blueprint calls (which reference
// an import alias like auth_blueprint) can be correlated back to the
// blueprint's logical name (the first string arg to Blueprint()).
func collectBlueprintPrefixes(dir string) (map[string]string, map[string]string) {
	prefixByVarName := make(map[string]string)
	prefixByBPName := make(map[string]string)
	// varToBPName maps a Python variable name to the blueprint's logical name
	// (first arg in Blueprint(...)). Built in pass 1.
	varToBPName := make(map[string]string)

	collectFileLines := func(path string, visit func(line string)) {
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			visit(strings.TrimSpace(sc.Text()))
		}
	}

	walkPythonFiles := func(visit func(path string)) {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() && shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			if isPythonFile(path) {
				visit(path)
			}
			return nil
		})
	}

	// Pass 1: collect Blueprint() constructor definitions to build
	// varToBPName (and handle constructors that include url_prefix directly).
	walkPythonFiles(func(path string) {
		collectFileLines(path, func(line string) {
			if m := flaskBlueprintCtorRe.FindStringSubmatch(line); m != nil {
				varName, bpName, prefix := m[1], m[2], m[3]
				prefixByVarName[varName] = prefix
				prefixByBPName[bpName] = prefix
				varToBPName[varName] = bpName
				return
			}
			if m := flaskBlueprintNameRe.FindStringSubmatch(line); m != nil {
				varToBPName[m[1]] = m[2]
			}
		})
	})

	// Pass 2: collect register_blueprint calls, recording prefixes under both
	// the import-alias variable name AND the blueprint's logical name.
	walkPythonFiles(func(path string) {
		collectFileLines(path, func(line string) {
			m := flaskRegisterBlueprintRe.FindStringSubmatch(line)
			if m == nil || m[2] == "" {
				return
			}
			varName, prefix := m[1], m[2]
			prefixByVarName[varName] = prefix

			// Propagate to the logical name so route files that import the
			// original variable (not the alias) can still find the prefix.
			if bpName, ok := varToBPName[varName]; ok {
				prefixByBPName[bpName] = prefix
			} else {
				// The variable may be an import alias (e.g. auth_blueprint for auth).
				// Try stripping common suffixes to find the original variable name.
				for _, suffix := range []string{"_blueprint", "_bp"} {
					trimmed := strings.TrimSuffix(varName, suffix)
					if trimmed != varName {
						if bpName, ok := varToBPName[trimmed]; ok {
							prefixByBPName[bpName] = prefix
							break
						}
						// trimmed itself might be the logical name.
						prefixByBPName[trimmed] = prefix
						break
					}
				}
				// Also store under varName as fallback.
				prefixByBPName[varName] = prefix
			}
		})
	})

	return prefixByVarName, prefixByBPName
}

// filePrefix resolves the url_prefix for a file by examining its lines and
// the project-wide blueprint maps. The decorator variable in @varName.route()
// is the key used to look up the prefix.
//
// Resolution order per Blueprint variable used in this file:
//  1. url_prefix in a Blueprint() constructor in this same file
//  2. Variable name looked up in prefixByVarName (from register_blueprint calls)
//  3. Blueprint logical name (first arg to Blueprint()) looked up in prefixByBPName
func filePrefix(lines []string, prefixByVarName, prefixByBPName map[string]string) string {
	// Build a local map: variable name → prefix, using only definitions in this file.
	localVarPrefix := make(map[string]string)
	localVarBPName := make(map[string]string)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Inline ctor with url_prefix wins immediately for that variable.
		if m := flaskBlueprintCtorRe.FindStringSubmatch(trimmed); m != nil {
			localVarPrefix[m[1]] = m[3]
			continue
		}

		// Ctor without url_prefix — record variable → blueprint logical name.
		if m := flaskBlueprintNameRe.FindStringSubmatch(trimmed); m != nil {
			localVarBPName[m[1]] = m[2]
		}
	}

	// Collect all decorator variable names used in route decorators in this file.
	// A file often uses a single blueprint variable throughout, but we build a
	// per-variable prefix lookup to handle edge cases.
	prefixForDecoVar := func(decorVar string) string {
		// Inline definition with url_prefix in this file.
		if p, ok := localVarPrefix[decorVar]; ok {
			return p
		}
		// Project-wide register_blueprint map (uses the import alias).
		if p, ok := prefixByVarName[decorVar]; ok {
			return p
		}
		// Direct match: decorator variable IS the blueprint logical name
		// (common in files that import the blueprint with `from . import auth`).
		if p, ok := prefixByBPName[decorVar]; ok {
			return p
		}
		// Blueprint logical name → prefix via file-local map.
		if bpName, ok := localVarBPName[decorVar]; ok {
			if p, ok := prefixByBPName[bpName]; ok {
				return p
			}
			if p, ok := prefixByVarName[bpName]; ok {
				return p
			}
		}
		return ""
	}

	// Return the first non-empty prefix found among decorator variables in this file.
	for _, line := range lines {
		if m := flaskRouteRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			decorVar := m[1]
			if p := prefixForDecoVar(decorVar); p != "" {
				return p
			}
		}
	}
	return ""
}

func scanFlaskFile(path string, prefixByVarName, prefixByBPName map[string]string) ([]Route, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	prefix := filePrefix(lines, prefixByVarName, prefixByBPName)

	var routes []Route
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		m := flaskRouteRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		// m[1]=decoratorVar, m[2]=routePath, m[3]=methodsList
		routePath := joinPaths(prefix, m[2])

		// Parse methods list (default GET).
		methods := []string{"GET"}
		if m[3] != "" {
			methods = parseFlaskMethods(m[3])
		}

		// Find handler function name in the next few lines.
		handler := ""
		for j := i + 1; j < len(lines) && j < i+4; j++ {
			if mm := flaskDefRe.FindStringSubmatch(strings.TrimSpace(lines[j])); mm != nil {
				handler = mm[1]
				break
			}
		}

		for _, method := range methods {
			routes = append(routes, Route{
				Method:  method,
				Path:    routePath,
				Handler: handler,
				File:    path,
				Line:    lineNum,
			})
		}
	}

	return routes, nil
}

// parseFlaskMethods parses the string inside methods=[...] into individual method names.
func parseFlaskMethods(raw string) []string {
	var methods []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `'"`)
		part = strings.ToUpper(part)
		if part != "" {
			methods = append(methods, part)
		}
	}
	return methods
}
