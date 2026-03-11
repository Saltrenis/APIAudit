// Package beads provides an integration layer for the bd CLI issue tracker.
package beads

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Saltrenis/APIAudit/internal/analyze"
)

// IsInstalled reports whether the bd CLI is available in PATH.
func IsInstalled() bool {
	_, err := exec.LookPath("bd")
	return err == nil
}

// IsInitialized reports whether the target directory already has a .beads/ directory.
func IsInitialized(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".beads"))
	return err == nil
}

// Init runs `bd init` in the target directory.
func Init(dir string) error {
	cmd := exec.Command("bd", "init")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("beads: bd init: %w", err)
	}
	return nil
}

// CreateIssue shells out to `bd create` to create a new issue for the finding.
// It returns the new issue ID on success.
func CreateIssue(finding analyze.Finding) (string, error) {
	if !IsInstalled() {
		return "", fmt.Errorf("beads: bd CLI not found in PATH")
	}

	issueType := mapCategoryToType(finding.Category)
	priority := mapSeverityToPriority(finding.Severity)

	title := buildTitle(finding)
	body := buildBody(finding)

	args := []string{
		"create", title,
		"--type", issueType,
		"-p", priority,
		"--description", body,
	}

	cmd := exec.Command("bd", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("beads: bd create: %w", err)
	}

	// bd create typically prints the new issue ID on stdout.
	id := strings.TrimSpace(string(out))
	return id, nil
}

// mapCategoryToType maps a finding category to a beads issue type.
func mapCategoryToType(category string) string {
	switch category {
	case "missing-swagger":
		return "task"
	case "api-inconsistency":
		return "bug"
	case "endpoint-missing":
		return "bug"
	case "dead-code":
		return "chore"
	case "response-issue":
		return "bug"
	case "mock-data":
		return "bug"
	default:
		return "task"
	}
}

// mapSeverityToPriority converts P1-P4 to numeric priority values used by bd.
func mapSeverityToPriority(severity string) string {
	switch severity {
	case "P1":
		return "1"
	case "P2":
		return "2"
	case "P3":
		return "3"
	default:
		return "4"
	}
}

// buildTitle creates a concise issue title from a finding.
func buildTitle(finding analyze.Finding) string {
	if finding.Route != nil {
		return fmt.Sprintf("[%s] %s %s — %s",
			finding.Category, finding.Route.Method, finding.Route.Path, truncateTitle(finding.Message, 60))
	}
	return truncateTitle(finding.Message, 80)
}

// buildBody creates a multi-line description for the issue.
func buildBody(finding analyze.Finding) string {
	var sb strings.Builder

	sb.WriteString(finding.Message)
	sb.WriteString("\n\n")

	if finding.File != "" {
		sb.WriteString(fmt.Sprintf("File: %s", finding.File))
		if finding.Line > 0 {
			sb.WriteString(fmt.Sprintf(" (line %d)", finding.Line))
		}
		sb.WriteString("\n")
	}

	if finding.Route != nil {
		sb.WriteString(fmt.Sprintf("Route: %s %s\n", finding.Route.Method, finding.Route.Path))
		if finding.Route.Handler != "" {
			sb.WriteString(fmt.Sprintf("Handler: %s\n", finding.Route.Handler))
		}
	}

	if finding.Suggestion != "" {
		sb.WriteString("\nSuggestion:\n")
		sb.WriteString(finding.Suggestion)
		sb.WriteString("\n")
	}

	return sb.String()
}

func truncateTitle(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
