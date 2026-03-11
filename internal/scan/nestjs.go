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
//	@Get('/path')
//	@Post('/path')
//	@Put(':id')
//	@Delete(':id')
var (
	nestControllerRe       = regexp.MustCompile(`@Controller\s*\(\s*['"]([^'"]*)['"]\s*\)`)
	nestControllerNoPathRe = regexp.MustCompile(`@Controller\s*\(\s*\)`)
	nestRouteRe            = regexp.MustCompile(
		`@(Get|Post|Put|Delete|Patch|Head|Options|All)\s*\(\s*(?:'([^']*)'|"([^"]*)")?\s*\)`,
	)
	nestMethodRe = regexp.MustCompile(`(?:async\s+)?(\w+)\s*\(`)
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
		// NestJS controllers are typically *.controller.ts
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
	controllerPrefix := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := nestControllerRe.FindStringSubmatch(trimmed); m != nil {
			controllerPrefix = m[1]
			break
		}
		if nestControllerNoPathRe.MatchString(trimmed) {
			controllerPrefix = ""
			break
		}
	}

	// Pass 2: find method decorators and match to handler methods.
	var routes []Route
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		m := nestRouteRe.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}

		httpMethod := strings.ToUpper(m[1])
		// m[2] or m[3] holds the path (single vs double quote).
		subPath := m[2]
		if subPath == "" {
			subPath = m[3]
		}

		fullPath := joinPaths("/"+strings.TrimPrefix(controllerPrefix, "/"), subPath)

		// Look ahead for the method name (next non-decorator, non-blank line).
		handler := ""
		for j := i + 1; j < len(lines) && j < i+5; j++ {
			nextTrimmed := strings.TrimSpace(lines[j])
			if nextTrimmed == "" || strings.HasPrefix(nextTrimmed, "@") {
				continue
			}
			if mm := nestMethodRe.FindStringSubmatch(nextTrimmed); mm != nil {
				handler = mm[1]
				break
			}
		}

		hasSwagger := jsLinesHaveSwagger(lines, i, 20)

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
