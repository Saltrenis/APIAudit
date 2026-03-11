package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FastifyScanner extracts routes from Node.js Fastify projects.
type FastifyScanner struct{}

// Name implements Scanner.
func (s *FastifyScanner) Name() string { return "fastify" }

// Fastify route patterns:
//
//	fastify.get('/path', handler)
//	fastify.post('/path', opts, handler)
//	app.get('/path', { schema: ... }, handler)
//	fastify.route({ method: 'GET', url: '/path', handler: handler })
//	fastify.register(routes, { prefix: '/api/v1' })
var (
	// fastifyShorthandRe matches shorthand route calls:
	//   fastify.get('/path', ...)   app.post('/path', ...)
	// Captures (1) method, (2) path.
	fastifyShorthandRe = regexp.MustCompile(
		`(?i)(?:fastify|app|server)\s*\.\s*(get|post|put|delete|patch|head|options|all)\s*\(\s*['"]([^'"]+)['"]\s*,\s*`,
	)

	// fastifyRouteRe matches fastify.route({ ... }) object-style registration.
	// We rely on multi-line collapsed text; individual fields are extracted below.
	fastifyRouteRe = regexp.MustCompile(
		`(?i)(?:fastify|app|server)\s*\.route\s*\(`,
	)

	// fastifyRouteMethodRe extracts method: 'GET' or method: ['GET', 'POST']
	// from a collapsed fastify.route({...}) block.
	fastifyRouteMethodRe = regexp.MustCompile(
		`(?i)method\s*:\s*(?:'([^']+)'|"([^"]+)"|\[([^\]]+)\])`,
	)

	// fastifyRouteURLRe extracts url: '/path' from a fastify.route({...}) block.
	fastifyRouteURLRe = regexp.MustCompile(
		`(?i)url\s*:\s*['"]([^'"]+)['"]`,
	)

	// fastifyRouteHandlerRe extracts handler: fn from a fastify.route({...}) block.
	fastifyRouteHandlerRe = regexp.MustCompile(
		`(?i)handler\s*:\s*(\w[\w.]*|async\s+function\s*\w*|function\s*\w*)`,
	)

	// fastifyRegisterRe matches fastify.register(plugin, { prefix: '/api' })
	// and captures the prefix value.
	fastifyRegisterRe = regexp.MustCompile(
		`(?i)(?:fastify|app|server)\s*\.register\s*\([^,)]+,\s*\{\s*prefix\s*:\s*['"]([^'"]+)['"]`,
	)
)

// Scan implements Scanner.
func (s *FastifyScanner) Scan(dir string) ([]Route, error) {
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

		found, ferr := scanFastifyFile(path)
		if ferr != nil {
			return nil
		}
		routes = append(routes, found...)
		return nil
	})

	return routes, err
}

func scanFastifyFile(path string) ([]Route, error) {
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

	// --- Pass 1: collect register prefixes and shorthand routes line-by-line ---
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Track prefixes from fastify.register(plugin, { prefix: '/api' }).
		if m := fastifyRegisterRe.FindStringSubmatch(trimmed); m != nil {
			prefixes = append(prefixes, m[1])
		}

		loc := fastifyShorthandRe.FindStringSubmatchIndex(trimmed)
		if loc == nil {
			continue
		}

		method := strings.ToUpper(trimmed[loc[2]:loc[3]])
		routePath := trimmed[loc[4]:loc[5]]

		// loc[1] is the position just after the matched prefix — extract handler.
		args, _ := jsExtractBalancedArgs(trimmed[loc[1]:])
		handler := cleanJSHandler(args)

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

	// --- Pass 2: fastify.route({...}) object-style, potentially multi-line ---
	fullText := strings.Join(lines, "\n")
	objRoutes := scanFastifyRouteObjects(fullText, path, lines, prefixes)
	routes = append(routes, objRoutes...)

	return routes, nil
}

// scanFastifyRouteObjects handles fastify.route({ method, url, handler }) calls
// that may span multiple lines by operating on the collapsed file text.
func scanFastifyRouteObjects(text, filePath string, lines []string, prefixes []string) []Route {
	collapsed := jsMultiSpaceRe.ReplaceAllString(text, " ")

	prefix := ""
	if len(prefixes) > 0 {
		prefix = prefixes[len(prefixes)-1]
	}

	var routes []Route

	// Find each fastify.route( occurrence and extract the balanced object body.
	locs := fastifyRouteRe.FindAllStringIndex(collapsed, -1)
	for _, loc := range locs {
		// loc[1] is just after "fastify.route(" — extract content up to matching ")".
		body, _ := jsExtractBalancedArgs(collapsed[loc[1]:])

		methodMatch := fastifyRouteMethodRe.FindStringSubmatch(body)
		urlMatch := fastifyRouteURLRe.FindStringSubmatch(body)
		handlerMatch := fastifyRouteHandlerRe.FindStringSubmatch(body)

		if urlMatch == nil {
			continue
		}

		routePath := joinPaths(prefix, urlMatch[1])

		handler := ""
		if handlerMatch != nil {
			handler = strings.TrimSpace(handlerMatch[1])
			handler = strings.TrimPrefix(handler, "async ")
			handler = strings.TrimSpace(handler)
		}

		// method can be a single value or an array; normalise to a slice.
		var methods []string
		if methodMatch != nil {
			switch {
			case methodMatch[1] != "":
				methods = append(methods, strings.ToUpper(methodMatch[1]))
			case methodMatch[2] != "":
				methods = append(methods, strings.ToUpper(methodMatch[2]))
			case methodMatch[3] != "":
				// Array like ['GET', 'POST'] — split on commas.
				for _, m := range strings.Split(methodMatch[3], ",") {
					m = strings.Trim(strings.TrimSpace(m), `'"`)
					if m != "" {
						methods = append(methods, strings.ToUpper(m))
					}
				}
			}
		}
		if len(methods) == 0 {
			methods = []string{"GET"}
		}

		lineNum := jsApproxLineNum(text, urlMatch[1])
		hasSwagger := jsLinesHaveSwagger(lines, lineNum-1, 20)

		for _, method := range methods {
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
