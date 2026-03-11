package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
//
// When outPath is a directory (or empty string), a dated filename is
// auto-generated using the current date and the active --format flag, e.g.
// "audit-results-2024-01-15.md". The file is placed inside outPath when it is
// a directory, or in the current working directory when outPath is empty.
func writeOutput(content, outPath string) error {
	resolved, err := resolveOutputPath(outPath, globalFlags.Format)
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	if resolved == "" {
		fmt.Print(content)
		return nil
	}

	if err := os.WriteFile(resolved, []byte(content), 0644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Report written to %s\n", resolved)
	return nil
}

// resolveOutputPath returns the file path to write to. When outPath is empty
// it returns an empty string (stdout). When outPath is an existing directory,
// or is empty but a path was implied, a dated filename is generated. When
// outPath is a specific file path it is returned unchanged.
func resolveOutputPath(outPath, format string) (string, error) {
	if outPath == "" {
		return "", nil
	}

	info, err := os.Stat(outPath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("stat %s: %w", outPath, err)
	}

	// If the path exists and is a directory, generate a filename inside it.
	if err == nil && info.IsDir() {
		return filepath.Join(outPath, datedFilename(format)), nil
	}

	// Path does not exist or is a file — use it as-is.
	return outPath, nil
}

// datedFilename returns a filename like "audit-results-2024-01-15.md" using
// the current date and the extension that matches the given format.
func datedFilename(format string) string {
	ext := "txt"
	switch format {
	case "json":
		ext = "json"
	case "markdown":
		ext = "md"
	case "table":
		ext = "txt"
	}
	date := time.Now().Format("2006-01-02")
	return fmt.Sprintf("audit-results-%s.%s", date, ext)
}
