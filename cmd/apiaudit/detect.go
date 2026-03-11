package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Saltrenis/APIAudit/internal/detect"
	"github.com/Saltrenis/APIAudit/internal/repo"
	"github.com/spf13/cobra"
)

var detectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Detect the web framework used in a project",
	Long: `Detect scans the target directory and identifies the programming language,
web framework, version, and whether a frontend or existing swagger spec is present.`,
	RunE: runDetect,
}

func init() {
	rootCmd.AddCommand(detectCmd)
}

func runDetect(cmd *cobra.Command, _ []string) error {
	dir := globalFlags.Dir

	// Clone first if --repo was given.
	if globalFlags.Repo != "" {
		cloned, cleanup, err := repo.TempClone(globalFlags.Repo)
		if err != nil {
			return fmt.Errorf("detect: clone repo: %w", err)
		}
		defer cleanup()
		dir = cloned
	}

	fw, err := detect.Detect(dir)
	if err != nil {
		return fmt.Errorf("detect: %w", err)
	}

	output, err := formatDetectResult(fw, globalFlags.Format)
	if err != nil {
		return err
	}

	return writeOutput(output, globalFlags.Output)
}

func formatDetectResult(fw *detect.Framework, format string) (string, error) {
	switch format {
	case "json":
		data, err := json.MarshalIndent(fw, "", "  ")
		if err != nil {
			return "", fmt.Errorf("detect: marshal: %w", err)
		}
		return string(data), nil

	default: // table
		var rows [][]string
		rows = append(rows, []string{"Language", fw.Language})
		rows = append(rows, []string{"Framework", fw.Framework})
		if fw.Version != "" {
			rows = append(rows, []string{"Version", fw.Version})
		}
		rows = append(rows, []string{"Confidence", fmt.Sprintf("%.0f%%", fw.Confidence*100)})
		rows = append(rows, []string{"Has Frontend", fmt.Sprintf("%v", fw.HasFrontend)})
		if fw.HasFrontend {
			rows = append(rows, []string{"Frontend Dir", fw.FrontendDir})
		}
		rows = append(rows, []string{"Has Swagger", fmt.Sprintf("%v", fw.HasSwagger)})
		if len(fw.EntryPoints) > 0 {
			rows = append(rows, []string{"Entry Points", joinStrings(fw.EntryPoints)})
		}

		out := "=== Framework Detection ===\n\n"
		for _, row := range rows {
			out += fmt.Sprintf("%-16s %s\n", row[0]+":", row[1])
		}
		return out, nil
	}
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

// writeOutput writes content to outPath, or to stdout if outPath is empty.
func writeOutput(content, outPath string) error {
	if outPath == "" {
		fmt.Print(content)
		return nil
	}
	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Report written to %s\n", outPath)
	return nil
}
