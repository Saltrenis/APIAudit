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

// ListOpenTitles returns the set of open issue titles by running `bd list --status=open`.
// The returned map keys are lower-cased titles for case-insensitive comparison.
func ListOpenTitles(dir string) (map[string]struct{}, error) {
	cmd := exec.Command("bd", "list", "--status=open")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// bd may exit non-zero when there are no issues; treat that as empty.
		return map[string]struct{}{}, nil
	}

	titles := make(map[string]struct{})
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// bd list output format: "<id>  <title>" or similar table rows.
		// Strip the leading ID token (first whitespace-separated field) to get the title.
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			title := strings.ToLower(strings.TrimSpace(parts[1]))
			if title != "" {
				titles[title] = struct{}{}
			}
		}
	}
	return titles, nil
}

// ListOpenIssues returns the titles of all open issues by running `bd list --status=open`.
// It is a slice-returning complement to ListOpenTitles for callers that prefer a []string.
func ListOpenIssues(dir string) ([]string, error) {
	cmd := exec.Command("bd", "list", "--status=open")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// bd may exit non-zero when there are no issues; treat that as empty.
		return nil, nil
	}

	var titles []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// bd list output format: "<id>  <title>" — strip the leading ID token.
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			title := strings.TrimSpace(parts[1])
			if title != "" {
				titles = append(titles, title)
			}
		}
	}
	return titles, nil
}

// IsDuplicate reports whether title already exists among existing issue titles.
// Comparison is case-insensitive.
func IsDuplicate(title string, existing []string) bool {
	needle := strings.ToLower(title)
	for _, t := range existing {
		if strings.ToLower(t) == needle {
			return true
		}
	}
	return false
}

// GroupedFinding represents one or more findings that share the same category and file,
// collapsed into a single beads issue.
type GroupedFinding struct {
	Category string
	File     string
	Severity string
	Findings []analyze.Finding
}

// GroupFindings collapses findings that share the same category and file into
// a single GroupedFinding.  Findings with no file are each kept separate.
func GroupFindings(findings []analyze.Finding) []GroupedFinding {
	type key struct {
		category string
		file     string
	}

	order := make([]key, 0, len(findings))
	grouped := make(map[key]*GroupedFinding)
	noFileSeq := 0

	for _, f := range findings {
		// Only group when a file is present; otherwise keep each finding separate
		// using a unique synthetic key.
		if f.File == "" {
			noFileSeq++
			k := key{category: f.Category, file: fmt.Sprintf("\x00nofile\x00%d", noFileSeq)}
			order = append(order, k)
			grouped[k] = &GroupedFinding{
				Category: f.Category,
				File:     "",
				Severity: f.Severity,
				Findings: []analyze.Finding{f},
			}
			continue
		}

		k := key{category: f.Category, file: f.File}
		if _, exists := grouped[k]; !exists {
			order = append(order, k)
			grouped[k] = &GroupedFinding{
				Category: f.Category,
				File:     f.File,
				Severity: f.Severity,
				Findings: []analyze.Finding{},
			}
		}
		gf := grouped[k]
		gf.Findings = append(gf.Findings, f)
		// Use the highest priority (lowest numeric P value) seen in the group.
		if compareSeverity(f.Severity, gf.Severity) < 0 {
			gf.Severity = f.Severity
		}
	}

	result := make([]GroupedFinding, 0, len(order))
	for _, k := range order {
		result = append(result, *grouped[k])
	}
	return result
}

// compareSeverity returns negative if a is higher priority than b (e.g. P1 < P2).
func compareSeverity(a, b string) int {
	return severityRank(a) - severityRank(b)
}

func severityRank(s string) int {
	switch s {
	case "P1":
		return 1
	case "P2":
		return 2
	case "P3":
		return 3
	default:
		return 4
	}
}

// CreateResult holds the outcome of a bulk issue creation run.
type CreateResult struct {
	Created      int
	SkippedDupes int
	SkippedLimit int
}

// CreateIssues creates beads issues for the given findings, applying deduplication
// against existing open issues and capping creation at limit.  It prints a dot to
// stderr for each issue created so the caller sees progress.  When limit <= 0 no
// cap is applied.
func CreateIssues(findings []analyze.Finding, dir string, limit int) (CreateResult, error) {
	if !IsInstalled() {
		return CreateResult{}, fmt.Errorf("beads: bd CLI not found in PATH")
	}

	openTitles, err := ListOpenTitles(dir)
	if err != nil {
		return CreateResult{}, fmt.Errorf("beads: list open issues: %w", err)
	}

	groups := GroupFindings(findings)

	var res CreateResult
	for _, g := range groups {
		title := buildGroupTitle(g)
		titleKey := strings.ToLower(title)

		if _, exists := openTitles[titleKey]; exists {
			res.SkippedDupes++
			continue
		}

		if limit > 0 && res.Created >= limit {
			res.SkippedLimit++
			continue
		}

		if _, err := createGroupIssue(g, dir); err != nil {
			fmt.Fprintf(os.Stderr, "\n  Warning: could not create beads issue: %v\n", err)
			continue
		}

		// Mark as known so subsequent identical titles in this run are also deduplicated.
		openTitles[titleKey] = struct{}{}
		res.Created++
		fmt.Fprint(os.Stderr, ".")
	}

	if res.Created > 0 {
		// Newline after the progress dots.
		fmt.Fprintln(os.Stderr)
	}

	return res, nil
}

// CreateIssue shells out to `bd create` to create a new issue for a single finding.
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

// createGroupIssue creates a single beads issue for a GroupedFinding.
func createGroupIssue(g GroupedFinding, dir string) (string, error) {
	issueType := mapCategoryToType(g.Category)
	priority := mapSeverityToPriority(g.Severity)
	title := buildGroupTitle(g)
	body := buildGroupBody(g)

	args := []string{
		"create", title,
		"--type", issueType,
		"-p", priority,
		"--description", body,
	}

	cmd := exec.Command("bd", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("beads: bd create: %w", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// buildGroupTitle produces a concise title for a grouped finding.
func buildGroupTitle(g GroupedFinding) string {
	if len(g.Findings) == 1 {
		return buildTitle(g.Findings[0])
	}
	if g.File != "" {
		return fmt.Sprintf("[%s] %d findings in %s",
			g.Category, len(g.Findings), filepath.Base(g.File))
	}
	return fmt.Sprintf("[%s] %d findings", g.Category, len(g.Findings))
}

// buildGroupBody produces a multi-line description for a grouped finding.
func buildGroupBody(g GroupedFinding) string {
	if len(g.Findings) == 1 {
		return buildBody(g.Findings[0])
	}

	var sb strings.Builder

	if g.File != "" {
		sb.WriteString(fmt.Sprintf("File: %s\n\n", g.File))
	}

	sb.WriteString(fmt.Sprintf("Category: %s | %d findings grouped\n\n", g.Category, len(g.Findings)))

	for i, f := range g.Findings {
		sb.WriteString(fmt.Sprintf("--- Finding %d ---\n", i+1))
		sb.WriteString(f.Message)
		sb.WriteString("\n")

		if f.Line > 0 {
			sb.WriteString(fmt.Sprintf("Line: %d\n", f.Line))
		}

		if f.Route != nil {
			sb.WriteString(fmt.Sprintf("Route: %s %s\n", f.Route.Method, f.Route.Path))
			if f.Route.Handler != "" {
				sb.WriteString(fmt.Sprintf("Handler: %s\n", f.Route.Handler))
			}
		}

		if f.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("Suggestion: %s\n", f.Suggestion))
		}

		sb.WriteString("\n")
	}

	return sb.String()
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
