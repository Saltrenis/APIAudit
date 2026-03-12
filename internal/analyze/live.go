package analyze

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Saltrenis/APIAudit/internal/scan"
)

// placeholderUUID is used to replace path parameters during live testing.
const placeholderUUID = "00000000-0000-0000-0000-000000000000"

// liveClient is a shared HTTP client with a conservative timeout. No auth
// headers are added — 401 and 403 responses are acceptable and not findings.
var liveClient = &http.Client{
	Timeout: 5 * time.Second,
}

// pathParamRe matches common path parameter styles: :id, {id}, <id>.
var pathParamRe = regexp.MustCompile(`(?::[^/]+|\{[^}]+\}|<[^>]+>)`)

// CheckLive hits each route's endpoint against baseURL and returns findings for
// server errors, missing routes, and non-JSON responses on API paths.
//
// For GET routes the full response is inspected. For non-GET routes only the
// status code from a HEAD request is checked; a 404 is reported as a P2 finding.
func CheckLive(routes []scan.Route, baseURL string) []Finding {
	baseURL = strings.TrimRight(baseURL, "/")

	var findings []Finding

	for i := range routes {
		r := &routes[i]
		targetURL := baseURL + normalizePath(r.Path)

		if r.Method == "GET" {
			findings = append(findings, checkGET(r, targetURL)...)
		} else {
			findings = append(findings, checkNonGET(r, targetURL)...)
		}
	}

	return findings
}

// normalizePath replaces path parameter placeholders with a UUID string so the
// URL is syntactically valid for an HTTP request.
func normalizePath(path string) string {
	return pathParamRe.ReplaceAllString(path, placeholderUUID)
}

// checkGET performs a GET request and evaluates the response for common issues.
func checkGET(r *scan.Route, url string) []Finding {
	resp, err := liveClient.Get(url) //nolint:noctx // intentional: no context needed for CLI live-test
	if err != nil {
		if isConnectionRefused(err) {
			return []Finding{connectionRefusedFinding(r, url)}
		}
		// Non-connection errors (DNS, TLS, redirect loops) — surface as P2.
		return []Finding{{
			Category: "live-test",
			Severity: "P2",
			Route:    r,
			Message:  fmt.Sprintf("GET %s request error: %v", r.Path, err),
		}}
	}
	defer resp.Body.Close()

	var findings []Finding

	switch {
	case resp.StatusCode >= 500:
		findings = append(findings, Finding{
			Category: "live-test",
			Severity: "P1",
			Route:    r,
			Message:  fmt.Sprintf("GET %s returned %s", r.Path, resp.Status),
		})

	case resp.StatusCode == http.StatusNotFound:
		findings = append(findings, Finding{
			Category:   "live-test",
			Severity:   "P2",
			Route:      r,
			Message:    fmt.Sprintf("GET %s returned 404 Not Found (route may not be registered)", r.Path),
			Suggestion: "Verify the route is mounted and the path matches the scanned definition",
		})
	}

	// Non-JSON content type on a non-404/5xx response — only flag 2xx responses
	// to avoid noise from redirect/error pages.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "application/json") && !strings.Contains(ct, "application/ld+json") {
			findings = append(findings, Finding{
				Category:   "live-test",
				Severity:   "P2",
				Route:      r,
				Message:    fmt.Sprintf("GET %s returned non-JSON content type: %q", r.Path, ct),
				Suggestion: "API endpoints should return application/json; check handler response headers",
			})
		}
	}

	return findings
}

// checkNonGET sends a HEAD request and flags 404 responses for non-GET routes.
func checkNonGET(r *scan.Route, url string) []Finding {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return nil // malformed URL — skip silently
	}

	resp, err := liveClient.Do(req)
	if err != nil {
		if isConnectionRefused(err) {
			return []Finding{connectionRefusedFinding(r, url)}
		}
		return nil // non-GET errors are not findings without a full body
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return []Finding{{
			Category:   "live-test",
			Severity:   "P2",
			Route:      r,
			Message:    fmt.Sprintf("%s %s returned 404 Not Found (route may not be registered)", r.Method, r.Path),
			Suggestion: "Verify the route is mounted and the path matches the scanned definition",
		}}
	}

	return nil
}

// isConnectionRefused reports whether an error represents a refused TCP connection.
func isConnectionRefused(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "connect: connection refused"))
}

// connectionRefusedFinding returns a P1 finding for an unreachable server.
func connectionRefusedFinding(r *scan.Route, url string) Finding {
	// Extract just the base URL (scheme + host) from the full URL for clarity.
	baseURL := url
	if idx := strings.Index(url, r.Path); idx > 0 {
		baseURL = url[:idx]
	}
	return Finding{
		Category:   "live-test",
		Severity:   "P1",
		Route:      r,
		Message:    fmt.Sprintf("Cannot connect to %s (connection refused)", baseURL),
		Suggestion: "Ensure the server is running and --live base URL is correct",
	}
}
