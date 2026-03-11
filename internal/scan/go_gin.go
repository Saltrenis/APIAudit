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
//	router.GET("", handler)  // empty string path, equivalent to "/"
var (
	// ginRouteRe matches HTTP method registrations. Capture groups:
	// 1=receiver variable, 2=HTTP method, 3=path (may be empty — "" is valid
	// in gin as a synonym for "/"), 4=handler arguments.
	ginRouteRe = regexp.MustCompile(
		`(?i)(\w+)\.\s*(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|ANY)\s*\(\s*"([^"]*)"\s*,\s*([^)]+)\)`,
	)
	// ginGroupAssignRe captures variable-assigned Group calls so we can build a
	// variable-to-prefix map for same-file route resolution.
	// Capture groups: 1=lhs var name, 2=base var, 3=path prefix.
	// e.g. `v1 := r.Group("/api")` → lhs="v1", base="r", prefix="/api"
	ginGroupAssignRe = regexp.MustCompile(
		`(\w+)\s*:?=\s*(\w+)\.Group\s*\(\s*"([^"]*)"`,
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

	// First pass: build a variable-to-full-prefix map from Group assignments
	// within this file. For example:
	//   v1 := r.Group("/api")          → varPrefix["v1"] = "/api"
	//   testAuth := r.Group("/api/ping") → varPrefix["testAuth"] = "/api/ping"
	//   sub := v1.Group("/users")      → varPrefix["sub"] = "/api/users"
	//
	// Groups passed as function arguments (not assigned) are not tracked here;
	// those routes are registered in other files and appear without a prefix.
	varPrefix := make(map[string]string)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := ginGroupAssignRe.FindStringSubmatch(trimmed); m != nil {
			lhsVar, baseVar, prefix := m[1], m[2], m[3]
			if parentPrefix, ok := varPrefix[baseVar]; ok {
				varPrefix[lhsVar] = joinPaths(parentPrefix, prefix)
			} else {
				varPrefix[lhsVar] = prefix
			}
		}
	}

	// Second pass: extract route registrations.
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		m := ginRouteRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		receiverVar := m[1]
		method := strings.ToUpper(m[2])
		rawPath := m[3]
		// Gin allows "" as a path synonym for "/"; normalise it so callers
		// always see a non-empty path string.
		if rawPath == "" {
			rawPath = "/"
		}

		prefix := varPrefix[receiverVar] // empty string when not a known group var
		routePath := joinPaths(prefix, rawPath)
		handlerRaw := strings.TrimSpace(m[4])
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

// currentPrefix returns the accumulated prefix by joining all stack entries.
// Used by chi and echo scanners that still use a brace-counting prefix stack.
func currentPrefix(stack []string) string {
	return strings.Join(stack, "")
}

// joinPaths concatenates a prefix and a path, avoiding double slashes.
// When path is empty (e.g. a bare @Post() with no sub-path), the prefix
// itself is the full path and no trailing slash is appended.
func joinPaths(prefix, path string) string {
	if prefix == "" {
		if path == "" {
			return "/"
		}
		return path
	}
	prefix = strings.TrimRight(prefix, "/")
	if path == "" {
		return prefix
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return prefix + path
}

// lastHandler extracts the last identifier from a comma-separated handler list.
// In gin, `r.GET("/", mw1, mw2, actualHandler)` — the last one is the handler.
// Inline anonymous functions (func(...) { ... }) are reported as "<inline>".
func lastHandler(raw string) string {
	// Detect inline anonymous handler before splitting on commas, since the
	// function literal may itself contain commas (e.g. func(c *gin.Context)).
	if strings.HasPrefix(raw, "func(") || strings.Contains(raw, ", func(") {
		return "<inline>"
	}
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
