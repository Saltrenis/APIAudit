package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Saltrenis/APIAudit/internal/detect"
	"github.com/Saltrenis/APIAudit/internal/openapi"
	"github.com/Saltrenis/APIAudit/internal/repo"
	"github.com/Saltrenis/APIAudit/internal/scan"
	"github.com/spf13/cobra"
)

var generateFlags struct {
	Title       string
	Version     string
	Description string
	JSONFormat  bool
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate an OpenAPI 3.0 spec from scanned routes",
	Long: `Generate detects the framework, scans all routes, and writes an OpenAPI 3.0
specification in YAML (default) or JSON format.

Examples:
  apiaudit generate --dir . --output openapi.yaml
  apiaudit generate --dir . --output openapi.json --json
  apiaudit generate --dir . --title "My API" --api-version "2.0.0"`,
	RunE: runGenerate,
}

func init() {
	generateCmd.Flags().StringVar(&generateFlags.Title, "title", "", "API title (default: directory name)")
	generateCmd.Flags().StringVar(&generateFlags.Version, "api-version", "1.0.0", "API version string")
	generateCmd.Flags().StringVar(&generateFlags.Description, "description", "", "API description")
	generateCmd.Flags().BoolVar(&generateFlags.JSONFormat, "json", false, "Write JSON instead of YAML")
	rootCmd.AddCommand(generateCmd)
}

func runGenerate(cmd *cobra.Command, _ []string) error {
	dir := globalFlags.Dir

	if globalFlags.Repo != "" {
		cloned, cleanup, err := repo.TempClone(globalFlags.Repo)
		if err != nil {
			return fmt.Errorf("generate: clone repo: %w", err)
		}
		defer cleanup()
		dir = cloned
	}

	fw, err := detect.Detect(dir)
	if err != nil {
		return fmt.Errorf("generate: detect: %w", err)
	}

	if fw.Framework == "unknown" || fw.Confidence == 0 {
		return fmt.Errorf("generate: could not detect a supported framework in %s", dir)
	}

	scanner, err := scan.GetScanner(fw.Framework)
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	routes, err := scanner.Scan(dir)
	if err != nil {
		return fmt.Errorf("generate: scan: %w", err)
	}

	title := generateFlags.Title
	if title == "" {
		title = filepath.Base(filepath.Clean(dir))
		title = strings.ReplaceAll(title, "-", " ")
		title = strings.Title(title)
	}

	info := openapi.Info{
		Title:       title,
		Version:     generateFlags.Version,
		Description: generateFlags.Description,
	}

	spec, err := openapi.Generate(routes, info)
	if err != nil {
		return fmt.Errorf("generate: build spec: %w", err)
	}

	outPath := globalFlags.Output
	if outPath == "" {
		if generateFlags.JSONFormat {
			outPath = "openapi.json"
		} else {
			outPath = "openapi.yaml"
		}
	}

	if generateFlags.JSONFormat || strings.HasSuffix(outPath, ".json") {
		if err := openapi.WriteJSON(spec, outPath); err != nil {
			return fmt.Errorf("generate: write json: %w", err)
		}
	} else {
		if err := openapi.WriteYAML(spec, outPath); err != nil {
			return fmt.Errorf("generate: write yaml: %w", err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Generated OpenAPI spec: %s (%d routes)\n", outPath, len(routes))
	return nil
}
