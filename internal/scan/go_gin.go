package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GinScanner extracts routes from Go projects using the gin-gonic/gin framework.
// It is also reused for fiber and stdlib (net/http mux-style) with a name override.
type GinScanner struct {
	nameOverride string
}

// Name implements Scanner.
func (s *GinScanner) Name() string {
	if s.nameOverride != "" {
		return s.nameOverride
	}
	return "gin"
}

// Gin route patterns:
//
//	r.GET("/path", handler)
//	r.POST("/path", middleware1, handler)
//	group.GET("/path", handler)
//	v1 := r.Group("/v1")
var (
	ginRouteRe = regexp.MustCompile(
		`(?i)\.\s*(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|ANY)\s*\(\s*"([^"]+)"\s*,\s*([^)]+)\)`,
	)
	ginGroupRe = regexp.MustCompile(
		`(?i)\.Group\s*\(\s*"([^"]+)"`,
	)
	swaggerCommentRe = regexp.MustCompile(`//\s*@\w+`)
)

// Scan implements Scanner.
func (s *GinScanner) Scan(dir string) ([]Route, error) {
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

		found, ferr := scanGinFile(path)
		if ferr != nil {
			return nil
		}
		routes = append(routes, found...)
		return nil
	})

	return routes, err
}

func scanGinFile(path string) ([]Route, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		routes      []Route
		prefixStack []string
		lines       []string
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

		// Track group prefixes: v1 := r.Group("/v1")
		if m := ginGroupRe.FindStringSubmatch(trimmed); m != nil {
			prefixStack = append(prefixStack, m[1])
		}

		// Pop prefix when we see closing brace (heuristic).
		if trimmed == "}" && len(prefixStack) > 0 {
			prefixStack = prefixStack[:len(prefixStack)-1]
		}

		// Match route registrations.
		m := ginRouteRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		method := strings.ToUpper(m[1])
		routePath := joinPaths(currentPrefix(prefixStack), m[2])
		handlerRaw := strings.TrimSpace(m[3])
		handler := lastHandler(handlerRaw)

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

// currentPrefix returns the accumulated prefix from the stack.
func currentPrefix(stack []string) string {
	return strings.Join(stack, "")
}

// joinPaths concatenates a prefix and a path, avoiding double slashes.
func joinPaths(prefix, path string) string {
	if prefix == "" {
		return path
	}
	prefix = strings.TrimRight(prefix, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return prefix + path
}

// lastHandler extracts the last identifier from a comma-separated handler list.
// In gin, `r.GET("/", mw1, mw2, actualHandler)` — the last one is the handler.
func lastHandler(raw string) string {
	parts := strings.Split(raw, ",")
	last := strings.TrimSpace(parts[len(parts)-1])
	// Remove trailing ) if it slipped through.
	last = strings.TrimRight(last, ")")
	return strings.TrimSpace(last)
}

// linesHaveSwagger checks up to lookback lines before lineIdx for swaggo annotations.
func linesHaveSwagger(lines []string, lineIdx, lookback int) bool {
	start := lineIdx - lookback
	if start < 0 {
		start = 0
	}
	for _, l := range lines[start:lineIdx] {
		if swaggerCommentRe.MatchString(l) {
			return true
		}
	}
	return false
}

// shouldSkipDir reports whether a directory name should be skipped during walk.
func shouldSkipDir(name string) bool {
	skip := map[string]bool{
		".git": true, "vendor": true, "node_modules": true,
		".idea": true, ".vscode": true, "testdata": true,
		"dist": true, "build": true, "__pycache__": true,
	}
	return skip[name]
}

// isGoFile reports whether path is a Go source file.
func isGoFile(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}
