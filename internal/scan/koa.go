package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// KoaScanner extracts routes from Node.js Koa projects (koa-router).
type KoaScanner struct{}

// Name implements Scanner.
func (s *KoaScanner) Name() string { return "koa" }

// Koa route patterns (koa-router):
//
//	router.get('/path', handler)
//	router.post('/path', middleware, handler)
//	const router = new Router({ prefix: '/api' })
//	router.use('/users', usersRouter.routes())
var (
	// koaRouteRe matches shorthand koa-router method calls:
	//   router.get('/path', ...)
	// Captures (1) method, (2) path.
	koaRouteRe = regexp.MustCompile(
		`(?i)(?:\w+)\s*\.\s*(get|post|put|delete|patch|head|options|all)\s*\(\s*['"]([^'"]+)['"]\s*,\s*`,
	)

	// koaNewRouterRe detects constructor calls that set a prefix:
	//   new Router({ prefix: '/api' })
	koaNewRouterRe = regexp.MustCompile(
		`(?i)new\s+Router\s*\(\s*\{\s*prefix\s*:\s*['"]([^'"]+)['"]`,
	)

	// koaUseRe detects nested router mounting:
	//   router.use('/users', usersRouter.routes())
	koaUseRe = regexp.MustCompile(
		`(?i)(?:\w+)\s*\.use\s*\(\s*['"]([^'"]+)['"]\s*,`,
	)
)

// Scan implements Scanner.
func (s *KoaScanner) Scan(dir string) ([]Route, error) {
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

		found, ferr := scanKoaFile(path)
		if ferr != nil {
			return nil
		}
		routes = append(routes, found...)
		return nil
	})

	return routes, err
}

func scanKoaFile(path string) ([]Route, error) {
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

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Detect new Router({ prefix: '/api' }) constructor.
		if m := koaNewRouterRe.FindStringSubmatch(trimmed); m != nil {
			prefixes = append(prefixes, m[1])
		}

		// Detect router.use('/prefix', nestedRouter.routes()) — track as a prefix.
		if m := koaUseRe.FindStringSubmatch(trimmed); m != nil {
			prefixes = append(prefixes, m[1])
		}

		loc := koaRouteRe.FindStringSubmatchIndex(trimmed)
		if loc == nil {
			continue
		}

		method := strings.ToUpper(trimmed[loc[2]:loc[3]])
		routePath := trimmed[loc[4]:loc[5]]

		// loc[1] is just after the path + comma — extract handler argument list.
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

	return routes, nil
}
