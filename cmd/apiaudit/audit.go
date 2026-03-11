package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Saltrenis/APIAudit/internal/analyze"
	beadspkg "github.com/Saltrenis/APIAudit/internal/beads"
	"github.com/Saltrenis/APIAudit/internal/detect"
	"github.com/Saltrenis/APIAudit/internal/openapi"
	"github.com/Saltrenis/APIAudit/internal/repo"
	"github.com/Saltrenis/APIAudit/internal/report"
	"github.com/Saltrenis/APIAudit/internal/scan"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var auditFlags struct {
	SkipFrontend bool
	SkipGenerate bool
	StaticOnly   bool
	FrontendDir  string
}

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Run the full audit pipeline: detect → scan → generate → analyze → report",
	Long: `Audit runs all stages in sequence:

  1. Detect  — identify the framework and project structure
  2. Scan    — extract every route from source code
  3. Generate — write an openapi.yaml to the project (unless --skip-generate)
  4. Analyze — check coverage, consistency, and frontend contract
  5. Report  — print findings in the requested format

Examples:
  apiaudit audit --dir ./my-project
  apiaudit audit --dir . --format markdown --output audit.md
  apiaudit audit --dir . --beads            # create beads issues for findings
  apiaudit audit --repo https://github.com/org/repo --format json
  apiaudit audit --dir . --static-only      # coverage + consistency only, no frontend`,
	RunE: runAudit,
}

func init() {
	auditCmd.Flags().BoolVar(&auditFlags.SkipFrontend, "skip-frontend", false, "Skip frontend contract analysis")
	auditCmd.Flags().BoolVar(&auditFlags.SkipGenerate, "skip-generate", false, "Skip OpenAPI spec generation")
	auditCmd.Flags().BoolVar(&auditFlags.StaticOnly, "static-only", false, "Run coverage + consistency analysis only (no frontend prompt)")
	auditCmd.Flags().StringVar(&auditFlags.FrontendDir, "frontend-dir", "", "Override detected frontend directory")
	rootCmd.AddCommand(auditCmd)
}

func runAudit(cmd *cobra.Command, _ []string) error {
	dir := globalFlags.Dir

	// Step 0: Clone if requested.
	if globalFlags.Repo != "" {
		cloned, cleanup, err := repo.TempClone(globalFlags.Repo)
		if err != nil {
			return fmt.Errorf("audit: clone: %w", err)
		}
		defer cleanup()
		dir = cloned
	}

	// Step 1: Detect.
	fmt.Fprintln(cmd.ErrOrStderr(), "→ Detecting framework...")
	fw, err := detect.Detect(dir)
	if err != nil {
		return fmt.Errorf("audit: detect: %w", err)
	}

	if fw.Confidence == 0 {
		return fmt.Errorf("audit: could not detect a supported framework in %s", dir)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "  Detected: %s %s (%.0f%% confidence)\n",
		fw.Language, fw.Framework, fw.Confidence*100)

	// Step 2: Scan.
	fmt.Fprintln(cmd.ErrOrStderr(), "→ Scanning routes...")
	scanner, err := scan.GetScanner(fw.Framework)
	if err != nil {
		return fmt.Errorf("audit: get scanner: %w", err)
	}

	routes, err := scanner.Scan(dir)
	if err != nil {
		return fmt.Errorf("audit: scan: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "  Found %d routes\n", len(routes))

	// Step 3: Generate OpenAPI spec.
	if !auditFlags.SkipGenerate && len(routes) > 0 {
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
	//
	// When no frontend is detected, warn the user and—unless --static-only was
	// passed—ask whether to continue with static analysis or stop after
	// reporting routes and the generated spec.
	skipFrontend := auditFlags.SkipFrontend || auditFlags.StaticOnly

	if !fw.HasFrontend && auditFlags.FrontendDir == "" && !skipFrontend {
		fmt.Fprintln(cmd.ErrOrStderr(), "⚠ No frontend detected. API contract testing is designed for frontend↔backend comparison.")

		runStatic, promptErr := promptStaticOnly(cmd)
		if promptErr != nil {
			return fmt.Errorf("audit: prompt: %w", promptErr)
		}

		if !runStatic {
			// User declined static analysis: report routes and spec only.
			reporter := buildReporter(globalFlags.Format)
			output, repErr := reporter.Report(nil, routes, *fw)
			if repErr != nil {
				return fmt.Errorf("audit: report: %w", repErr)
			}
			return writeOutput(output, globalFlags.Output)
		}

		// User accepted: treat this run as static-only.
		skipFrontend = true
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "→ Analyzing...")
	var findings []analyze.Finding

	findings = append(findings, analyze.CheckCoverage(routes)...)
	findings = append(findings, analyze.CheckConsistency(routes)...)

	if !skipFrontend {
		frontendDir := auditFlags.FrontendDir
		if frontendDir == "" && fw.HasFrontend {
			frontendDir = filepath.Join(dir, fw.FrontendDir)
		}
		frontendFindings := analyze.CheckFrontend(routes, frontendDir)
		findings = append(findings, frontendFindings...)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "  Found %d findings\n", len(findings))

	// Step 5: Optionally create beads issues.
	if globalFlags.Beads {
		if err := createBeadsIssues(cmd, findings, dir); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "  Warning: beads: %v\n", err)
		}
	}

	// Step 6: Report.
	reporter := buildReporter(globalFlags.Format)
	output, err := reporter.Report(findings, routes, *fw)
	if err != nil {
		return fmt.Errorf("audit: report: %w", err)
	}

	return writeOutput(output, globalFlags.Output)
}

// promptStaticOnly prints the static-analysis prompt to stderr and reads a
// single line from stdin. It returns true when the user accepts (or when stdin
// is not an interactive terminal, in which case it defaults to yes).
func promptStaticOnly(cmd *cobra.Command) (bool, error) {
	// When stdin is not a terminal (e.g. piped), default to yes silently.
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return true, nil
	}

	fmt.Fprint(cmd.ErrOrStderr(), "Would you like to run static analysis only? [Y/n] ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("read input: %w", err)
	}

	answer := strings.TrimSpace(strings.ToLower(line))
	// Empty input or explicit "y"/"yes" both mean yes.
	return answer == "" || answer == "y" || answer == "yes", nil
}

// buildReporter returns the Reporter matching the given format string.
func buildReporter(format string) report.Reporter {
	switch format {
	case "json":
		return &report.JSONReporter{}
	case "markdown":
		return &report.MarkdownReporter{}
	default:
		return &report.TableReporter{}
	}
}

// createBeadsIssues groups findings, deduplicates against existing open issues,
// respects the --beads-limit cap, and prints a summary when done.
func createBeadsIssues(cmd *cobra.Command, findings []analyze.Finding, dir string) error {
	if !beadspkg.IsInstalled() {
		return fmt.Errorf("bd CLI not found in PATH — install beads to use --beads flag")
	}

	if !beadspkg.IsInitialized(dir) {
		// Best-effort init.
		_ = beadspkg.Init(dir)
	}

	res, err := beadspkg.CreateIssues(findings, dir, globalFlags.BeadsLimit)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.ErrOrStderr(),
		"Beads: created %d, skipped %d (duplicates), skipped %d (limit reached)\n",
		res.Created, res.SkippedDupes, res.SkippedLimit)

	return nil
}
