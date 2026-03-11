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
var (
	echoRouteRe = regexp.MustCompile(
		`(?i)\.\s*(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*\(\s*"([^"]+)"\s*,\s*([^)]+)\)`,
	)
	echoGroupRe = regexp.MustCompile(
		`\.Group\s*\(\s*"([^"]+)"`,
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

		// Track group prefixes.
		if m := echoGroupRe.FindStringSubmatch(trimmed); m != nil {
			prefixStack = append(prefixStack, m[1])
		}

		if trimmed == "}" && len(prefixStack) > 0 {
			prefixStack = prefixStack[:len(prefixStack)-1]
		}

		m := echoRouteRe.FindStringSubmatch(trimmed)
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
