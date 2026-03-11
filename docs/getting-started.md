# Getting Started with APIAudit

This guide walks you through installing APIAudit and running your first audit.

## Prerequisites

- **Go 1.21+** (for installing from source)
- Or **Homebrew** (macOS/Linux)

## Installation

### Option A: Homebrew

```bash
brew install saltrenis/tap/apiaudit
```

### Option B: Go Install

```bash
# CLI only
go install github.com/Saltrenis/APIAudit/cmd/apiaudit@latest

# CLI + Web UI
go install github.com/Saltrenis/APIAudit/cmd/apiaudit-web@latest
```

### Option C: Build from Source

```bash
git clone https://github.com/Saltrenis/APIAudit.git
cd APIAudit
make build       # builds the CLI
make web         # builds the CLI + web UI
```

The binaries are placed in `bin/`.

## Your First Audit (CLI)

### 1. Navigate to your project

```bash
cd ~/my-project
```

### 2. Detect the framework

```bash
apiaudit detect
```

Output:
```
Language:    Go
Framework:   echo
Version:     v4.15.0
Confidence:  33%
Has Frontend: true (frontend/)
Has Swagger:  false
```

### 3. Run a full audit

```bash
apiaudit audit
```

This runs the full pipeline:
1. **Detect** -- identifies your framework
2. **Scan** -- extracts all routes from source code
3. **Generate** -- writes an `openapi.yaml` to your project
4. **Analyze** -- checks coverage, consistency, and frontend contracts
5. **Report** -- prints findings

### 4. Save the report

```bash
# Markdown report
apiaudit audit --format markdown --output audit-report.md

# JSON report (for tooling)
apiaudit audit --format json --output audit-report.json
```

## Your First Audit (Web UI)

### 1. Start the web server

```bash
apiaudit-web
```

Opens at [http://127.0.0.1:8090](http://127.0.0.1:8090).

### 2. Follow the wizard

**Step 1 -- Project Source**: Enter your project directory path (e.g., `~/my-project`) or a GitHub repo URL. If your frontend lives in a separate directory, enter it in the optional "Frontend Directory" field.

**Step 2 -- Command**: Select "Full Audit" (recommended) or pick a specific command.

**Step 3 -- Options**: Toggle any options you need. Common choices:
- **Static Only** -- skip frontend analysis if you don't have a frontend
- **Beads** -- create issue tracker entries for findings

**Step 4 -- Review**: See the exact CLI command that will run. Click "Run Audit".

**Step 5 -- Results**: Watch real-time progress, then explore:
- **Summary** -- coverage percentage and finding counts by severity
- **Routes tab** -- sortable table of all discovered routes
- **Findings tab** -- issues grouped by severity with fix suggestions

## Common Workflows

### Audit a remote repository

```bash
apiaudit audit --repo https://github.com/org/my-api
```

APIAudit clones the repo to a temp directory, runs the audit, and cleans up.

### Audit with separate backend/frontend directories

Some projects have the backend and frontend in different directories:

```bash
apiaudit audit --dir ~/project/backend --frontend-dir ~/project/frontend
```

### Generate an OpenAPI spec only

```bash
apiaudit generate --dir . --title "My API" --api-version "2.0.0"
```

Writes `openapi.yaml` (or `openapi.json` with `--json`) to the project root.

### Find unannotated routes

```bash
# List routes missing swagger docs
apiaudit annotate --dry-run

# Generate annotations with Claude Code
apiaudit annotate --ai-assist
```

### Create issue tracker entries

```bash
# Create beads issues for all findings
apiaudit audit --beads

# Limit to 20 issues
apiaudit audit --beads --beads-limit 20
```

### Coverage-only analysis (no frontend)

```bash
apiaudit audit --static-only
```

## Understanding the Output

### Severity Levels

| Level | Meaning | Examples |
|-------|---------|---------|
| **P1** | Breaking -- frontend will fail | Missing endpoint, wrong response shape, field name mismatch |
| **P2** | Important -- data issues | Undocumented POST/PUT/DELETE, type mismatches |
| **P3** | Minor -- documentation gaps | Undocumented GET routes |
| **P4** | Informational | Dead code, hardcoded values |

### Finding Categories

| Category | Description |
|----------|-------------|
| `missing-swagger` | Route has no swagger/OpenAPI annotation |
| `ENDPOINT_MISSING` | Frontend calls a route that doesn't exist |
| `SHAPE_MISMATCH` | Response structure differs from frontend expectation |
| `FIELD_MISSING` | Frontend accesses a field not in the response |
| `FIELD_CASING` | snake_case vs camelCase mismatch |
| `MOCK_DATA` | Frontend uses hardcoded data instead of API call |
| `DEAD_CODE` | API function defined but never called |

## Next Steps

- Run `apiaudit init` to create a `.apiaudit.json` config so you don't need to repeat flags
- Add `apiaudit audit --format json` to your CI pipeline to catch issues before merge
- Use `apiaudit annotate --ai-assist` to auto-generate swagger docs for undocumented routes
