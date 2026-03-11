package report

import (
	"encoding/json"
	"fmt"

	"github.com/Saltrenis/APIAudit/internal/analyze"
	"github.com/Saltrenis/APIAudit/internal/detect"
	"github.com/Saltrenis/APIAudit/internal/scan"
)

// JSONReporter produces a structured JSON audit report.
type JSONReporter struct{}

// jsonReport is the top-level JSON output structure.
type jsonReport struct {
	Framework detect.Framework  `json:"framework"`
	Summary   jsonSummary       `json:"summary"`
	Routes    []scan.Route      `json:"routes"`
	Findings  []analyze.Finding `json:"findings"`
}

type jsonSummary struct {
	TotalRoutes int     `json:"totalRoutes"`
	Documented  int     `json:"documented"`
	Coverage    float64 `json:"coveragePct"`
	P1          int     `json:"p1"`
	P2          int     `json:"p2"`
	P3          int     `json:"p3"`
	P4          int     `json:"p4"`
}

// Report implements Reporter.
func (r *JSONReporter) Report(findings []analyze.Finding, routes []scan.Route, framework detect.Framework) (string, error) {
	p1, p2, p3, p4 := countBySeverity(findings)
	documented := countDocumented(routes)

	rep := jsonReport{
		Framework: framework,
		Summary: jsonSummary{
			TotalRoutes: len(routes),
			Documented:  documented,
			Coverage:    coverage(len(routes), documented),
			P1:          p1,
			P2:          p2,
			P3:          p3,
			P4:          p4,
		},
		Routes:   routes,
		Findings: findings,
	}

	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return "", fmt.Errorf("report: json marshal: %w", err)
	}

	return string(data), nil
}
