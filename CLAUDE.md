# APIAudit

CLI tool for auditing REST APIs — detects frameworks, extracts routes, generates OpenAPI specs, and finds issues.

## Tech Stack
- **Language**: Go 1.21+
- **CLI**: Cobra
- **Install**: `go install` or Homebrew

## Project Structure
- `cmd/apiaudit/` — CLI commands (Cobra)
- `internal/detect/` — Framework auto-detection
- `internal/scan/` — Route extraction per framework
- `internal/openapi/` — OpenAPI spec generation
- `internal/analyze/` — Static analysis (consistency, coverage, frontend contracts)
- `internal/report/` — Report generation (markdown, json, table)
- `internal/beads/` — Beads issue tracking integration
- `internal/repo/` — Git repo operations

## Development
- Run `go fmt ./...` before commits
- Run `go vet ./...` to check for issues
- Git commits: `[bug]`, `[chore]`, `[feat]` prefix with bulleted changes
- Do not push to main directly

## Claude Usage
Claude is only used for the `annotate` command — inserting swagger annotations into source code. All other functionality is pure Go. The `/api-audit-annotate` slash command handles this.

## Beads
This project uses beads (`bd` CLI) for issue tracking. See the system prompt for beads workflow.
