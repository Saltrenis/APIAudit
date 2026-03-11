package scan

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// NestJSScanner extracts routes from NestJS TypeScript projects using decorator analysis.
type NestJSScanner struct{}

// Name implements Scanner.
func (s *NestJSScanner) Name() string { return "nestjs" }

// NestJS decorator patterns:
//
//	@Controller('/prefix')
//	@Controller({ path: 'prefix', version: '1' })
//	@Get('/path')
//	@Post('/path')
//	@Put(':id')
//	@Delete(':id')
var (
	// nestControllerStringRe matches the simple string form: @Controller('prefix')
	nestControllerStringRe = regexp.MustCompile(`@Controller\s*\(\s*['"]([^'"]*)['"]\s*\)`)

	// nestControllerObjectRe matches the object form: @Controller({ path: 'prefix', ... })
	// It captures only the path property value.
	nestControllerObjectRe = regexp.MustCompile(`@Controller\s*\(\s*\{[^}]*path\s*:\s*['"]([^'"]*)['"]\s*`)

	// nestControllerEmptyRe matches @Controller() with no argument.
	nestControllerEmptyRe = regexp.MustCompile(`@Controller\s*\(\s*\)`)

	nestRouteRe = regexp.MustCompile(
		`@(Get|Post|Put|Delete|Patch|Head|Options|All)\s*\(\s*(?:'([^']*)'|"([^"]*)")?\s*\)`,
	)
	nestMethodRe = regexp.MustCompile(`(?:async\s+)?(\w+)\s*\(`)

	// nestSwaggerRe matches NestJS @nestjs/swagger decorators as well as JSDoc-style markers.
	nestSwaggerRe = regexp.MustCompile(`@(ApiTags|ApiOperation|ApiResponse|ApiOkResponse|ApiCreatedResponse|ApiBearerAuth|ApiBody|ApiParam|ApiQuery|ApiProperty|ApiExcludeEndpoint|swagger|openapi)`)
)

// Scan implements Scanner.
func (s *NestJSScanner) Scan(dir string) ([]Route, error) {
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

		found, ferr := scanNestFile(path)
		if ferr != nil {
			return nil
		}
		routes = append(routes, found...)
		return nil
	})

	return routes, err
}

func scanNestFile(path string) ([]Route, error) {
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

	// Pass 1: find controller prefix.
	// The @Controller decorator may span multiple lines for the object form,
	// so we join lines to detect multi-line declarations.
	controllerPrefix := extractControllerPrefix(lines)

	// Pass 2: find method decorators and match to handler methods.
	var routes []Route
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		m := nestRouteRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		lineNum := i + 1
		httpMethod := strings.ToUpper(m[1])

		// m[2] = single-quoted path, m[3] = double-quoted path, both empty means root.
		subPath := m[2]
		if subPath == "" {
			subPath = m[3]
		}

		// Build the full path. normalizePrefix ensures the prefix starts with
		// "/" so joinPaths produces absolute paths (e.g. "/auth/me").
		// An empty prefix (bare @Controller()) is left as "" so joinPaths
		// returns the sub-path unchanged (or "/" for bare @Get()).
		prefix := controllerPrefix
		if prefix != "" && !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		fullPath := joinPaths(prefix, subPath)

		// Look ahead for the method name, skipping decorators and blank lines.
		// Use a larger window (15 lines) to accommodate stacked NestJS decorators.
		handler := ""
		for j := i + 1; j < len(lines) && j < i+15; j++ {
			nextTrimmed := strings.TrimSpace(lines[j])
			if nextTrimmed == "" || strings.HasPrefix(nextTrimmed, "@") {
				continue
			}
			if mm := nestMethodRe.FindStringSubmatch(nextTrimmed); mm != nil {
				handler = mm[1]
				break
			}
		}

		hasSwagger := nestLinesHaveSwagger(lines, i, 20)

		routes = append(routes, Route{
			Method:     httpMethod,
			Path:       fullPath,
			Handler:    handler,
			File:       path,
			Line:       lineNum,
			HasSwagger: hasSwagger,
		})
	}

	return routes, nil
}

// extractControllerPrefix scans all lines to find the @Controller decorator and
// returns the path prefix. It handles both the simple string form and the object
// form which may span multiple lines.
func extractControllerPrefix(lines []string) string {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "@Controller") {
			continue
		}

		// Build a combined string from this line and the next few to handle
		// multi-line object declarations like:
		//   @Controller({
		//     path: 'auth',
		//     version: '1',
		//   })
		combined := trimmed
		for j := i + 1; j < len(lines) && j < i+6; j++ {
			combined += " " + strings.TrimSpace(lines[j])
			// Stop once we see the closing paren of the decorator.
			if strings.Contains(combined, ")") {
				break
			}
		}

		// Try object form first (more specific).
		if m := nestControllerObjectRe.FindStringSubmatch(combined); m != nil {
			return m[1]
		}
		// Try simple string form.
		if m := nestControllerStringRe.FindStringSubmatch(combined); m != nil {
			return m[1]
		}
		// Empty @Controller() or unrecognized form — no prefix.
		if nestControllerEmptyRe.MatchString(combined) {
			return ""
		}
	}
	return ""
}

// nestLinesHaveSwagger checks for NestJS @nestjs/swagger decorators or JSDoc
// swagger annotations in the lookback window before lineIdx.
func nestLinesHaveSwagger(lines []string, lineIdx, lookback int) bool {
	start := lineIdx - lookback
	if start < 0 {
		start = 0
	}
	for _, l := range lines[start:lineIdx] {
		if nestSwaggerRe.MatchString(l) {
			return true
		}
	}
	return false
}
