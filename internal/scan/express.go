package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ExpressScanner extracts routes from Node.js Express projects.
type ExpressScanner struct{}

// Name implements Scanner.
func (s *ExpressScanner) Name() string { return "express" }

// Express route patterns:
//
//	app.get('/path', handler)
//	router.post('/path', mw, handler)
//	app.use('/prefix', router)
var (
	expressRouteRe = regexp.MustCompile(
		`(?i)(?:app|router)\s*\.\s*(get|post|put|delete|patch|head|options|all)\s*\(\s*['"]([^'"]+)['"]\s*,\s*([^)]+)\)`,
	)
	expressUseRe = regexp.MustCompile(
		`(?i)(?:app|router)\s*\.use\s*\(\s*['"]([^'"]+)['"]\s*,`,
	)
	jsSwaggerRe = regexp.MustCompile(`@swagger|@openapi|swagger-jsdoc`)
)

// Scan implements Scanner.
func (s *ExpressScanner) Scan(dir string) ([]Route, error) {
	var routes []Route

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}
		if !isJSFile(path) {
			return nil
		}

		found, ferr := scanExpressFile(path)
		if ferr != nil {
			return nil
		}
		routes = append(routes, found...)
		return nil
	})

	return routes, err
}

func scanExpressFile(path string) ([]Route, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		routes   []Route
		lines    []string
		prefixes []string
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Track use-prefixes (app.use('/api', router) style).
		if m := expressUseRe.FindStringSubmatch(trimmed); m != nil {
			prefixes = append(prefixes, m[1])
		}

		m := expressRouteRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		method := strings.ToUpper(m[1])
		routePath := m[2]
		handlerRaw := strings.TrimSpace(m[3])
		handler := lastJSHandler(handlerRaw)

		// Apply any detected prefix.
		prefix := ""
		if len(prefixes) > 0 {
			prefix = prefixes[len(prefixes)-1]
		}
		fullPath := joinPaths(prefix, routePath)

		hasSwagger := jsLinesHaveSwagger(lines, i, 20)

		routes = append(routes, Route{
			Method:     method,
			Path:       fullPath,
			Handler:    handler,
			File:       path,
			Line:       lineNum,
			HasSwagger: hasSwagger,
		})
	}

	return routes, nil
}

// lastJSHandler picks the last comma-separated handler name (strips async keyword, etc.).
func lastJSHandler(raw string) string {
	parts := strings.Split(raw, ",")
	last := strings.TrimSpace(parts[len(parts)-1])
	last = strings.TrimRight(last, ")")
	last = strings.TrimSpace(last)
	// Strip "async" keyword.
	last = strings.TrimPrefix(last, "async ")
	last = strings.TrimSpace(last)
	return last
}

// jsLinesHaveSwagger checks for JSDoc swagger annotations near lineIdx.
func jsLinesHaveSwagger(lines []string, lineIdx, lookback int) bool {
	start := lineIdx - lookback
	if start < 0 {
		start = 0
	}
	for _, l := range lines[start:lineIdx] {
		if jsSwaggerRe.MatchString(l) {
			return true
		}
	}
	return false
}

// isJSFile reports whether path is a JavaScript or TypeScript source file.
func isJSFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".js" || ext == ".ts" || ext == ".mjs" || ext == ".cjs"
}
