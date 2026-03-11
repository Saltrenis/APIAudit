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
var (
	flaskRouteRe = regexp.MustCompile(
		`@\w+\.route\s*\(\s*["']([^"']+)["'](?:[^)]*methods\s*=\s*\[([^\]]+)\])?`,
	)
	flaskBlueprintRe = regexp.MustCompile(
		`Blueprint\s*\(\s*['"][^'"]+['"]\s*,\s*__name__[^)]*url_prefix\s*=\s*['"]([^'"]+)['"]`,
	)
	flaskDefRe = regexp.MustCompile(`def\s+(\w+)\s*\(`)
)

// Scan implements Scanner.
func (s *FlaskScanner) Scan(dir string) ([]Route, error) {
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

		found, ferr := scanFlaskFile(path)
		if ferr != nil {
			return nil
		}
		routes = append(routes, found...)
		return nil
	})

	return routes, err
}

func scanFlaskFile(path string) ([]Route, error) {
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

	// Detect blueprint prefix for this file.
	prefix := ""
	for _, line := range lines {
		if m := flaskBlueprintRe.FindStringSubmatch(line); m != nil {
			prefix = m[1]
			break
		}
	}

	var routes []Route
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		m := flaskRouteRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		routePath := joinPaths(prefix, m[1])

		// Parse methods list (default GET).
		methods := []string{"GET"}
		if m[2] != "" {
			methods = parseFlaskMethods(m[2])
		}

		// Find handler function.
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
