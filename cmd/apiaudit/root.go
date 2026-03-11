// Package main is the entry point for the apiaudit CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X main.version=x.y.z".
// It defaults to "dev" for local builds.
var version = "dev"

// globalFlags holds values bound to the persistent global flags.
var globalFlags struct {
	Dir        string
	Repo       string
	Output     string
	AIAssist   bool
	Format     string
	Beads      bool
	BeadsLimit int
}

// rootCmd is the base command. All sub-commands are attached to it.
var rootCmd = &cobra.Command{
	Use:     "apiaudit",
	Version: version,
	Short:   "apiaudit — detect, scan, and document your REST API",
	Long: `apiaudit is a zero-configuration CLI that detects your web framework,
extracts all route definitions, generates an OpenAPI spec, and reports
inconsistencies between your backend and frontend.

Quick start:
  apiaudit audit --dir ./my-project
  apiaudit scan  --dir ./my-project --format table
  apiaudit generate --dir ./my-project --output openapi.yaml`,
	SilenceUsage: true,
}

// Execute runs the root command and exits on failure.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&globalFlags.Dir, "dir", ".", "Target project directory")
	pf.StringVar(&globalFlags.Repo, "repo", "", "Git repository URL to clone before running")
	pf.StringVar(&globalFlags.Output, "output", "", "Write report to this file path instead of stdout")
	pf.BoolVar(&globalFlags.AIAssist, "ai-assist", false, "Use Claude for ambiguous analysis (requires ANTHROPIC_API_KEY)")
	pf.StringVar(&globalFlags.Format, "format", "table", "Output format: table | json | markdown")
	pf.BoolVar(&globalFlags.Beads, "beads", false, "Create beads issues for each finding")
	pf.IntVar(&globalFlags.BeadsLimit, "beads-limit", 50, "Maximum number of beads issues to create per run (0 = unlimited)")
}
