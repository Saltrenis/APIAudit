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

// Django URL patterns (modern and legacy):
//
//	path('users/', views.UserListView.as_view(), name='user-list')
//	re_path(r'^users/(?P<pk>\d+)/$', views.UserDetailView.as_view())
//	url(r'^users/$', views.UserListView.as_view())        # legacy Django 1.x
//	urlpatterns = [...]
//	@api_view(['GET', 'POST'])
//	class MyView(APIView):
//	    def get(self, request): ...
var (
	// djangoURLFuncRe matches path(), re_path(), and legacy url() function calls.
	// It captures the URL pattern string and the view/handler argument.
	// The handler group captures through optional .as_view() calls by allowing
	// one level of nested parens: \w+\([^)]*\) or a plain identifier.
	djangoURLFuncRe = regexp.MustCompile(
		`(?:path|re_path|url)\s*\(\s*r?['"]([^'"]+)['"]\s*,\s*([\w.]+(?:\([^)]*\))?)\s*[,)]`,
	)
	djangoAPIViewRe = regexp.MustCompile(
		`@api_view\s*\(\s*\[([^\]]+)\]`,
	)
	djangoDefRe = regexp.MustCompile(`def\s+(\w+)\s*\(`)
	// djangoIncludeRe matches any url/path/re_path call that delegates to include().
	djangoIncludeRe = regexp.MustCompile(
		`(?:path|re_path|url)\s*\(\s*r?['"][^'"]*['"]\s*,\s*include\s*\(`,
	)
	// djangoCBVMethodRe matches HTTP method handlers inside class-based views.
	djangoCBVMethodRe = regexp.MustCompile(`^\s*def\s+(get|post|put|patch|delete|head|options)\s*\(`)
	// djangoClassRe matches a class definition that inherits from a DRF view base.
	djangoClassRe = regexp.MustCompile(`^class\s+(\w+)\s*\(`)
	// djangoNamedGroupRe converts (?P<name>...) regex groups to {name} path parameters.
	djangoNamedGroupRe = regexp.MustCompile(`\(\?P<(\w+)>[^)]+\)`)
	// djangoUnnamedGroupRe removes remaining unnamed capture/non-capture groups like (\d+).
	djangoUnnamedGroupRe = regexp.MustCompile(`\([^)]*\)`)
	// djangoAsViewRe strips .as_view() from handler strings.
	djangoAsViewRe = regexp.MustCompile(`\.as_view\s*\(\s*\)`)
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
		// Only process urls.py files and views.py for @api_view / CBV decorators.
		base := filepath.Base(path)
		if base != "urls.py" && base != "views.py" &&
			!strings.HasSuffix(base, "_urls.py") && !strings.HasSuffix(base, "_views.py") {
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

type djangoLogicalLine struct {
	line    string
	lineNum int
}

// djangoJoinContinuationLines merges lines where an open url()/path() call
// spans multiple physical lines. It returns a slice of logical lines, each
// annotated with the 1-based line number where the logical line starts.
func djangoJoinContinuationLines(lines []string) []djangoLogicalLine {
	var out []djangoLogicalLine

	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])

		// Does this line open a url/path/re_path call without closing it?
		if isURLFuncStart(trimmed) && !isURLFuncComplete(trimmed) {
			startNum := i + 1
			joined := trimmed
			i++
			// Absorb continuation lines until the view argument is present.
			for i < len(lines) {
				next := strings.TrimSpace(lines[i])
				joined += " " + next
				i++
				if isURLFuncComplete(joined) {
					break
				}
			}
			out = append(out, djangoLogicalLine{line: joined, lineNum: startNum})
			continue
		}

		out = append(out, djangoLogicalLine{line: trimmed, lineNum: i + 1})
		i++
	}

	return out
}

// isURLFuncStart reports whether a line begins a url()/path()/re_path() call.
func isURLFuncStart(line string) bool {
	return regexp.MustCompile(`(?:path|re_path|url)\s*\(`).MatchString(line)
}

// isURLFuncComplete reports whether a string contains a balanced closing paren
// after the URL string argument, i.e. the view argument is present.
func isURLFuncComplete(line string) bool {
	// A complete call has the pattern string and at least one more argument.
	return djangoURLFuncRe.MatchString(line)
}

func parseDjangoURLs(path string, lines []string) []Route {
	var routes []Route

	// Strip triple-quoted docstring sections before joining continuation lines
	// so that example url() calls inside docstrings are not matched.
	stripped := djangoStripDocstrings(lines)

	logicalLines := djangoJoinContinuationLines(stripped)

	for _, ll := range logicalLines {
		trimmed := ll.line
		lineNum := ll.lineNum

		// Skip include() delegations — they reference other url files.
		if djangoIncludeRe.MatchString(trimmed) {
			continue
		}

		m := djangoURLFuncRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		rawPattern := m[1]
		handler := strings.TrimSpace(m[2])

		routePath := djangoPatternToPath(rawPattern)

		// Strip .as_view() suffix for readability.
		handler = djangoAsViewRe.ReplaceAllString(handler, "")
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

// djangoStripDocstrings returns a copy of lines with triple-quoted string
// content replaced by blank lines, preserving line numbers.
func djangoStripDocstrings(lines []string) []string {
	out := make([]string, len(lines))
	inDocstring := false
	for i, line := range lines {
		count := strings.Count(line, `"""`)
		if !inDocstring {
			if count >= 2 {
				// Opening and closing triple-quote on the same line — blank it.
				out[i] = ""
				continue
			}
			if count == 1 {
				// Entering a multi-line docstring.
				inDocstring = true
				out[i] = ""
				continue
			}
		} else {
			out[i] = ""
			if count >= 1 {
				// Closing triple-quote found.
				inDocstring = false
			}
			continue
		}
		out[i] = line
	}
	return out
}

// djangoPatternToPath converts a Django URL regex/path pattern to a readable
// path string. It strips regex anchors and converts named capture groups.
//
// Examples:
//
//	r'^users/(?P<pk>\d+)/$'  →  /users/{pk}/
//	'articles/<int:pk>/'     →  /articles/{pk}/
func djangoPatternToPath(raw string) string {
	// Strip leading ^ and trailing $
	p := strings.TrimPrefix(raw, "^")
	p = strings.TrimSuffix(p, "$")
	p = strings.TrimSuffix(p, "/$")

	// Convert named regex groups: (?P<name>...) -> {name}
	p = djangoNamedGroupRe.ReplaceAllString(p, "{$1}")

	// Convert Django 2+ path converters: <int:pk> -> {pk}
	p = regexp.MustCompile(`<(?:\w+:)?(\w+)>`).ReplaceAllString(p, "{$1}")

	// Remove remaining unnamed/non-capture groups like (\d+) or (?:foo)
	p = djangoUnnamedGroupRe.ReplaceAllString(p, "")

	// Remove regex quantifiers that leak into path segments (e.g. "?", "*", "+")
	// but only outside of path parameter braces.
	p = regexp.MustCompile(`[?*+]`).ReplaceAllString(p, "")

	// Collapse repeated slashes
	p = regexp.MustCompile(`/{2,}`).ReplaceAllString(p, "/")

	// Ensure leading slash
	p = "/" + strings.TrimPrefix(p, "/")

	// Remove trailing slash for consistency.
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}

	return p
}

func parseDjangoViews(path string, lines []string) []Route {
	var routes []Route

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// --- @api_view decorator (function-based views) ---
		if m := djangoAPIViewRe.FindStringSubmatch(trimmed); m != nil {
			methods := parseDjangoMethods(m[1])

			// Find function definition on the next few lines.
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
			continue
		}

		// --- Class-based views: detect HTTP method handlers (get/post/put/...) ---
		// When we find a class definition that looks like a DRF view, collect the
		// HTTP method handlers defined inside it.
		if cm := djangoClassRe.FindStringSubmatch(trimmed); cm != nil {
			className := cm[1]
			classLine := lineNum

			// Scan the class body to find method handlers.
			// Stop when we reach a new top-level definition (unindented class/def).
			for j := i + 1; j < len(lines); j++ {
				bodyLine := lines[j]
				// A non-empty line with no indentation signals the end of the class.
				if len(bodyLine) > 0 && bodyLine[0] != ' ' && bodyLine[0] != '\t' {
					break
				}
				if mm := djangoCBVMethodRe.FindStringSubmatch(bodyLine); mm != nil {
					httpMethod := strings.ToUpper(mm[1])
					routes = append(routes, Route{
						Method:  httpMethod,
						Path:    "/" + className, // best-effort path; real path from urls.py
						Handler: className + "." + mm[1],
						File:    path,
						Line:    classLine,
					})
				}
			}
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
