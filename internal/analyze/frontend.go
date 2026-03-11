package analyze

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Saltrenis/APIAudit/internal/scan"
)

// FrontendEndpoint represents an API call extracted from frontend source code.
type FrontendEndpoint struct {
	Method string
	Path   string
	File   string
	Line   int
}

var (
	// axiosRe matches axios.get('/url'), axios.post(`${base}/url`), etc.
	axiosRe = regexp.MustCompile(
		`axios\s*\.\s*(get|post|put|delete|patch|head|options)\s*\(\s*['"\x60]([^'"\x60]+)['"\x60]`,
	)
	// fetchRe matches fetch('/url', { method: 'POST' }) and fetch('/url').
	fetchURLRe    = regexp.MustCompile(`fetch\s*\(\s*['"\x60]([^'"\x60]+)['"\x60]`)
	fetchMethodRe = regexp.MustCompile(`method\s*:\s*['"]([A-Z]+)['"]`)
	// apiCallRe matches patterns like api.get('/url'), apiClient.post('/url').
	apiClientRe = regexp.MustCompile(
		`(?:api|apiClient|client|http|service)\s*\.\s*(get|post|put|delete|patch)\s*\(\s*['"\x60]([^'"\x60]+)['"\x60]`,
	)
	// template literal base URL removal: ${BASE_URL}/path → /path
	templateLiteralRe = regexp.MustCompile(`\$\{[^}]+\}`)
)

// CheckFrontend scans the frontend directory for API calls and compares them
// against the known backend routes.
func CheckFrontend(routes []scan.Route, frontendDir string) []Finding {
	if frontendDir == "" {
		return []Finding{{
			Category:   "endpoint-missing",
			Severity:   "P4",
			Message:    "No frontend directory detected — skipping frontend contract analysis",
			Suggestion: "Pass --frontend-dir or structure the project with a frontend/ directory",
		}}
	}

	endpoints, err := extractFrontendEndpoints(frontendDir)
	if err != nil {
		return []Finding{{
			Category:   "endpoint-missing",
			Severity:   "P4",
			Message:    fmt.Sprintf("Could not scan frontend directory %s: %v", frontendDir, err),
			Suggestion: "Check that the frontend directory is accessible",
		}}
	}

	if len(endpoints) == 0 {
		return []Finding{{
			Category:   "endpoint-missing",
			Severity:   "P4",
			Message:    "No API calls found in frontend source files",
			Suggestion: "Ensure frontend API clients are in api/, services/, or src/ directories",
		}}
	}

	return compareEndpoints(routes, endpoints)
}

// extractFrontendEndpoints walks frontendDir and collects all detected API calls.
func extractFrontendEndpoints(dir string) ([]FrontendEndpoint, error) {
	var endpoints []FrontendEndpoint

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == "dist" || name == "build" || name == ".nuxt" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isFrontendAPIFile(path) {
			return nil
		}

		found, ferr := scanFrontendFile(path)
		if ferr != nil {
			return nil
		}
		endpoints = append(endpoints, found...)
		return nil
	})

	return endpoints, err
}

// isFrontendAPIFile returns true for JS/TS files likely to contain API calls.
func isFrontendAPIFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".ts" && ext != ".js" && ext != ".vue" && ext != ".jsx" && ext != ".tsx" {
		return false
	}
	// Prefer api/, services/, stores/ directories but scan all JS/TS.
	return true
}

// scanFrontendFile extracts API endpoint calls from a single source file.
func scanFrontendFile(path string) ([]FrontendEndpoint, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var endpoints []FrontendEndpoint
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Axios method calls.
		if matches := axiosRe.FindAllStringSubmatch(line, -1); matches != nil {
			for _, m := range matches {
				ep := FrontendEndpoint{
					Method: strings.ToUpper(m[1]),
					Path:   normalizeFrontendPath(m[2]),
					File:   path,
					Line:   lineNum,
				}
				endpoints = append(endpoints, ep)
			}
		}

		// API client method calls (api.get, service.post, etc.).
		if matches := apiClientRe.FindAllStringSubmatch(line, -1); matches != nil {
			for _, m := range matches {
				ep := FrontendEndpoint{
					Method: strings.ToUpper(m[1]),
					Path:   normalizeFrontendPath(m[2]),
					File:   path,
					Line:   lineNum,
				}
				endpoints = append(endpoints, ep)
			}
		}

		// fetch() calls — method defaults to GET unless overridden on the same line.
		if m := fetchURLRe.FindStringSubmatch(line); m != nil {
			method := "GET"
			if mm := fetchMethodRe.FindStringSubmatch(line); mm != nil {
				method = strings.ToUpper(mm[1])
			}
			ep := FrontendEndpoint{
				Method: method,
				Path:   normalizeFrontendPath(m[1]),
				File:   path,
				Line:   lineNum,
			}
			endpoints = append(endpoints, ep)
		}
	}

	return endpoints, scanner.Err()
}

// normalizeFrontendPath strips template literal variables and cleans the path.
func normalizeFrontendPath(raw string) string {
	// Remove template literal expressions: ${variable}
	path := templateLiteralRe.ReplaceAllString(raw, "")
	// Remove trailing slashes left over from variable removal.
	path = strings.TrimRight(path, "/")
	// Ensure leading slash.
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

// compareEndpoints cross-references frontend calls against backend routes.
func compareEndpoints(backendRoutes []scan.Route, frontendEndpoints []FrontendEndpoint) []Finding {
	var findings []Finding

	// Build a lookup map for backend routes: "METHOD /path" → true.
	backendLookup := make(map[string]bool, len(backendRoutes))
	for _, r := range backendRoutes {
		key := strings.ToUpper(r.Method) + " " + normalizeLookupPath(r.Path)
		backendLookup[key] = true
	}

	// Build a lookup map for frontend endpoints.
	frontendLookup := make(map[string]bool, len(frontendEndpoints))
	for _, ep := range frontendEndpoints {
		key := ep.Method + " " + normalizeLookupPath(ep.Path)
		frontendLookup[key] = true
	}

	// Frontend calls without a matching backend route.
	for _, ep := range frontendEndpoints {
		key := ep.Method + " " + normalizeLookupPath(ep.Path)
		if !backendLookup[key] {
			findings = append(findings, Finding{
				Category:   "endpoint-missing",
				Severity:   "P2",
				Message:    fmt.Sprintf("Frontend calls %s %s but no matching backend route was found", ep.Method, ep.Path),
				File:       ep.File,
				Line:       ep.Line,
				Suggestion: fmt.Sprintf("Add a backend route: %s %s", ep.Method, ep.Path),
			})
		}
	}

	// Backend routes never called from frontend (potential dead code).
	for _, r := range backendRoutes {
		key := strings.ToUpper(r.Method) + " " + normalizeLookupPath(r.Path)
		if !frontendLookup[key] {
			findings = append(findings, Finding{
				Category:   "dead-code",
				Severity:   "P4",
				Route:      &r,
				Message:    fmt.Sprintf("Backend route %s %s is not called by any detected frontend code", r.Method, r.Path),
				File:       r.File,
				Line:       r.Line,
				Suggestion: "Verify this route is consumed by mobile apps, external clients, or is intentionally internal",
			})
		}
	}

	return findings
}

// normalizeLookupPath normalizes a path for comparison by replacing parameter
// placeholders with a canonical token.
func normalizeLookupPath(path string) string {
	var parts []string
	for _, seg := range strings.Split(path, "/") {
		if seg == "" {
			parts = append(parts, seg)
			continue
		}
		if strings.HasPrefix(seg, ":") ||
			(strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")) ||
			(strings.HasPrefix(seg, "<") && strings.HasSuffix(seg, ">")) {
			parts = append(parts, ":param")
		} else {
			parts = append(parts, seg)
		}
	}
	return strings.Join(parts, "/")
}
