package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Saltrenis/APIAudit/internal/analyze"
	"github.com/Saltrenis/APIAudit/internal/detect"
	"github.com/Saltrenis/APIAudit/internal/openapi"
	"github.com/Saltrenis/APIAudit/internal/scan"
	"github.com/spf13/cobra"
)

var watchFlags struct {
	Interval     time.Duration
	SkipGenerate bool
	StaticOnly   bool
	FrontendDir  string
	Clear        bool
}

// watchExtensions lists file extensions that trigger a re-run when modified.
var watchExtensions = map[string]bool{
	".go":   true,
	".ts":   true,
	".tsx":  true,
	".js":   true,
	".jsx":  true,
	".py":   true,
	".mod":  true,
	".sum":  true,
	".json": true,
	".yaml": true,
	".yml":  true,
}

// watchSkipDirs lists directory names to skip when walking for changes.
var watchSkipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	"__pycache__":  true,
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Re-run the audit whenever source files change",
	Long: `Watch polls the project directory at regular intervals and re-runs the full
audit pipeline whenever relevant source files are modified.

File types monitored: .go .ts .tsx .js .jsx .py .mod .sum .json .yaml .yml
Directories skipped:  node_modules .git vendor dist build __pycache__

Press Ctrl+C to stop watching.

Examples:
  apiaudit watch --dir ./my-project
  apiaudit watch --dir . --interval 5s --clear
  apiaudit watch --dir . --static-only --format markdown`,
	RunE: runWatch,
}

func init() {
	watchCmd.Flags().DurationVar(&watchFlags.Interval, "interval", 3*time.Second, "Polling interval (e.g. 3s, 10s)")
	watchCmd.Flags().BoolVar(&watchFlags.SkipGenerate, "skip-generate", false, "Skip OpenAPI spec generation on each re-run")
	watchCmd.Flags().BoolVar(&watchFlags.StaticOnly, "static-only", false, "Run coverage + consistency analysis only (no frontend)")
	watchCmd.Flags().StringVar(&watchFlags.FrontendDir, "frontend-dir", "", "Override detected frontend directory")
	watchCmd.Flags().BoolVar(&watchFlags.Clear, "clear", false, "Clear terminal before each re-run")
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, _ []string) error {
	dir := globalFlags.Dir

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("watch: resolve dir: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Watching %s... (press Ctrl+C to stop)\n", absDir)

	// Run the initial audit.
	if err := runWatchAudit(cmd, absDir); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Audit error: %v\n", err)
	}

	// Snapshot mtime state after initial run.
	snapshot, err := snapshotMtimes(absDir)
	if err != nil {
		return fmt.Errorf("watch: snapshot: %w", err)
	}

	// Set up SIGINT / SIGTERM handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(watchFlags.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-sigCh:
			fmt.Fprintln(cmd.ErrOrStderr(), "\nWatch stopped.")
			return nil

		case <-ticker.C:
			current, err := snapshotMtimes(absDir)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Watch: snapshot error: %v\n", err)
				continue
			}

			if !mtimesChanged(snapshot, current) {
				continue
			}

			snapshot = current

			if watchFlags.Clear {
				fmt.Fprint(cmd.ErrOrStderr(), "\033[2J\033[H")
			}

			ts := time.Now().Format("15:04:05")
			fmt.Fprintf(cmd.ErrOrStderr(), "[%s] Change detected, re-running audit...\n", ts)

			if err := runWatchAudit(cmd, absDir); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Audit error: %v\n", err)
			}
		}
	}
}

// runWatchAudit executes a single audit pass for the watch command.
func runWatchAudit(cmd *cobra.Command, dir string) error {
	// Step 1: Detect.
	fw, err := detect.Detect(dir)
	if err != nil {
		return fmt.Errorf("detect: %w", err)
	}
	if fw.Confidence == 0 {
		return fmt.Errorf("could not detect a supported framework in %s", dir)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "  Detected: %s %s (%.0f%% confidence)\n",
		fw.Language, fw.Framework, fw.Confidence*100)

	// Step 2: Scan.
	scanner, err := scan.GetScanner(fw.Framework)
	if err != nil {
		return fmt.Errorf("get scanner: %w", err)
	}
	routes, err := scanner.Scan(dir)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "  Found %d routes\n", len(routes))

	// Step 3: Generate OpenAPI spec.
	if !watchFlags.SkipGenerate && len(routes) > 0 {
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
			}
		}
	}

	// Step 4: Analyze.
	var findings []analyze.Finding

	findings = append(findings, analyze.CheckCoverage(routes)...)
	findings = append(findings, analyze.CheckConsistency(routes)...)

	if !watchFlags.StaticOnly {
		frontendDir := watchFlags.FrontendDir
		if frontendDir == "" && fw.HasFrontend {
			frontendDir = filepath.Join(dir, fw.FrontendDir)
		}
		if frontendDir != "" {
			findings = append(findings, analyze.CheckFrontend(routes, frontendDir)...)
		}
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "  Found %d findings\n", len(findings))

	// Step 5: Report.
	reporter := buildReporter(globalFlags.Format)
	output, err := reporter.Report(findings, routes, *fw)
	if err != nil {
		return fmt.Errorf("report: %w", err)
	}
	return writeOutput(output, globalFlags.Output)
}

// fileMtime holds modification time info for a file.
type fileMtime struct {
	path    string
	modTime time.Time
}

// snapshotMtimes walks dir and records the mtime of every relevant source file.
func snapshotMtimes(dir string) (map[string]time.Time, error) {
	mtimes := make(map[string]time.Time)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			if watchSkipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if watchExtensions[strings.ToLower(filepath.Ext(path))] {
			mtimes[path] = info.ModTime()
		}
		return nil
	})
	return mtimes, err
}

// mtimesChanged reports whether any file was added, removed, or modified.
func mtimesChanged(old, current map[string]time.Time) bool {
	if len(old) != len(current) {
		return true
	}
	for path, mt := range current {
		if old[path] != mt {
			return true
		}
	}
	return false
}
