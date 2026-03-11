package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// StdlibScanner extracts routes from Go projects using net/http stdlib or
// custom mux implementations (e.g., ardanlabs web.Mux pattern).
type StdlibScanner struct{}

func (s *StdlibScanner) Name() string { return "stdlib" }

// Patterns we recognize:
//
// Standard library:
//
//	http.HandleFunc("/path", handler)
//	http.Handle("/path", handler)
//	mux.HandleFunc("/path", handler)
//	mux.Handle("/path", handler)
//	r.HandleFunc("/path", handler)
//
// Go 1.22+ mux patterns:
//
//	mux.HandleFunc("GET /path", handler)
//	http.HandleFunc("POST /path", handler)
//
// Custom mux (ardanlabs-style):
//
//	app.HandlerFunc(http.MethodGet, version, "/path", handler, middleware...)
//	app.Handle(http.MethodPost, "v1", "/path", handler)
var (
	// http.HandleFunc("/path", handler) or mux.HandleFunc("/path", handler)
	stdHandleFuncRe = regexp.MustCompile(
		`\w+\.HandleFunc\s*\(\s*"([^"]+)"\s*,\s*(\w[\w.]*)`,
	)
	// http.Handle("/path", handler)
	stdHandleRe = regexp.MustCompile(
		`\w+\.Handle\s*\(\s*"([^"]+)"\s*,\s*(\w[\w.]*)`,
	)
	// Go 1.22+: mux.HandleFunc("GET /path", handler)
	stdMethodHandleFuncRe = regexp.MustCompile(
		`\w+\.HandleFunc\s*\(\s*"(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+([^"]+)"\s*,\s*(\w[\w.]*)`,
	)
	// Ardanlabs/custom: app.HandlerFunc(http.MethodGet, "v1", "/path", handler, mw...)
	// Also matches: app.HandlerFunc(http.MethodGet, version, "/path", handler)
	ardanlabsRe = regexp.MustCompile(
		`\w+\.(?:HandlerFunc|Handle)\s*\(\s*http\.Method(\w+)\s*,\s*(?:(\w+)|"([^"]*)")\s*,\s*"([^"]+)"\s*,\s*(\w[\w.]*)`,
	)
	// http.MethodGet, http.MethodPost, etc. used as constants
	httpMethodMap = map[string]string{
		"Get":     "GET",
		"Post":    "POST",
		"Put":     "PUT",
		"Delete":  "DELETE",
		"Patch":   "PATCH",
		"Head":    "HEAD",
		"Options": "OPTIONS",
	}
)

func (s *StdlibScanner) Scan(dir string) ([]Route, error) {
	var routes []Route

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}
		if !isGoFile(path) {
			return nil
		}

		found, ferr := scanStdlibFile(path)
		if ferr != nil {
			return nil
		}
		routes = append(routes, found...)
		return nil
	})

	// Post-process: for routes without swagger detected in the route file,
	// check sibling files in the same package for handler functions with annotations.
	for i := range routes {
		if routes[i].HasSwagger {
			continue
		}
		routes[i].HasSwagger = checkHandlerSwagger(routes[i].File, routes[i].Handler)
	}

	return routes, err
}

// checkHandlerSwagger looks for swagger annotations on a handler method
// in sibling Go files within the same directory as routeFile.
func checkHandlerSwagger(routeFile, handler string) bool {
	// Extract the method name from patterns like "api.create" -> "create".
	methodName := handler
	if idx := strings.LastIndex(handler, "."); idx >= 0 {
		methodName = handler[idx+1:]
	}
	if methodName == "" {
		return false
	}

	dir := filepath.Dir(routeFile)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	funcRe := regexp.MustCompile(`func\s+\([^)]+\)\s+` + regexp.QuoteMeta(methodName) + `\s*\(`)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		fpath := filepath.Join(dir, e.Name())
		// Skip the route file itself (already checked).
		if fpath == routeFile {
			continue
		}

		data, err := os.ReadFile(fpath)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if funcRe.MatchString(line) {
				// Check lines above this function for swagger annotations.
				if linesHaveSwagger(lines, i, 20) {
					return true
				}
			}
		}
	}
	return false
}

func scanStdlibFile(path string) ([]Route, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		routes []Route
		lines  []string
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// First pass: find version variable assignments like `const version = "v1"`.
	versionVars := findVersionVars(lines)

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Try ardanlabs pattern first (most specific).
		if m := ardanlabsRe.FindStringSubmatch(trimmed); m != nil {
			method := httpMethodMap[m[1]]
			if method == "" {
				method = strings.ToUpper(m[1])
			}
			// m[2] = variable name for version, m[3] = literal version string.
			version := m[3]
			if version == "" && m[2] != "" {
				if v, ok := versionVars[m[2]]; ok {
					version = v
				} else {
					version = m[2]
				}
			}
			routePath := m[4]
			if version != "" {
				routePath = "/" + strings.Trim(version, "/") + routePath
			}
			handler := m[5]

			routes = append(routes, Route{
				Method:     method,
				Path:       routePath,
				Handler:    handler,
				File:       path,
				Line:       lineNum,
				HasSwagger: linesHaveSwagger(lines, i, 20),
			})
			continue
		}

		// Go 1.22+ method pattern: HandleFunc("GET /path", handler).
		if m := stdMethodHandleFuncRe.FindStringSubmatch(trimmed); m != nil {
			routes = append(routes, Route{
				Method:     strings.ToUpper(m[1]),
				Path:       m[2],
				Handler:    m[3],
				File:       path,
				Line:       lineNum,
				HasSwagger: linesHaveSwagger(lines, i, 20),
			})
			continue
		}

		// Standard HandleFunc("/path", handler).
		if m := stdHandleFuncRe.FindStringSubmatch(trimmed); m != nil {
			pathStr := m[1]
			handler := m[2]
			method := "ANY"
			// If path contains method prefix (Go 1.22+ style without explicit match above).
			if parts := strings.SplitN(pathStr, " ", 2); len(parts) == 2 {
				method = strings.ToUpper(parts[0])
				pathStr = parts[1]
			}

			routes = append(routes, Route{
				Method:     method,
				Path:       pathStr,
				Handler:    handler,
				File:       path,
				Line:       lineNum,
				HasSwagger: linesHaveSwagger(lines, i, 20),
			})
			continue
		}

		// Standard Handle("/path", handler).
		if m := stdHandleRe.FindStringSubmatch(trimmed); m != nil {
			routes = append(routes, Route{
				Method:     "ANY",
				Path:       m[1],
				Handler:    m[2],
				File:       path,
				Line:       lineNum,
				HasSwagger: linesHaveSwagger(lines, i, 20),
			})
			continue
		}
	}

	return routes, nil
}

// findVersionVars extracts const/var assignments like `const version = "v1"`.
func findVersionVars(lines []string) map[string]string {
	vars := make(map[string]string)
	re := regexp.MustCompile(`(?:const|var)\s+(\w+)\s*=\s*"([^"]*)"`)
	for _, line := range lines {
		if m := re.FindStringSubmatch(line); m != nil {
			vars[m[1]] = m[2]
		}
	}
	return vars
}
