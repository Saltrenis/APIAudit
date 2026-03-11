// Package report provides Reporter implementations for audit findings.
package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/Saltrenis/APIAudit/internal/analyze"
	"github.com/Saltrenis/APIAudit/internal/detect"
	"github.com/Saltrenis/APIAudit/internal/scan"
)

// Reporter generates a formatted report from audit findings.
type Reporter interface {
	Report(findings []analyze.Finding, routes []scan.Route, framework detect.Framework) (string, error)
}

// MarkdownReporter produces a Markdown-formatted audit report.
type MarkdownReporter struct{}

// Report implements Reporter.
func (r *MarkdownReporter) Report(findings []analyze.Finding, routes []scan.Route, framework detect.Framework) (string, error) {
	var sb strings.Builder

	now := time.Now().Format("2006-01-02 15:04:05")

	sb.WriteString("# API Audit Report\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", now))

	// Framework summary.
	sb.WriteString("## Framework\n\n")
	sb.WriteString(fmt.Sprintf("| Field | Value |\n|---|---|\n"))
	sb.WriteString(fmt.Sprintf("| Language | %s |\n", framework.Language))
	sb.WriteString(fmt.Sprintf("| Framework | %s |\n", framework.Framework))
	if framework.Version != "" {
		sb.WriteString(fmt.Sprintf("| Version | %s |\n", framework.Version))
	}
	sb.WriteString(fmt.Sprintf("| Confidence | %.0f%% |\n", framework.Confidence*100))
	sb.WriteString(fmt.Sprintf("| Has Frontend | %v |\n", framework.HasFrontend))
	sb.WriteString(fmt.Sprintf("| Has Swagger | %v |\n", framework.HasSwagger))
	sb.WriteString("\n")

	// Stats.
	p1, p2, p3, p4 := countBySeverity(findings)
	documented := countDocumented(routes)

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("| Metric | Value |\n|---|---|\n"))
	sb.WriteString(fmt.Sprintf("| Total Routes | %d |\n", len(routes)))
	sb.WriteString(fmt.Sprintf("| Documented | %d |\n", documented))
	sb.WriteString(fmt.Sprintf("| Coverage | %.0f%% |\n", coverage(len(routes), documented)))
	sb.WriteString(fmt.Sprintf("| Total Findings | %d |\n", len(findings)))
	sb.WriteString(fmt.Sprintf("| P1 | %d |\n", p1))
	sb.WriteString(fmt.Sprintf("| P2 | %d |\n", p2))
	sb.WriteString(fmt.Sprintf("| P3 | %d |\n", p3))
	sb.WriteString(fmt.Sprintf("| P4 | %d |\n", p4))
	sb.WriteString("\n")

	// Routes table.
	sb.WriteString("## Routes\n\n")
	sb.WriteString("| Method | Path | Handler | File | Swagger |\n|---|---|---|---|---|\n")
	for _, route := range routes {
		swagger := "No"
		if route.HasSwagger {
			swagger = "Yes"
		}
		sb.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | `%s:%d` | %s |\n",
			route.Method, route.Path, route.Handler, shortPath(route.File), route.Line, swagger))
	}
	sb.WriteString("\n")

	// Findings by category.
	if len(findings) == 0 {
		sb.WriteString("## Findings\n\nNo issues found.\n\n")
		return sb.String(), nil
	}

	sb.WriteString("## Findings\n\n")

	categories := []string{"missing-swagger", "api-inconsistency", "endpoint-missing", "dead-code", "response-issue", "mock-data"}
	for _, cat := range categories {
		catFindings := filterByCategory(findings, cat)
		if len(catFindings) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s (%d)\n\n", cat, len(catFindings)))
		for _, f := range catFindings {
			sb.WriteString(fmt.Sprintf("**[%s]** %s\n\n", f.Severity, f.Message))
			if f.File != "" {
				sb.WriteString(fmt.Sprintf("- File: `%s`", shortPath(f.File)))
				if f.Line > 0 {
					sb.WriteString(fmt.Sprintf(" (line %d)", f.Line))
				}
				sb.WriteString("\n")
			}
			if f.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("- Suggestion: %s\n", f.Suggestion))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

func countBySeverity(findings []analyze.Finding) (p1, p2, p3, p4 int) {
	for _, f := range findings {
		switch f.Severity {
		case "P1":
			p1++
		case "P2":
			p2++
		case "P3":
			p3++
		default:
			p4++
		}
	}
	return
}

func countDocumented(routes []scan.Route) int {
	n := 0
	for _, r := range routes {
		if r.HasSwagger {
			n++
		}
	}
	return n
}

func coverage(total, documented int) float64 {
	if total == 0 {
		return 100
	}
	return float64(documented) / float64(total) * 100
}

func filterByCategory(findings []analyze.Finding, cat string) []analyze.Finding {
	var result []analyze.Finding
	for _, f := range findings {
		if f.Category == cat {
			result = append(result, f)
		}
	}
	return result
}

func shortPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 3 {
		return "..." + strings.Join(parts[len(parts)-3:], "/")
	}
	return path
}
