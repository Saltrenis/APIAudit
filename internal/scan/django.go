package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DjangoScanner extracts routes from Django REST Framework projects.
type DjangoScanner struct{}

// Name implements Scanner.
func (s *DjangoScanner) Name() string { return "django" }

// Django URL patterns:
//
//	path('users/', views.UserListView.as_view(), name='user-list')
//	re_path(r'^users/(?P<pk>\d+)/$', views.UserDetailView.as_view())
//	urlpatterns = [...]
//	@api_view(['GET', 'POST'])
var (
	djangoPathRe = regexp.MustCompile(
		`(?:path|re_path)\s*\(\s*r?['"]([^'"]+)['"]\s*,\s*([^,)]+)`,
	)
	djangoAPIViewRe = regexp.MustCompile(
		`@api_view\s*\(\s*\[([^\]]+)\]`,
	)
	djangoDefRe     = regexp.MustCompile(`def\s+(\w+)\s*\(`)
	djangoIncludeRe = regexp.MustCompile(
		`path\s*\(\s*['"]([^'"]+)['"]\s*,\s*include\s*\(`,
	)
)

// Scan implements Scanner.
func (s *DjangoScanner) Scan(dir string) ([]Route, error) {
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
		// Only process urls.py files and views.py for @api_view decorators.
		base := filepath.Base(path)
		if base != "urls.py" && base != "views.py" && !strings.HasSuffix(base, "_views.py") {
			return nil
		}

		found, ferr := scanDjangoFile(path)
		if ferr != nil {
			return nil
		}
		routes = append(routes, found...)
		return nil
	})

	return routes, err
}

func scanDjangoFile(path string) ([]Route, error) {
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

	base := filepath.Base(path)

	if base == "urls.py" || strings.HasSuffix(base, "_urls.py") {
		return parseDjangoURLs(path, lines), nil
	}

	return parseDjangoViews(path, lines), nil
}

func parseDjangoURLs(path string, lines []string) []Route {
	var routes []Route

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Skip include() lines — they reference other url files.
		if djangoIncludeRe.MatchString(trimmed) {
			continue
		}

		m := djangoPathRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		routePath := "/" + strings.TrimPrefix(m[1], "/")
		handler := strings.TrimSpace(m[2])
		// Strip .as_view() suffix for readability.
		handler = regexp.MustCompile(`\.as_view\s*\(\s*\)`).ReplaceAllString(handler, "")
		handler = strings.TrimSpace(handler)

		// Django url patterns don't specify a method; use ANY as placeholder.
		routes = append(routes, Route{
			Method:  "ANY",
			Path:    routePath,
			Handler: handler,
			File:    path,
			Line:    lineNum,
		})
	}

	return routes
}

func parseDjangoViews(path string, lines []string) []Route {
	var routes []Route

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		m := djangoAPIViewRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		methods := parseDjangoMethods(m[1])

		// Find function definition.
		handler := ""
		for j := i + 1; j < len(lines) && j < i+4; j++ {
			if mm := djangoDefRe.FindStringSubmatch(strings.TrimSpace(lines[j])); mm != nil {
				handler = mm[1]
				break
			}
		}

		for _, method := range methods {
			routes = append(routes, Route{
				Method:  method,
				Path:    "/" + handler, // best-effort; real path comes from urls.py
				Handler: handler,
				File:    path,
				Line:    lineNum,
			})
		}
	}

	return routes
}

func parseDjangoMethods(raw string) []string {
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
