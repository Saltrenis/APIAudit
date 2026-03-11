# APIAudit

Zero-config API route scanner and auditor.

APIAudit is a CLI tool that automatically detects your web framework, extracts every registered route from source code, generates an OpenAPI 3.0 specification, and reports inconsistencies between your backend API and frontend consumers. It supports Go, Node/TypeScript, and Python frameworks. Everything runs as pure Go static analysis -- no AI tokens are used except by the optional `annotate` command.

APIAudit also ships with a **web-based wizard** that provides a guided UI for running audits, with real-time streaming output and structured results.

## Quick Start

### Install

```bash
# Install with Go
go install github.com/Saltrenis/APIAudit/cmd/apiaudit@latest

# Or with Homebrew
brew install saltrenis/tap/apiaudit
```

### CLI

```bash
# Audit the current directory
apiaudit audit

# Audit a specific project
apiaudit audit --dir ~/my-project --format markdown --output audit.md

# Audit a remote repo
apiaudit audit --repo https://github.com/org/repo
```

### Web UI

```bash
# Install the web server
go install github.com/Saltrenis/APIAudit/cmd/apiaudit-web@latest

# Start the wizard (opens at http://127.0.0.1:8090)
apiaudit-web
```

The web wizard walks you through project setup, command selection, and options -- then streams results in real-time with sortable route tables and findings grouped by severity.

## Supported Frameworks

| Language | Frameworks | Detection File |
|----------|-----------|----------------|
| Go | Gin, Echo, Chi, Fiber, net/http (stdlib) | go.mod |
| Node / TypeScript | Express, NestJS, Fastify, Koa | package.json |
| Python | FastAPI, Flask, Django REST Framework | requirements.txt, pyproject.toml |

When multiple frameworks are present, APIAudit uses a priority-weighted scoring system to select the best match and reports a confidence value.

## Commands

- **detect** -- Identify the project's language, web framework, version, and whether a frontend or existing swagger spec is present.
- **scan** -- Walk the source tree and extract every registered route with HTTP method, path, handler name, source file, and line number.
- **generate** -- Produce an OpenAPI 3.0 specification in YAML or JSON from the scanned routes.
- **audit** -- Run the full pipeline: detect, scan, generate, analyze, and report. This is the primary command.
- **annotate** -- Generate swagger annotations for undocumented routes. Uses Claude when `--ai-assist` is set; otherwise prints templates to stdout.
- **init** -- Create an `.apiaudit.json` config file in the target project. Optionally initializes beads issue tracking.

## Usage Examples

Detect the framework in the current directory:

```bash
$ apiaudit detect
Language:    Go
Framework:   chi
Version:     v5.0.11
Confidence:  1.00
Has Frontend: true (frontend/)
Has Swagger:  true
```

Scan and list all routes as a table:

```bash
$ apiaudit scan --dir ./my-api --format table
METHOD  PATH                    HANDLER              FILE                          LINE
GET     /v1/users               userapi.GetAll       api/domain/http/userapi/...   42
POST    /v1/users               userapi.Create       api/domain/http/userapi/...   87
GET     /v1/users/:id           userapi.GetByID      api/domain/http/userapi/...   63
DELETE  /v1/users/:id           userapi.Delete       api/domain/http/userapi/...   105
```

Generate an OpenAPI spec:

```bash
$ apiaudit generate --dir . --title "My API" --api-version "2.0.0" --output openapi.yaml
```

Run a full audit and write findings as markdown:

```bash
$ apiaudit audit --dir ./my-project --format markdown --output audit.md
```

Audit a remote repository and create beads issues for each finding:

```bash
$ apiaudit audit --repo https://github.com/org/repo --beads --format json
```

## Flags Reference

### Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dir` | string | `.` | Target project directory |
| `--repo` | string | | Git repository URL to clone before running |
| `--output` | string | | Write report to a file instead of stdout |
| `--format` | string | `table` | Output format: `table`, `json`, or `markdown` |
| `--beads` | bool | `false` | Create beads issues for each finding |
| `--ai-assist` | bool | `false` | Use Claude Code (local CLI) for annotation generation |
| `--beads-limit` | int | `50` | Maximum number of beads issues to create per run (0 = unlimited) |

### `audit` Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--static-only` | bool | `false` | Run coverage and consistency analysis only (no frontend) |
| `--skip-generate` | bool | `false` | Skip OpenAPI spec generation step |
| `--skip-frontend` | bool | `false` | Skip frontend contract analysis |
| `--frontend-dir` | string | | Override the auto-detected frontend directory |

### `generate` Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--title` | string | directory name | API title in the generated spec |
| `--api-version` | string | `1.0.0` | API version string |
| `--description` | string | | API description |
| `--json` | bool | `false` | Write JSON instead of YAML |

## Output Formats

APIAudit supports three output formats controlled by the `--format` flag:

- **table** (default) -- Compact columnar output for terminal use.
- **json** -- Structured JSON, suitable for piping into `jq` or other tools.
- **markdown** -- GitHub-flavored markdown, useful for audit reports or pasting into issues.

All formats can be written to a file with `--output` or printed to stdout.

## Beads Integration

The `--beads` flag integrates with the [beads](https://github.com/Saltrenis/beads) issue tracker. When enabled, APIAudit creates one beads issue per finding, grouped by category and file, with automatic deduplication. Use `--beads-limit` to cap the number of issues created per run.

The `init` command will set up beads tracking in the target project if the `bd` CLI is installed.

## Claude Integration

The `annotate` command is the only part of APIAudit that uses AI. When `--ai-assist` is passed, it invokes your locally installed [Claude Code](https://claude.com/claude-code) CLI to generate swagger annotations for unannotated routes. No API key is needed -- it uses your existing Claude Code session.

Everything else -- framework detection, route scanning, OpenAPI generation, analysis, and reporting -- is pure Go with zero external calls. The tool works fully offline for all commands except `annotate --ai-assist`.

```bash
# With Claude Code installed locally
apiaudit annotate --dir . --ai-assist

# Without Claude Code -- outputs route data for manual annotation
apiaudit annotate --dir .
```

## Web UI

APIAudit includes a browser-based wizard for users who prefer a graphical interface over the CLI.

### Features

- **Guided wizard** -- 5-step flow: pick your project source, choose a command, configure options, review, and run
- **Real-time streaming** -- watch progress logs as the audit runs, with structured results when complete
- **Sortable route table** -- filter by HTTP method, search by path, colored method badges
- **Findings by severity** -- P1-P4 grouping with expandable suggestions and file locations
- **Single binary** -- the frontend is embedded in the Go binary, no Node.js required at runtime

### Running the Web UI

```bash
# From source
cd ~/APIAudit
make web        # builds frontend + Go server
bin/apiaudit-web

# With go install
go install github.com/Saltrenis/APIAudit/cmd/apiaudit-web@latest
apiaudit-web
```

Opens at `http://127.0.0.1:8090`. The server binds to localhost only.

### Web Server Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8090` | Port to listen on |
| `--bin` | auto-detect | Path to the `apiaudit` binary |
| `--dev` | `false` | Serve frontend from disk (for development) |

### Development

To work on the frontend with hot-reload:

```bash
# Terminal 1: Start the Go API server
cd ~/APIAudit && go run ./cmd/apiaudit-web --dev

# Terminal 2: Start Vite dev server
cd ~/APIAudit/frontend && npm run dev
```

Vite proxies `/api/*` requests to the Go server on port 8090.

## Contributing

1. Fork the repository and create a feature branch.
2. Make your changes. Run `go fmt ./...` and `go vet ./...` before committing.
3. Commit with a prefix: `[feat]`, `[bug]`, or `[chore]`.
4. Open a pull request against `main`.

## License

See [LICENSE](LICENSE) for details.
