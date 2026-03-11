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
//	@router.get(
//	    "/path",
//	    dependencies=[...],
//	)
//	router = APIRouter(prefix="/prefix")
var (
	// fastapiRouteRe matches a route decorator in a single string (which may be
	// a joined multi-line decorator).  The path may use single or double quotes.
	fastapiRouteRe = regexp.MustCompile(
		`@(?:\w+)\.(get|post|put|delete|patch|head|options)\s*\(\s*["']([^"']+)["']`,
	)

	// fastapiDecoratorStartRe detects the opening of a route decorator so that
	// multi-line forms can be collected before applying fastapiRouteRe.
	fastapiDecoratorStartRe = regexp.MustCompile(
		`@(?:\w+)\.(get|post|put|delete|patch|head|options)\s*\(`,
	)

	// fastapiRouterPrefixRe matches a single-line APIRouter declaration that
	// includes a prefix keyword argument.
	fastapiRouterPrefixRe = regexp.MustCompile(
		`APIRouter\s*\([^)]*prefix\s*=\s*["']([^"']+)["']`,
	)

	// fastapiRouterPrefixMultiRe is used on joined multi-line APIRouter blocks
	// where the [^)]* look-ahead would span too many lines.
	fastapiRouterPrefixMultiRe = regexp.MustCompile(
		`prefix\s*=\s*["']([^"']+)["']`,
	)

	fastapiDefRe = regexp.MustCompile(`(?:async\s+)?def\s+(\w+)\s*\(`)
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
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	// Detect router prefix for this file, handling both single-line and
	// multi-line APIRouter(...) declarations.
	prefix := detectFARouterPrefix(lines)

	var routes []Route
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

		// Fast-path: skip lines that cannot start a route decorator.
		if !fastapiDecoratorStartRe.MatchString(trimmed) {
			continue
		}

		lineNum := i + 1

		// Collect the full decorator text.  For single-line forms this is just
		// the line itself.  For multi-line forms we join continuation lines
		// until the opening parenthesis is balanced.
		decoratorText, decoratorEnd := faCollectDecoratorLines(lines, i)

		m := fastapiRouteRe.FindStringSubmatch(decoratorText)
		if m == nil {
			continue
		}

		method := strings.ToUpper(m[1])
		routePath := joinPaths(prefix, m[2])

		// Find the function definition immediately following the decorator block.
		handler := ""
		for j := decoratorEnd + 1; j < len(lines) && j < decoratorEnd+5; j++ {
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
			HasSwagger: false, // FastAPI generates docs automatically.
		})

		// Advance past the decorator block to avoid double-counting.
		i = decoratorEnd
	}

	return routes, nil
}

// faCollectDecoratorLines returns the full decorator text (lines joined with a
// single space) and the index of the last line that belongs to the decorator.
// It stops when the opening parenthesis opened on startIdx is balanced, or when
// a def/async def is encountered (which already belongs to the next statement).
func faCollectDecoratorLines(lines []string, startIdx int) (string, int) {
	var sb strings.Builder
	depth := 0

	for i := startIdx; i < len(lines) && i < startIdx+20; i++ {
		text := strings.TrimSpace(lines[i])

		// A def/async def on any line after the first means the decorator ended
		// on the previous line (open paren count must have reached zero).
		if i > startIdx && fastapiDefRe.MatchString(text) {
			return sb.String(), i - 1
		}

		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(text)

		for _, ch := range text {
			switch ch {
			case '(':
				depth++
			case ')':
				depth--
			}
		}

		if depth <= 0 {
			return sb.String(), i
		}
	}

	return sb.String(), startIdx
}

// detectFARouterPrefix searches lines for an APIRouter prefix, handling both
// single-line and multi-line APIRouter(...) declarations.
func detectFARouterPrefix(lines []string) string {
	for i, line := range lines {
		if !strings.Contains(line, "APIRouter") {
			continue
		}

		// Single-line match first (most common).
		if m := fastapiRouterPrefixRe.FindStringSubmatch(line); m != nil {
			return m[1]
		}

		// Multi-line: join the APIRouter(...) block and search within it.
		joined, _ := faCollectDecoratorLines(lines, i)
		if m := fastapiRouterPrefixMultiRe.FindStringSubmatch(joined); m != nil {
			return m[1]
		}
	}
	return ""
}

// isPythonFile reports whether path is a Python source file.
func isPythonFile(path string) bool {
	return strings.HasSuffix(path, ".py")
}
