package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saltrenis/APIAudit/internal/analyze"
	"github.com/Saltrenis/APIAudit/internal/detect"
	"github.com/Saltrenis/APIAudit/internal/openapi"
	"github.com/Saltrenis/APIAudit/internal/repo"
	"github.com/Saltrenis/APIAudit/internal/scan"
	"github.com/spf13/cobra"
)

const defaultBaselineFile = ".apiaudit-baseline.json"

var diffFlags struct {
	Base         string
	Save         bool
	SkipGenerate bool
	StaticOnly   bool
	FrontendDir  string
}

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show only new findings compared to a saved baseline",
	Long: `Diff runs the full audit pipeline and reports only findings that are new
relative to a previously saved baseline. This makes it easy to track whether a
code change introduced new API issues without being overwhelmed by pre-existing ones.

A finding is considered new when no baseline finding matches on all of:
  category + route.method + route.path + message

Baseline workflow:
  apiaudit diff --save                  # save current state as baseline
  apiaudit diff                         # show only findings added since baseline
  apiaudit diff --base ./other.json     # compare against a specific baseline file

Examples:
  apiaudit diff --dir ./my-project --save
  apiaudit diff --dir ./my-project
  apiaudit diff --dir . --base ./ci-baseline.json --format markdown`,
	RunE: runDiff,
}

func init() {
	diffCmd.Flags().StringVar(&diffFlags.Base, "base", defaultBaselineFile, "Baseline file path (default: .apiaudit-baseline.json)")
	diffCmd.Flags().BoolVar(&diffFlags.Save, "save", false, "Save current findings as new baseline and exit")
	diffCmd.Flags().BoolVar(&diffFlags.SkipGenerate, "skip-generate", false, "Skip OpenAPI spec generation")
	diffCmd.Flags().BoolVar(&diffFlags.StaticOnly, "static-only", false, "Run coverage + consistency analysis only (no frontend)")
	diffCmd.Flags().StringVar(&diffFlags.FrontendDir, "frontend-dir", "", "Override detected frontend directory")
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, _ []string) error {
	dir := globalFlags.Dir

	// Clone if requested.
	if globalFlags.Repo != "" {
		cloned, cleanup, err := repo.TempClone(globalFlags.Repo)
		if err != nil {
			return fmt.Errorf("diff: clone: %w", err)
		}
		defer cleanup()
		dir = cloned
	}

	// Step 1: Detect.
	fmt.Fprintln(cmd.ErrOrStderr(), "→ Detecting framework...")
	fw, err := detect.Detect(dir)
	if err != nil {
		return fmt.Errorf("diff: detect: %w", err)
	}
	if fw.Confidence == 0 {
		return fmt.Errorf("diff: could not detect a supported framework in %s", dir)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "  Detected: %s %s (%.0f%% confidence)\n",
		fw.Language, fw.Framework, fw.Confidence*100)

	// Step 2: Scan.
	fmt.Fprintln(cmd.ErrOrStderr(), "→ Scanning routes...")
	scanner, err := scan.GetScanner(fw.Framework)
	if err != nil {
		return fmt.Errorf("diff: get scanner: %w", err)
	}
	routes, err := scanner.Scan(dir)
	if err != nil {
		return fmt.Errorf("diff: scan: %w", err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "  Found %d routes\n", len(routes))

	// Step 3: Generate OpenAPI spec.
	if !diffFlags.SkipGenerate && len(routes) > 0 {
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

	if !diffFlags.StaticOnly {
		frontendDir := diffFlags.FrontendDir
		if frontendDir == "" && fw.HasFrontend {
			frontendDir = filepath.Join(dir, fw.FrontendDir)
		}
		if frontendDir != "" {
			findings = append(findings, analyze.CheckFrontend(routes, frontendDir)...)
		}
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "  Found %d findings\n", len(findings))

	// Step 5: Save mode — write baseline and exit.
	if diffFlags.Save {
		baselinePath := resolveBaselinePath(diffFlags.Base, dir)
		if err := saveBaseline(findings, baselinePath); err != nil {
			return fmt.Errorf("diff: save baseline: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Baseline saved: %s (%d findings)\n", baselinePath, len(findings))
		return nil
	}

	// Step 6: Load baseline.
	baselinePath := resolveBaselinePath(diffFlags.Base, dir)
	baseline, baselineTotal, err := loadBaseline(baselinePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(cmd.ErrOrStderr(),
				"Warning: baseline file %s not found — showing all findings\n", baselinePath)
			// Show all findings as if they're all new.
			return diffReport(cmd, findings, findings, 0, fw, routes)
		}
		return fmt.Errorf("diff: load baseline: %w", err)
	}

	// Step 7: Compute new findings.
	newFindings := filterNew(findings, baseline)
	fmt.Fprintf(cmd.ErrOrStderr(),
		"Diff: %d new findings (baseline had %d total)\n", len(newFindings), baselineTotal)

	return diffReport(cmd, newFindings, findings, baselineTotal, fw, routes)
}

// diffReport writes the report of new findings.
func diffReport(cmd *cobra.Command, newFindings, _ []analyze.Finding, _ int, fw *detect.Framework, routes []scan.Route) error {
	reporter := buildReporter(globalFlags.Format)
	output, err := reporter.Report(newFindings, routes, *fw)
	if err != nil {
		return fmt.Errorf("diff: report: %w", err)
	}
	return writeOutput(output, globalFlags.Output)
}

// findingKey returns a stable deduplication key for a finding.
func findingKey(f analyze.Finding) string {
	var method, path string
	if f.Route != nil {
		method = f.Route.Method
		path = f.Route.Path
	}
	return strings.Join([]string{f.Category, method, path, f.Message}, "\x00")
}

// filterNew returns findings from current that have no matching entry in baseline.
func filterNew(current, baseline []analyze.Finding) []analyze.Finding {
	seen := make(map[string]struct{}, len(baseline))
	for _, f := range baseline {
		seen[findingKey(f)] = struct{}{}
	}

	var newFindings []analyze.Finding
	for _, f := range current {
		if _, ok := seen[findingKey(f)]; !ok {
			newFindings = append(newFindings, f)
		}
	}
	return newFindings
}

// saveBaseline writes findings to path as a JSON array.
func saveBaseline(findings []analyze.Finding, path string) error {
	data, err := json.MarshalIndent(findings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// loadBaseline reads a JSON findings array from path. It returns the parsed
// findings and the total count of items in the file.
func loadBaseline(path string) ([]analyze.Finding, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	var findings []analyze.Finding
	if err := json.Unmarshal(data, &findings); err != nil {
		return nil, 0, fmt.Errorf("parse %s: %w", path, err)
	}
	return findings, len(findings), nil
}

// resolveBaselinePath returns an absolute path for the baseline file. When the
// configured base path is the default (relative), it is placed under dir.
func resolveBaselinePath(base, dir string) string {
	if base == defaultBaselineFile {
		return filepath.Join(dir, base)
	}
	return base
}
