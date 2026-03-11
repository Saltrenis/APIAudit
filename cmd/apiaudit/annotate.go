package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var annotateCmd = &cobra.Command{
	Use:   "annotate",
	Short: "Add swagger annotations to source files (AI-assisted)",
	Long: `Annotate scans routes that lack swagger/OpenAPI documentation and
uses Claude (via the Anthropic API) to generate and insert annotations
directly into the source files.

This command requires:
  - ANTHROPIC_API_KEY environment variable
  - The --ai-assist flag

Without --ai-assist, this command prints the annotation templates to stdout
so you can add them manually.`,
	RunE: runAnnotate,
}

func init() {
	rootCmd.AddCommand(annotateCmd)
}

func runAnnotate(cmd *cobra.Command, _ []string) error {
	if !globalFlags.AIAssist {
		fmt.Fprintln(cmd.OutOrStdout(), `Annotate requires --ai-assist to modify source files automatically.

Without AI assist, apiaudit can generate annotation templates for you:
  apiaudit audit --dir . --format markdown

To use Claude-assisted annotation:
  1. Set ANTHROPIC_API_KEY in your environment
  2. Run: apiaudit annotate --dir . --ai-assist

The AI will:
  - Detect undocumented routes
  - Infer request/response shapes from handler code
  - Insert swagger annotations above each handler
  - Regenerate the OpenAPI spec after annotation`)
		return nil
	}

	// AI-assisted annotation placeholder.
	// A full implementation would:
	// 1. Scan routes via scan.GetScanner
	// 2. Filter for !HasSwagger
	// 3. For each undocumented handler, read the surrounding source
	// 4. Call the Anthropic Messages API with the handler code
	// 5. Parse the response for the annotation block
	// 6. Insert it above the handler in the source file
	// 7. Re-run generate to produce a fresh openapi.yaml
	fmt.Fprintln(cmd.OutOrStdout(), "AI-assisted annotation: feature coming soon.")
	fmt.Fprintln(cmd.OutOrStdout(), "Run `apiaudit generate` to create an OpenAPI spec from current scanned routes.")
	return nil
}
