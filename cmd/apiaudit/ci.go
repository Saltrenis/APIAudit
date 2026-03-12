package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Saltrenis/APIAudit/internal/analyze"
	"github.com/Saltrenis/APIAudit/internal/detect"
	"github.com/Saltrenis/APIAudit/internal/openapi"
	"github.com/Saltrenis/APIAudit/internal/repo"
	"github.com/Saltrenis/APIAudit/internal/scan"
	"github.com/spf13/cobra"
)

var ciFlags struct {
	FailOn       string
	SkipGenerate bool
	StaticOnly   bool
	FrontendDir  string
}

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "Run the audit pipeline and exit non-zero when findings exceed a threshold",
	Long: `CI runs the full audit pipeline (detect → scan → generate → analyze) and exits
with a non-zero status code when the number of findings at or above the configured
severity threshold is greater than zero. This is designed for use in CI pipelines.

The --fail-on flag controls the minimum severity level that triggers failure.
Accepted values: p1, p2, p3, p4 (case-insensitive). Default is p2.

Severity ordering: P1 (highest) > P2 > P3 > P4 (lowest)

Examples:
  apiaudit ci --dir ./my-project
  apiaudit ci --dir . --fail-on p1
  apiaudit ci --dir . --fail-on p3 --format json --output ci-report.json`,
	RunE: runCI,
}

func init() {
	ciCmd.Flags().StringVar(&ciFlags.FailOn, "fail-on", "p2", "Minimum severity that causes failure: p1, p2, p3, p4")
	ciCmd.Flags().BoolVar(&ciFlags.SkipGenerate, "skip-generate", false, "Skip OpenAPI spec generation")
	ciCmd.Flags().BoolVar(&ciFlags.StaticOnly, "static-only", false, "Run coverage + consistency analysis only (no frontend)")
	ciCmd.Flags().StringVar(&ciFlags.FrontendDir, "frontend-dir", "", "Override detected frontend directory")
	rootCmd.AddCommand(ciCmd)
}

// severityLevel converts a severity string to a numeric level for comparison.
// Lower number = higher severity (P1 = 1, P2 = 2, P3 = 3, P4 = 4).
func severityLevel(s string) int {
	switch strings.ToUpper(s) {
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	case "P4":
		return 4
	default:
		return 99
	}
}

func runCI(cmd *cobra.Command, _ []string) error {
	threshold := strings.ToLower(strings.TrimSpace(ciFlags.FailOn))
	switch threshold {
	case "p1", "p2", "p3", "p4":
		// valid
	default:
		return fmt.Errorf("ci: --fail-on must be one of: p1, p2, p3, p4 (got %q)", ciFlags.FailOn)
	}

	dir := globalFlags.Dir

	// Clone if requested.
	if globalFlags.Repo != "" {
		cloned, cleanup, err := repo.TempClone(globalFlags.Repo)
		if err != nil {
			return fmt.Errorf("ci: clone: %w", err)
		}
		defer cleanup()
		dir = cloned
	}

	// Step 1: Detect.
	fmt.Fprintln(cmd.ErrOrStderr(), "→ Detecting framework...")
	fw, err := detect.Detect(dir)
	if err != nil {
		return fmt.Errorf("ci: detect: %w", err)
	}
	if fw.Confidence == 0 {
		return fmt.Errorf("ci: could not detect a supported framework in %s", dir)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "  Detected: %s %s (%.0f%% confidence)\n",
		fw.Language, fw.Framework, fw.Confidence*100)

	// Step 2: Scan.
	fmt.Fprintln(cmd.ErrOrStderr(), "→ Scanning routes...")
	scanner, err := scan.GetScanner(fw.Framework)
	if err != nil {
		return fmt.Errorf("ci: get scanner: %w", err)
	}
	routes, err := scanner.Scan(dir)
	if err != nil {
		return fmt.Errorf("ci: scan: %w", err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "  Found %d routes\n", len(routes))

	// Step 3: Generate OpenAPI spec.
	if !ciFlags.SkipGenerate && len(routes) > 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "→ Generating OpenAPI spec...")
		info := openapi.Info{
			Title:   filepath.Base(filepath.Clean(dir)),
			Version: "1.0.0",
		}
		spec, genErr := openapi.Generate(routes, info)
		if genErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "  Warning: could not generate spec: %v\n", genErr)
		} else {
			specPath := filepath.Join(dir, "openapi.yaml")
			if writeErr := openapi.WriteYAML(spec, specPath); writeErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "  Warning: could not write spec: %v\n", writeErr)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "  Written: %s\n", specPath)
			}
		}
	}

	// Step 4: Analyze.
	fmt.Fprintln(cmd.ErrOrStderr(), "→ Analyzing...")
	var findings []analyze.Finding

	findings = append(findings, analyze.CheckCoverage(routes)...)
	findings = append(findings, analyze.CheckConsistency(routes)...)

	if !ciFlags.StaticOnly {
		frontendDir := ciFlags.FrontendDir
		if frontendDir == "" && fw.HasFrontend {
			frontendDir = filepath.Join(dir, fw.FrontendDir)
		}
		if frontendDir != "" {
			findings = append(findings, analyze.CheckFrontend(routes, frontendDir)...)
		}
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "  Found %d findings\n", len(findings))

	// Step 5: Count by severity and determine pass/fail.
	counts := map[string]int{"P1": 0, "P2": 0, "P3": 0, "P4": 0}
	for _, f := range findings {
		upper := strings.ToUpper(f.Severity)
		counts[upper]++
	}

	thresholdLevel := severityLevel(threshold)
	failCount := 0
	for sev, count := range counts {
		if severityLevel(sev) <= thresholdLevel {
			failCount += count
		}
	}

	// Step 6: Report full findings.
	// Always use JSON internally; respect --format for the written report.
	jsonData, err := json.MarshalIndent(findings, "", "  ")
	if err != nil {
		return fmt.Errorf("ci: marshal findings: %w", err)
	}

	reporter := buildReporter(globalFlags.Format)
	output, err := reporter.Report(findings, routes, *fw)
	if err != nil {
		return fmt.Errorf("ci: report: %w", err)
	}
	if err := writeOutput(output, globalFlags.Output); err != nil {
		return err
	}

	// Suppress unused variable warning — jsonData is kept for potential future
	// structured CI output; use it in the summary line.
	_ = jsonData

	// Step 7: Print CI summary to stderr.
	thresholdUpper := strings.ToUpper(threshold)
	if failCount > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"CI: %d P1, %d P2, %d P3, %d P4 findings — FAIL (threshold: %s)\n",
			counts["P1"], counts["P2"], counts["P3"], counts["P4"], thresholdUpper)
		// Exit non-zero via a sentinel that cobra will surface.
		return fmt.Errorf("ci: %d finding(s) at or above %s threshold", failCount, thresholdUpper)
	}

	fmt.Fprintf(cmd.ErrOrStderr(),
		"CI: %d P1, %d P2, %d P3, %d P4 findings — PASS (threshold: %s)\n",
		counts["P1"], counts["P2"], counts["P3"], counts["P4"], thresholdUpper)
	return nil
}
