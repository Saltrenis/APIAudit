package analyze

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Saltrenis/APIAudit/internal/scan"
)

var (
	snakeCaseSegRe = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	camelCaseSegRe = regexp.MustCompile(`^[a-z][a-zA-Z0-9]+$`)
	kebabCaseSegRe = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)+$`)
	upperCaseSegRe = regexp.MustCompile(`[A-Z]`)
	pluralWordRe   = regexp.MustCompile(`s$`)
)

// CheckConsistency analyses a set of routes for naming and structural inconsistencies.
func CheckConsistency(routes []scan.Route) []Finding {
	var findings []Finding

	findings = append(findings, checkNamingConvention(routes)...)
	findings = append(findings, checkHTTPMethodUsage(routes)...)
	findings = append(findings, checkPluralConsistency(routes)...)

	return findings
}

// checkNamingConvention verifies that path segments use a single consistent casing style.
func checkNamingConvention(routes []scan.Route) []Finding {
	type styleCount struct{ snake, camel, kebab, other int }
	counts := styleCount{}

	for _, r := range routes {
		for _, seg := range pathSegments(r.Path) {
			if isPathParam(seg) {
				continue
			}
			switch {
			case snakeCaseSegRe.MatchString(seg):
				counts.snake++
			case kebabCaseSegRe.MatchString(seg):
				counts.kebab++
			case camelCaseSegRe.MatchString(seg):
				counts.camel++
			default:
				if upperCaseSegRe.MatchString(seg) {
					counts.other++
				}
			}
		}
	}

	dominant := dominantStyle(counts.snake, counts.camel, counts.kebab)

	var findings []Finding
	for i := range routes {
		r := &routes[i]
		for _, seg := range pathSegments(r.Path) {
			if isPathParam(seg) {
				continue
			}
			style := segmentStyle(seg)
			if style != "" && style != dominant {
				findings = append(findings, Finding{
					Category:   "api-inconsistency",
					Severity:   "P3",
					Route:      r,
					Message:    fmt.Sprintf("Path segment %q uses %s style; dominant style is %s", seg, style, dominant),
					File:       r.File,
					Line:       r.Line,
					Suggestion: fmt.Sprintf("Rename segment to use %s convention", dominant),
				})
			}
		}
	}
	return findings
}

// checkHTTPMethodUsage checks for common HTTP method misuse patterns.
func checkHTTPMethodUsage(routes []scan.Route) []Finding {
	var findings []Finding

	for i := range routes {
		r := &routes[i]
		path := strings.ToLower(r.Path)

		// POST used where DELETE is expected.
		if r.Method == "POST" && (strings.Contains(path, "/delete") || strings.Contains(path, "/remove")) {
			findings = append(findings, Finding{
				Category:   "api-inconsistency",
				Severity:   "P2",
				Route:      r,
				Message:    fmt.Sprintf("POST %s — path suggests deletion; prefer DELETE method", r.Path),
				File:       r.File,
				Line:       r.Line,
				Suggestion: "Use the DELETE HTTP method for resource deletion endpoints",
			})
		}

		// GET used for mutations.
		if r.Method == "GET" && (strings.Contains(path, "/create") || strings.Contains(path, "/update") || strings.Contains(path, "/edit")) {
			findings = append(findings, Finding{
				Category:   "api-inconsistency",
				Severity:   "P2",
				Route:      r,
				Message:    fmt.Sprintf("GET %s — path suggests mutation; GET should be read-only", r.Path),
				File:       r.File,
				Line:       r.Line,
				Suggestion: "Use POST/PUT/PATCH for endpoints that modify state",
			})
		}

		// PUT on a collection (no ID segment) suggests POST is more appropriate.
		if r.Method == "PUT" && !hasIDSegment(r.Path) {
			findings = append(findings, Finding{
				Category:   "api-inconsistency",
				Severity:   "P3",
				Route:      r,
				Message:    fmt.Sprintf("PUT %s targets a collection; consider POST for creation or include an ID", r.Path),
				File:       r.File,
				Line:       r.Line,
				Suggestion: "PUT should target a specific resource: PUT /resources/{id}",
			})
		}
	}

	return findings
}

// checkPluralConsistency warns when a mix of singular and plural resource names is used.
func checkPluralConsistency(routes []scan.Route) []Finding {
	type resourceInfo struct {
		plural bool
		route  *scan.Route
	}

	resources := make(map[string]*resourceInfo)

	for i := range routes {
		r := &routes[i]
		segs := pathSegments(r.Path)
		for _, seg := range segs {
			if isPathParam(seg) {
				continue
			}
			base := strings.ToLower(seg)
			singular := strings.TrimSuffix(base, "s")
			isPlural := pluralWordRe.MatchString(base) && len(base) > 2

			if existing, ok := resources[singular]; ok {
				if existing.plural != isPlural {
					// Conflict found.
					_ = existing
				}
			} else {
				resources[singular] = &resourceInfo{plural: isPlural, route: r}
			}
		}
	}

	// Collect inconsistencies: if same logical resource appears as both plural and singular.
	var findings []Finding
	checked := make(map[string]bool)

	for i := range routes {
		r := &routes[i]
		for _, seg := range pathSegments(r.Path) {
			if isPathParam(seg) {
				continue
			}
			base := strings.ToLower(seg)
			singular := strings.TrimSuffix(base, "s")
			isPlural := pluralWordRe.MatchString(base) && len(base) > 2

			if checked[singular] {
				continue
			}

			if info, ok := resources[singular]; ok && info.plural != isPlural {
				checked[singular] = true
				findings = append(findings, Finding{
					Category:   "api-inconsistency",
					Severity:   "P3",
					Route:      r,
					Message:    fmt.Sprintf("Inconsistent plurality for resource %q — found both singular and plural forms", singular),
					File:       r.File,
					Line:       r.Line,
					Suggestion: "Use plural nouns consistently for REST resource collections (e.g., /users, /orders)",
				})
			}
		}
	}

	return findings
}

// pathSegments splits a URL path into non-empty segments.
func pathSegments(path string) []string {
	var segs []string
	for _, s := range strings.Split(path, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}

// isPathParam reports whether a path segment is a parameter placeholder.
func isPathParam(seg string) bool {
	return strings.HasPrefix(seg, ":") ||
		(strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")) ||
		(strings.HasPrefix(seg, "<") && strings.HasSuffix(seg, ">")) ||
		strings.HasPrefix(seg, "*")
}

// hasIDSegment reports whether the path contains a parameter segment (suggests a specific resource).
func hasIDSegment(path string) bool {
	for _, seg := range pathSegments(path) {
		if isPathParam(seg) {
			return true
		}
	}
	return false
}

func segmentStyle(seg string) string {
	switch {
	case snakeCaseSegRe.MatchString(seg):
		return "snake_case"
	case kebabCaseSegRe.MatchString(seg):
		return "kebab-case"
	case camelCaseSegRe.MatchString(seg):
		return "camelCase"
	default:
		return ""
	}
}

func dominantStyle(snake, camel, kebab int) string {
	if snake >= camel && snake >= kebab {
		return "snake_case"
	}
	if kebab >= camel {
		return "kebab-case"
	}
	return "camelCase"
}
