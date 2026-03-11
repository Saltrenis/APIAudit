package report

import (
	"fmt"
	"strings"

	"github.com/Saltrenis/APIAudit/internal/analyze"
	"github.com/Saltrenis/APIAudit/internal/detect"
	"github.com/Saltrenis/APIAudit/internal/scan"
)

// TableReporter produces a compact terminal table using only fmt.
type TableReporter struct{}

// Report implements Reporter.
func (r *TableReporter) Report(findings []analyze.Finding, routes []scan.Route, framework detect.Framework) (string, error) {
	var sb strings.Builder

	// Header.
	sb.WriteString("\n=== API Audit Report ===\n\n")

	// Framework block.
	sb.WriteString(fmt.Sprintf("Framework : %s %s", framework.Language, framework.Framework))
	if framework.Version != "" {
		sb.WriteString(fmt.Sprintf(" %s", framework.Version))
	}
	sb.WriteString(fmt.Sprintf(" (%.0f%% confidence)\n", framework.Confidence*100))
	if framework.HasFrontend {
		sb.WriteString(fmt.Sprintf("Frontend  : %s\n", framework.FrontendDir))
	}
	sb.WriteString(fmt.Sprintf("Swagger   : %v\n\n", framework.HasSwagger))

	// Summary.
	p1, p2, p3, p4 := countBySeverity(findings)
	documented := countDocumented(routes)
	sb.WriteString(fmt.Sprintf("Routes    : %d  (documented: %d / %.0f%%)\n",
		len(routes), documented, coverage(len(routes), documented)))
	sb.WriteString(fmt.Sprintf("Findings  : %d  [P1:%d  P2:%d  P3:%d  P4:%d]\n\n",
		len(findings), p1, p2, p3, p4))

	// Routes table.
	if len(routes) > 0 {
		sb.WriteString(tableRoutes(routes))
		sb.WriteString("\n")
	}

	// Findings table.
	if len(findings) > 0 {
		sb.WriteString(tableFindings(findings))
	} else {
		sb.WriteString("No findings — all checks passed.\n")
	}

	return sb.String(), nil
}

func tableRoutes(routes []scan.Route) string {
	// Column widths.
	const (
		wMethod  = 8
		wPath    = 40
		wHandler = 30
		wSwagger = 7
	)

	var sb strings.Builder

	header := fmt.Sprintf("%-*s %-*s %-*s %-*s",
		wMethod, "METHOD",
		wPath, "PATH",
		wHandler, "HANDLER",
		wSwagger, "SWAGGER",
	)
	sb.WriteString(header + "\n")
	sb.WriteString(strings.Repeat("-", len(header)) + "\n")

	for _, r := range routes {
		swagger := "No"
		if r.HasSwagger {
			swagger = "Yes"
		}
		row := fmt.Sprintf("%-*s %-*s %-*s %-*s",
			wMethod, truncate(r.Method, wMethod),
			wPath, truncate(r.Path, wPath),
			wHandler, truncate(r.Handler, wHandler),
			wSwagger, swagger,
		)
		sb.WriteString(row + "\n")
	}

	return sb.String()
}

func tableFindings(findings []analyze.Finding) string {
	const (
		wSeverity = 4
		wCategory = 22
		wMessage  = 60
	)

	var sb strings.Builder

	sb.WriteString("--- Findings ---\n")
	header := fmt.Sprintf("%-*s %-*s %-*s",
		wSeverity, "SEV",
		wCategory, "CATEGORY",
		wMessage, "MESSAGE",
	)
	sb.WriteString(header + "\n")
	sb.WriteString(strings.Repeat("-", len(header)) + "\n")

	for _, f := range findings {
		row := fmt.Sprintf("%-*s %-*s %-*s",
			wSeverity, f.Severity,
			wCategory, truncate(f.Category, wCategory),
			wMessage, truncate(f.Message, wMessage),
		)
		sb.WriteString(row + "\n")
		if f.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("         Suggestion: %s\n", truncate(f.Suggestion, 80)))
		}
	}

	return sb.String()
}

// truncate shortens s to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
