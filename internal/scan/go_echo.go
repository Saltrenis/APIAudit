package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// EchoScanner extracts routes from Go projects using the labstack/echo framework.
type EchoScanner struct{}

// Name implements Scanner.
func (s *EchoScanner) Name() string { return "echo" }

// Echo route patterns:
//
//	e.GET("/path", handler)
//	g := e.Group("/prefix")
//	g.POST("/path", handler)
//	g.POST("", handler)  // empty path — resolves to the group prefix itself
var (
	echoRouteRe = regexp.MustCompile(
		`(?i)(\w+)\.\s*(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*\(\s*"([^"]*)"\s*,\s*([^)]+)\)`,
	)
	// echoGroupAssignRe captures variable-assigned Group calls.
	// Capture groups: 1=lhs var, 2=base var, 3=path prefix.
	// e.g. `v1 := e.Group("/api")` → lhs="v1", base="e", prefix="/api"
	// also handles middleware args: `g := e.Group("/path", mw1)`
	echoGroupAssignRe = regexp.MustCompile(
		`(\w+)\s*:?=\s*(\w+)\.Group\s*\(\s*"([^"]*)"`,
	)
)

// Scan implements Scanner.
func (s *EchoScanner) Scan(dir string) ([]Route, error) {
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

		found, ferr := scanEchoFile(path)
		if ferr != nil {
			return nil
		}
		routes = append(routes, found...)
		return nil
	})

	return routes, err
}

func scanEchoFile(path string) ([]Route, error) {
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
	//   v1 := e.Group("/api")          → varPrefix["v1"] = "/api"
	//   users := v1.Group("/users")    → varPrefix["users"] = "/api/users"
	//
	// Groups received as function parameters (e.g. `func(h *Handler) Register(v1 *echo.Group)`)
	// are not tracked here; routes registered on those variables appear with only
	// the suffix path (no prefix), which is accurate for most per-file scans.
	varPrefix := make(map[string]string)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := echoGroupAssignRe.FindStringSubmatch(trimmed); m != nil {
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

		m := echoRouteRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		receiverVar := m[1]
		method := strings.ToUpper(m[2])
		rawPath := m[3]
		handlerRaw := strings.TrimSpace(m[4])

		// Echo allows "" as a path equivalent to the group prefix itself;
		// normalise so callers always see a rooted path.
		if rawPath == "" {
			rawPath = "/"
		}

		prefix := varPrefix[receiverVar] // empty string when not a known group var
		routePath := joinPaths(prefix, rawPath)
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
