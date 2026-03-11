package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	beadspkg "github.com/Saltrenis/APIAudit/internal/beads"
	"github.com/Saltrenis/APIAudit/internal/detect"
	"github.com/spf13/cobra"
)

// apiauditConfig is the structure written to .apiaudit.json.
type apiauditConfig struct {
	Dir         string `json:"dir"`
	Format      string `json:"format"`
	Language    string `json:"language,omitempty"`
	Framework   string `json:"framework,omitempty"`
	FrontendDir string `json:"frontendDir,omitempty"`
	HasBeads    bool   `json:"hasBeads"`
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize beads and write an .apiaudit.json config in the target project",
	Long: `Init scans the target directory, detects the framework, creates an
.apiaudit.json configuration file, and optionally initialises beads issue
tracking if the bd CLI is available.

The generated config file stores defaults so subsequent apiaudit runs in the
same project do not require repeated flags.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, _ []string) error {
	dir := globalFlags.Dir

	// Detect framework so we can record it in config.
	fmt.Fprintln(cmd.OutOrStdout(), "Detecting framework...")
	fw, err := detect.Detect(dir)
	if err != nil {
		return fmt.Errorf("init: detect: %w", err)
	}

	hasBeads := false

	// Initialise beads if the CLI is available.
	if beadspkg.IsInstalled() {
		if beadspkg.IsInitialized(dir) {
			fmt.Fprintln(cmd.OutOrStdout(), "beads already initialized.")
			hasBeads = true
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Initializing beads...")
			if berr := beadspkg.Init(dir); berr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: beads init failed: %v\n", berr)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "beads initialized.")
				hasBeads = true
			}
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "bd CLI not found — skipping beads init (install from https://beads.dev)")
	}

	// Write config file.
	cfg := apiauditConfig{
		Dir:         ".",
		Format:      "table",
		Language:    fw.Language,
		Framework:   fw.Framework,
		FrontendDir: fw.FrontendDir,
		HasBeads:    hasBeads,
	}

	cfgPath := filepath.Join(dir, ".apiaudit.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("init: marshal config: %w", err)
	}

	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return fmt.Errorf("init: write config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nInitialized apiaudit for %s %s project.\n", fw.Language, fw.Framework)
	fmt.Fprintf(cmd.OutOrStdout(), "Config written to: %s\n\n", cfgPath)
	fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
	fmt.Fprintln(cmd.OutOrStdout(), "  apiaudit scan     — list all routes")
	fmt.Fprintln(cmd.OutOrStdout(), "  apiaudit generate — generate openapi.yaml")
	fmt.Fprintln(cmd.OutOrStdout(), "  apiaudit audit    — full audit with findings")

	return nil
}
