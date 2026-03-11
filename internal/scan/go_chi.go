package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ChiScanner extracts routes from Go projects using the go-chi/chi router.
type ChiScanner struct{}

// Name implements Scanner.
func (s *ChiScanner) Name() string { return "chi" }

// Chi route patterns:
//
//	r.Get("/path", handler)
//	r.Post("/path", handler)
//	r.Route("/prefix", func(r chi.Router) { ... })
//	r.Group(func(r chi.Router) { ... })
//	r.With(mw).Get("/path", handler)
var (
	chiRouteRe = regexp.MustCompile(
		`(?i)\.\s*(Get|Post|Put|Delete|Patch|Head|Options)\s*\(\s*"([^"]+)"\s*,\s*([^)]+)\)`,
	)
	chiRouteBlockRe = regexp.MustCompile(
		`\.Route\s*\(\s*"([^"]+)"\s*,`,
	)
	chiGroupRe = regexp.MustCompile(
		`\.Group\s*\(`,
	)
)

// Scan implements Scanner.
func (s *ChiScanner) Scan(dir string) ([]Route, error) {
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

		found, ferr := scanChiFile(path)
		if ferr != nil {
			return nil
		}
		routes = append(routes, found...)
		return nil
	})

	return routes, err
}

func scanChiFile(path string) ([]Route, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		routes      []Route
		prefixStack []string
		// braceDepth tracks nested anonymous funcs for prefix pop.
		braceDepth []int
		depth      int
		lines      []string
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

		// Count brace depth changes in this line.
		opens := strings.Count(line, "{")
		closes := strings.Count(line, "}")
		depth += opens - closes

		// Push prefix on r.Route("/prefix", ...) pattern.
		if m := chiRouteBlockRe.FindStringSubmatch(trimmed); m != nil {
			prefixStack = append(prefixStack, m[1])
			braceDepth = append(braceDepth, depth)
		}

		// Push empty prefix for r.Group(...) — groups don't add a path segment.
		if chiGroupRe.MatchString(trimmed) && !chiRouteBlockRe.MatchString(trimmed) {
			prefixStack = append(prefixStack, "")
			braceDepth = append(braceDepth, depth)
		}

		// Pop when brace depth returns to push level.
		for len(braceDepth) > 0 && depth < braceDepth[len(braceDepth)-1] {
			prefixStack = prefixStack[:len(prefixStack)-1]
			braceDepth = braceDepth[:len(braceDepth)-1]
		}

		// Match individual routes.
		m := chiRouteRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		method := strings.ToUpper(m[1])
		routePath := joinPaths(currentPrefix(prefixStack), m[2])
		handler := lastHandler(strings.TrimSpace(m[3]))
		hasSwagger := linesHaveSwagger(lines, i, 20)

		routes = append(routes, Route{
			Method:     method,
			Path:       routePath,
			Handler:    handler,
			File:       path,
			Line:       lineNum,
			HasSwagger: hasSwagger,
		})
	}

	return routes, nil
}
