package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ExpressScanner extracts routes from Node.js Express projects.
type ExpressScanner struct{}

// Name implements Scanner.
func (s *ExpressScanner) Name() string { return "express" }

// Express route patterns:
//
//	app.get('/path', handler)
//	router.post('/path', mw, handler)
//	app.use('/prefix', router)
//	router.route('/path').get(mw, handler).post(mw, handler)
var (
	// expressRouteStartRe matches the beginning of a route call up to the opening
	// paren, capturing (1) HTTP method, (2) path string. The caller uses
	// jsExtractBalancedArgs to read the full argument list past the opening paren.
	expressRouteStartRe = regexp.MustCompile(
		`(?i)(?:app|router)\s*\.\s*(get|post|put|delete|patch|head|options|all)\s*\(\s*['"]([^'"]+)['"]\s*,\s*`,
	)
	expressUseRe = regexp.MustCompile(
		`(?i)(?:app|router)\s*\.use\s*\(\s*['"]([^'"]+)['"]\s*,`,
	)
	// expressRouteChainRe matches the .route('/path') anchor of chained calls.
	expressRouteChainRe = regexp.MustCompile(
		`(?i)(?:app|router)\s*\.route\s*\(\s*['"]([^'"]+)['"]\s*\)`,
	)
	// expressChainMethodStartRe matches the start of a .METHOD( call on a chain.
	// It captures the method name and leaves the cursor just after the opening paren
	// so balanced-paren extraction can find the full argument list.
	expressChainMethodStartRe = regexp.MustCompile(
		`(?i)\.\s*(get|post|put|delete|patch|head|options|all)\s*\(`,
	)
	jsSwaggerRe    = regexp.MustCompile(`@swagger|@openapi|swagger-jsdoc`)
	jsMultiSpaceRe = regexp.MustCompile(`[\s\r\n]+`)
)

// Scan implements Scanner.
func (s *ExpressScanner) Scan(dir string) ([]Route, error) {
	var routes []Route

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}
		if !isJSFile(path) {
			return nil
		}

		found, ferr := scanExpressFile(path)
		if ferr != nil {
			return nil
		}
		routes = append(routes, found...)
		return nil
	})

	return routes, err
}

func scanExpressFile(path string) ([]Route, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var (
		routes   []Route
		lines    []string
		prefixes []string
	)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// --- Pass 1: single-line router.METHOD('/path', ...) ---
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Track use-prefixes (app.use('/api', router) style).
		if m := expressUseRe.FindStringSubmatch(trimmed); m != nil {
			prefixes = append(prefixes, m[1])
		}

		loc := expressRouteStartRe.FindStringSubmatchIndex(trimmed)
		if loc == nil {
			continue
		}

		method := strings.ToUpper(trimmed[loc[2]:loc[3]])
		routePath := trimmed[loc[4]:loc[5]]

		// loc[1] is the end of the matched prefix (just after the path + comma +
		// trailing whitespace), which is where the handler arguments begin.
		args, _ := jsExtractBalancedArgs(trimmed[loc[1]:])
		handler := cleanJSHandler(args)

		// Apply any detected prefix.
		prefix := ""
		if len(prefixes) > 0 {
			prefix = prefixes[len(prefixes)-1]
		}
		fullPath := joinPaths(prefix, routePath)

		hasSwagger := jsLinesHaveSwagger(lines, i, 20)

		routes = append(routes, Route{
			Method:     method,
			Path:       fullPath,
			Handler:    handler,
			File:       path,
			Line:       lineNum,
			HasSwagger: hasSwagger,
		})
	}

	// --- Pass 2: router.route('/path').get(...).post(...) chains ---
	// Collapse the file to a single string so multi-line chains become tokens.
	fullText := strings.Join(lines, "\n")
	chainRoutes := scanExpressRouteChains(fullText, path, lines)
	routes = append(routes, chainRoutes...)

	return routes, nil
}

// scanExpressRouteChains finds router.route('/path').METHOD(handlers) patterns
// that may span multiple lines by operating on the collapsed file text.
func scanExpressRouteChains(text, filePath string, lines []string) []Route {
	// Collapse whitespace/newlines so multi-line chains become single tokens.
	collapsed := jsMultiSpaceRe.ReplaceAllString(text, " ")

	var routes []Route

	chainMatches := expressRouteChainRe.FindAllStringSubmatchIndex(collapsed, -1)
	for _, loc := range chainMatches {
		routePath := collapsed[loc[2]:loc[3]]

		// Grab everything after the .route('path') token up to the next
		// statement boundary (semicolon) to avoid spilling into the next chain.
		rest := collapsed[loc[1]:]
		if semi := strings.IndexByte(rest, ';'); semi >= 0 {
			rest = rest[:semi]
		}

		// Find each chained .METHOD( call within the chain segment.
		startMatches := expressChainMethodStartRe.FindAllStringSubmatchIndex(rest, -1)
		for _, sm := range startMatches {
			method := strings.ToUpper(rest[sm[2]:sm[3]])
			// sm[1] is the index just after '(' — extract balanced paren content.
			args, _ := jsExtractBalancedArgs(rest[sm[1]:])
			handler := cleanJSHandler(args)

			lineNum := jsApproxLineNum(text, routePath)
			hasSwagger := jsLinesHaveSwagger(lines, lineNum-1, 20)

			routes = append(routes, Route{
				Method:     method,
				Path:       routePath,
				Handler:    handler,
				File:       filePath,
				Line:       lineNum,
				HasSwagger: hasSwagger,
			})
		}
	}

	return routes
}

// jsExtractBalancedArgs returns the content between the first matched pair of
// balanced parentheses in s. It assumes s starts just after an opening '(' that
// was already consumed by the caller regex. Returns (content, remaining) where
// content is what was between the parens and remaining is what follows the
// closing ')'. If no balanced pair is found the full string is returned.
func jsExtractBalancedArgs(s string) (string, string) {
	depth := 1
	for i, ch := range s {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[:i], s[i+1:]
			}
		}
	}
	return s, ""
}

// jsApproxLineNum returns a 1-based line number for the first occurrence of
// needle in text, or 1 if not found.
func jsApproxLineNum(text, needle string) int {
	idx := strings.Index(text, needle)
	if idx < 0 {
		return 1
	}
	return strings.Count(text[:idx], "\n") + 1
}

// cleanJSHandler picks the last meaningful handler name from a raw handler
// expression, stripping middleware wrappers and async keywords.
func cleanJSHandler(raw string) string {
	// Split on commas that are not inside parentheses.
	parts := splitJSTopLevel(raw, ',')
	if len(parts) == 0 {
		return strings.TrimSpace(raw)
	}
	last := strings.TrimSpace(parts[len(parts)-1])
	// Strip trailing ) from outer match bleed-through.
	last = strings.TrimRight(last, ")")
	last = strings.TrimSpace(last)
	// Strip "async" keyword.
	last = strings.TrimPrefix(last, "async ")
	last = strings.TrimSpace(last)
	// If the result is a call expression like foo(...), extract just the name.
	if idx := strings.Index(last, "("); idx > 0 {
		last = strings.TrimSpace(last[:idx])
	}
	return last
}

// splitJSTopLevel splits s by sep only at the top level of parentheses.
func splitJSTopLevel(s string, sep rune) []string {
	var parts []string
	depth := 0
	start := 0
	for i, ch := range s {
		switch ch {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case sep:
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// jsLinesHaveSwagger checks for JSDoc swagger annotations near lineIdx,
// looking both before (for annotations above the route) and after (for
// JSDoc blocks that appear below the route definitions).
func jsLinesHaveSwagger(lines []string, lineIdx, lookback int) bool {
	start := lineIdx - lookback
	if start < 0 {
		start = 0
	}
	end := lineIdx + lookback
	if end > len(lines) {
		end = len(lines)
	}
	for _, l := range lines[start:end] {
		if jsSwaggerRe.MatchString(l) {
			return true
		}
	}
	return false
}

// isJSFile reports whether path is a JavaScript or TypeScript source file.
func isJSFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".js" || ext == ".ts" || ext == ".mjs" || ext == ".cjs"
}
