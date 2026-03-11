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
//	r.Mount("/prefix", subRouter())
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
	// chiMountRe matches r.Mount("/prefix", handler) calls.
	chiMountRe = regexp.MustCompile(
		`\.Mount\s*\(\s*"([^"]+)"\s*,`,
	)
	// chiStripStringsRe removes quoted string contents before brace counting so
	// that path parameters like {id} in "/{id}" do not corrupt brace depth.
	chiStripStringsRe = regexp.MustCompile(`"[^"\\]*(?:\\.[^"\\]*)*"`)
	// chiInlineCommentRe removes inline // comments before brace counting so
	// that path params in trailing comments (e.g. "// GET /users/{id}") do
	// not corrupt brace depth.
	chiInlineCommentRe = regexp.MustCompile(`//.*$`)
)

// stripForBraceCount prepares a line for brace counting by removing content
// that can contain bare `{` or `}` characters that are not code-level braces:
//
//   - Double-quoted string literals (e.g. "/{id}" contains { and })
//   - Inline comments following // (e.g. "// GET /users/{id}")
//
// This prevents path parameter tokens from skewing the brace-depth counter.
func stripForBraceCount(line string) string {
	s := chiStripStringsRe.ReplaceAllString(line, `""`)
	return chiInlineCommentRe.ReplaceAllString(s, "")
}

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
		routes []Route
		// prefixStack holds accumulated path segments from r.Route/r.Group blocks.
		prefixStack []string
		// braceDepth records the brace depth at which each prefix was opened.
		// A prefix is popped when depth falls below its recorded level.
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

		// Skip pure line comments. Comments can contain route-like patterns
		// (e.g. "// r.Route("/admin", ...)") that would push phantom prefixes.
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Strip string literals and inline comments before counting braces.
		// Path parameters like {id} in "/{id}" and comments like "// GET /x/{id}"
		// both contain bare { and } that would corrupt the brace-depth counter.
		stripped := stripForBraceCount(line)
		opens := strings.Count(stripped, "{")
		closes := strings.Count(stripped, "}")

		// Pop expired prefixes using depth after applying this line's closing
		// braces — before the opening braces of any new block on this line
		// are counted. This correctly handles the }) pattern that closes a
		// block and the case where the same line contains both a close and open.
		depthAfterClose := depth - closes
		for len(braceDepth) > 0 && depthAfterClose < braceDepth[len(braceDepth)-1] {
			prefixStack = prefixStack[:len(prefixStack)-1]
			braceDepth = braceDepth[:len(braceDepth)-1]
		}

		// Apply the net brace change for this line.
		depth += opens - closes

		// Push prefix on r.Route("/prefix", ...) pattern.
		// Depth is recorded AFTER the opening brace so the pop fires when we
		// return to the level at which the block was opened.
		if m := chiRouteBlockRe.FindStringSubmatch(trimmed); m != nil {
			prefixStack = append(prefixStack, m[1])
			braceDepth = append(braceDepth, depth)
		}

		// Push empty prefix for r.Group(...) — groups don't add a path segment.
		if chiGroupRe.MatchString(trimmed) && !chiRouteBlockRe.MatchString(trimmed) {
			prefixStack = append(prefixStack, "")
			braceDepth = append(braceDepth, depth)
		}

		// Record r.Mount("/prefix", ...) as a synthetic MOUNT route so the
		// mount point is visible in output even when the sub-router is defined
		// in a separate function or file.
		if mm := chiMountRe.FindStringSubmatch(trimmed); mm != nil {
			if chiRouteRe.FindStringSubmatch(trimmed) == nil {
				mountPath := joinPaths(currentPrefix(prefixStack), mm[1])
				routes = append(routes, Route{
					Method:     "MOUNT",
					Path:       mountPath + "/*",
					Handler:    "<mounted>",
					File:       path,
					Line:       lineNum,
					HasSwagger: linesHaveSwagger(lines, i, 20),
				})
			}
		}

		// Match individual route registrations.
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
