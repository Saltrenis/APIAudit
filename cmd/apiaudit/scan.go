package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Saltrenis/APIAudit/internal/detect"
	"github.com/Saltrenis/APIAudit/internal/repo"
	"github.com/Saltrenis/APIAudit/internal/scan"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan project routes and print a route table",
	Long: `Scan detects the framework, walks the source tree, and extracts every
registered route including its HTTP method, path, handler name, source file,
and line number.`,
	RunE: runScan,
}

func init() {
	rootCmd.AddCommand(scanCmd)
}

func runScan(cmd *cobra.Command, _ []string) error {
	dir := globalFlags.Dir

	if globalFlags.Repo != "" {
		cloned, cleanup, err := repo.TempClone(globalFlags.Repo)
		if err != nil {
			return fmt.Errorf("scan: clone repo: %w", err)
		}
		defer cleanup()
		dir = cloned
	}

	fw, err := detect.Detect(dir)
	if err != nil {
		return fmt.Errorf("scan: detect: %w", err)
	}

	if fw.Framework == "unknown" || fw.Confidence == 0 {
		return fmt.Errorf("scan: could not detect a supported framework in %s", dir)
	}

	scanner, err := scan.GetScanner(fw.Framework)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	routes, err := scanner.Scan(dir)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	output, err := formatRoutes(routes, fw, globalFlags.Format)
	if err != nil {
		return err
	}

	return writeOutput(output, globalFlags.Output)
}

func formatRoutes(routes []scan.Route, fw *detect.Framework, format string) (string, error) {
	switch format {
	case "json":
		data, err := json.MarshalIndent(routes, "", "  ")
		if err != nil {
			return "", fmt.Errorf("scan: marshal: %w", err)
		}
		return string(data), nil

	case "markdown":
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# Routes — %s %s\n\n", fw.Language, fw.Framework))
		sb.WriteString("| Method | Path | Handler | File | Swagger |\n")
		sb.WriteString("|--------|------|---------|------|---------|\n")
		for _, r := range routes {
			sw := "No"
			if r.HasSwagger {
				sw = "Yes"
			}
			sb.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | `%s:%d` | %s |\n",
				r.Method, r.Path, r.Handler, shortFilePath(r.File), r.Line, sw))
		}
		return sb.String(), nil

	default: // table
		if len(routes) == 0 {
			return fmt.Sprintf("No routes found in %s %s project.\n", fw.Language, fw.Framework), nil
		}

		const (
			wMethod  = 8
			wPath    = 42
			wHandler = 32
			wFile    = 35
			wSwagger = 7
		)

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Framework: %s %s  |  Routes: %d\n\n", fw.Language, fw.Framework, len(routes)))

		header := fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s",
			wMethod, "METHOD", wPath, "PATH", wHandler, "HANDLER", wFile, "FILE", wSwagger, "SWAGGER")
		sb.WriteString(header + "\n")
		sb.WriteString(strings.Repeat("-", len(header)) + "\n")

		for _, r := range routes {
			sw := "No"
			if r.HasSwagger {
				sw = "Yes"
			}
			filePart := fmt.Sprintf("%s:%d", shortFilePath(r.File), r.Line)
			row := fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s",
				wMethod, truncStr(r.Method, wMethod),
				wPath, truncStr(r.Path, wPath),
				wHandler, truncStr(r.Handler, wHandler),
				wFile, truncStr(filePart, wFile),
				wSwagger, sw,
			)
			sb.WriteString(row + "\n")
		}
		return sb.String(), nil
	}
}

func shortFilePath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 4 {
		return strings.Join(parts[len(parts)-4:], "/")
	}
	return path
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
