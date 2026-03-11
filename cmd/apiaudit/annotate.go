package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Saltrenis/APIAudit/internal/detect"
	"github.com/Saltrenis/APIAudit/internal/repo"
	"github.com/Saltrenis/APIAudit/internal/scan"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const annotateOutputFile = ".apiaudit-annotate.json"

var annotateFlags struct {
	DryRun bool
}

var annotateCmd = &cobra.Command{
	Use:   "annotate",
	Short: "Find unannotated routes and prepare them for swagger annotation (AI-assisted)",
	Long: `Annotate runs the detect + scan pipeline, filters routes that lack swagger
documentation, and optionally invokes Claude to generate and insert annotations
into the source files.

Without --ai-assist, the command writes route data to ` + annotateOutputFile + ` and
prints instructions for running the Claude slash command manually.

With --ai-assist, it attempts to invoke 'claude /api-audit-annotate' directly.

Use --dry-run to print the unannotated routes without writing any files or
prompting for action.`,
	RunE: runAnnotate,
}

func init() {
	annotateCmd.Flags().BoolVar(&annotateFlags.DryRun, "dry-run", false, "Print unannotated routes without prompting or writing files")
	rootCmd.AddCommand(annotateCmd)
}

func runAnnotate(cmd *cobra.Command, _ []string) error {
	dir := globalFlags.Dir

	if globalFlags.Repo != "" {
		cloned, cleanup, err := repo.TempClone(globalFlags.Repo)
		if err != nil {
			return fmt.Errorf("annotate: clone repo: %w", err)
		}
		defer cleanup()
		dir = cloned
	}

	// Step 1: Detect framework.
	fw, err := detect.Detect(dir)
	if err != nil {
		return fmt.Errorf("annotate: detect: %w", err)
	}

	if fw.Framework == "unknown" || fw.Confidence == 0 {
		return fmt.Errorf("annotate: could not detect a supported framework in %s", dir)
	}

	// Step 2: Scan routes.
	scanner, err := scan.GetScanner(fw.Framework)
	if err != nil {
		return fmt.Errorf("annotate: %w", err)
	}

	routes, err := scanner.Scan(dir)
	if err != nil {
		return fmt.Errorf("annotate: scan: %w", err)
	}

	// Step 3: Filter to unannotated routes.
	var unannotated []scan.Route
	for _, r := range routes {
		if !r.HasSwagger {
			unannotated = append(unannotated, r)
		}
	}

	// Step 4: Print summary.
	total := len(routes)
	missing := len(unannotated)

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d unannotated routes out of %d total\n", missing, total)

	if missing == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "All routes are documented!")
		return nil
	}

	// Step 5: Print the list of unannotated routes.
	fmt.Fprintln(cmd.OutOrStdout())
	fmt.Fprintln(cmd.OutOrStdout(), "Unannotated routes:")

	const (
		wMethod = 8
		wPath   = 42
	)
	for _, r := range unannotated {
		filePart := fmt.Sprintf("%s:%d", shortFilePath(r.File), r.Line)
		fmt.Fprintf(cmd.OutOrStdout(), "  %-*s %-*s  %s\n",
			wMethod, r.Method,
			wPath, r.Path,
			filePart,
		)
	}

	// Dry-run stops here — no prompt, no file output.
	if annotateFlags.DryRun {
		return nil
	}

	// Step 6: Prompt the user.
	generate, err := promptGenerateAnnotations(cmd)
	if err != nil {
		return fmt.Errorf("annotate: prompt: %w", err)
	}

	if !generate {
		return nil
	}

	// Step 7: Write route data to JSON file.
	data, err := json.MarshalIndent(unannotated, "", "  ")
	if err != nil {
		return fmt.Errorf("annotate: marshal routes: %w", err)
	}

	outPath := annotateOutputFile
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("annotate: write %s: %w", outPath, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nRoute data written to %s\n", outPath)

	// Step 8: Invoke claude or print manual instructions.
	if globalFlags.AIAssist {
		if err := runClaudeAnnotate(cmd); err != nil {
			// Claude not found or failed — fall back to manual instructions.
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not invoke claude: %v\n", err)
			printManualInstructions(cmd)
		}
		return nil
	}

	printManualInstructions(cmd)
	return nil
}

// promptGenerateAnnotations asks the user whether to proceed with annotation
// generation. It defaults to yes when stdin is not a terminal.
func promptGenerateAnnotations(cmd *cobra.Command) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return true, nil
	}

	fmt.Fprint(cmd.OutOrStdout(), "\nWould you like to generate swagger annotations for these routes? [Y/n] ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("read input: %w", err)
	}

	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "" || answer == "y" || answer == "yes", nil
}

// runClaudeAnnotate attempts to invoke the claude CLI with the /api-audit-annotate
// slash command, attaching the current process's stdin/stdout/stderr.
func runClaudeAnnotate(cmd *cobra.Command) error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found in PATH: %w", err)
	}

	c := exec.Command(claudePath, "/api-audit-annotate")
	c.Stdin = os.Stdin
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()

	if err := c.Run(); err != nil {
		return fmt.Errorf("claude exited with error: %w", err)
	}

	return nil
}

// printManualInstructions prints the instructions for running the Claude slash
// command or annotating manually.
func printManualInstructions(cmd *cobra.Command) {
	fmt.Fprintln(cmd.OutOrStdout(), `
To generate annotations with Claude, run:
  claude /api-audit-annotate

Or manually add swagger annotations to the listed handler files.`)
}
