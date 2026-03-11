// Package analyze provides route analysis and finding generation.
package analyze

import (
	"fmt"

	"github.com/Saltrenis/APIAudit/internal/scan"
)

// Finding represents a single detected issue or recommendation.
type Finding struct {
	Category   string      `json:"category"`
	Severity   string      `json:"severity"`
	Route      *scan.Route `json:"route,omitempty"`
	Message    string      `json:"message"`
	File       string      `json:"file,omitempty"`
	Line       int         `json:"line,omitempty"`
	Suggestion string      `json:"suggestion,omitempty"`
}

// CheckCoverage returns findings for routes that lack swagger/OpenAPI documentation.
func CheckCoverage(routes []scan.Route) []Finding {
	var findings []Finding

	total := len(routes)
	documented := 0

	for i := range routes {
		r := &routes[i]
		if r.HasSwagger {
			documented++
			continue
		}
		findings = append(findings, Finding{
			Category:   "missing-swagger",
			Severity:   coverageSeverity(r.Method),
			Route:      r,
			Message:    fmt.Sprintf("Route %s %s has no swagger/OpenAPI documentation", r.Method, r.Path),
			File:       r.File,
			Line:       r.Line,
			Suggestion: suggestSwaggerAnnotation(r),
		})
	}

	_ = total
	_ = documented

	return findings
}

// coverageSeverity assigns severity based on HTTP method importance.
func coverageSeverity(method string) string {
	switch method {
	case "POST", "PUT", "DELETE":
		return "P2"
	case "GET":
		return "P3"
	default:
		return "P4"
	}
}

// suggestSwaggerAnnotation returns a template swagger annotation for the route.
func suggestSwaggerAnnotation(r *scan.Route) string {
	return fmt.Sprintf(
		"Add swagger annotation above %s handler:\n// @Summary %s %s\n// @Tags api\n// @Produce json\n// @Success 200 {object} object\n// @Router %s [%s]",
		r.Handler, r.Method, r.Path, r.Path, r.Method,
	)
}
