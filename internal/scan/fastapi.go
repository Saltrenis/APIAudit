package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FastAPIScanner extracts routes from Python FastAPI projects.
type FastAPIScanner struct{}

// Name implements Scanner.
func (s *FastAPIScanner) Name() string { return "fastapi" }

// FastAPI decorator patterns:
//
//	@app.get("/path")
//	@router.post("/path")
//	@app.get("/path", response_model=SomeModel)
//	router = APIRouter(prefix="/prefix")
var (
	fastapiRouteRe = regexp.MustCompile(
		`@(?:\w+)\.(get|post|put|delete|patch|head|options)\s*\(\s*["']([^"']+)["']`,
	)
	fastapiRouterPrefixRe = regexp.MustCompile(
		`APIRouter\s*\([^)]*prefix\s*=\s*["']([^"']+)["']`,
	)
	fastapiDefRe = regexp.MustCompile(`(?:async\s+)?def\s+(\w+)\s*\(`)
	pySwaggerRe  = regexp.MustCompile(`@\w+\.(get|post|put|delete|patch)`)
)

// Scan implements Scanner.
func (s *FastAPIScanner) Scan(dir string) ([]Route, error) {
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

		found, ferr := scanFastAPIFile(path)
		if ferr != nil {
			return nil
		}
		routes = append(routes, found...)
		return nil
	})

	return routes, err
}

func scanFastAPIFile(path string) ([]Route, error) {
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

	// Detect router prefix for this file.
	prefix := ""
	for _, line := range lines {
		if m := fastapiRouterPrefixRe.FindStringSubmatch(line); m != nil {
			prefix = m[1]
			break
		}
	}

	var routes []Route
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		m := fastapiRouteRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		method := strings.ToUpper(m[1])
		routePath := joinPaths(prefix, m[2])

		// Find the function definition immediately following the decorator.
		handler := ""
		for j := i + 1; j < len(lines) && j < i+4; j++ {
			if mm := fastapiDefRe.FindStringSubmatch(strings.TrimSpace(lines[j])); mm != nil {
				handler = mm[1]
				break
			}
		}

		routes = append(routes, Route{
			Method:     method,
			Path:       routePath,
			Handler:    handler,
			File:       path,
			Line:       lineNum,
			HasSwagger: false, // FastAPI generates docs automatically; no manual annotations needed.
		})
	}

	return routes, nil
}

// isPythonFile reports whether path is a Python source file.
func isPythonFile(path string) bool {
	return strings.HasSuffix(path, ".py")
}
