# go-prism

PR evidence reports for Go modules.

`go-prism` turns deterministic Go module checks into a single PR-ready evidence report. It is designed for Go OSS maintainers who need to understand release, API, vulnerability, dependency, and downstream compatibility risk before merge.

`go-prism` does not replace `gorelease`, `govulncheck`, `modver`, `go-apidiff`, GoReleaser, or OpenSSF Scorecard. It composes their signals into a maintainer-centered report.

## Current Status

This repository is in early MVP development.

Implemented now:

- CLI skeleton: `go-prism pr`
- Structured evidence model
- Markdown and JSON report renderers
- `.go-prism.yml` config loading
- Current `go.mod` policy check
- Base/head `go.mod` diff evidence for module, Go version, toolchain, requirements, replace, and retract changes

Planned next:

- API/SemVer adapters for `gorelease`, `modver`, and `go-apidiff`
- `govulncheck` delta reports
- Downstream canary testing with temporary `replace`
- GitHub Action sticky PR comments
- Optional AI summaries based only on deterministic evidence

## Why This Exists

Go maintainers already have strong individual tools:

| Tool | What it does | How go-prism relates |
| --- | --- | --- |
| `gorelease` | Checks release compatibility before publishing a Go module | Baseline release evidence |
| `govulncheck` | Finds reachable known vulnerabilities | Vulnerability delta evidence |
| `modver` / `go-apidiff` | Detect API and SemVer impact | API impact evidence |
| GoReleaser | Builds and publishes artifacts | Runs after merge/release decision |
| OpenSSF Scorecard | Repository security posture | Optional future signal |

The missing layer is a compact PR report that separates blockers, warnings, informational changes, suggested release impact, downstream canary results, and release/migration note drafts.

## Quick Start

Install from source:

```bash
go install github.com/Athena900/go-prism/cmd/go-prism@latest
```

Run a local report:

```bash
go-prism pr \
  --base origin/main \
  --head HEAD \
  --format markdown \
  --output report.md
```

Generate JSON evidence:

```bash
go-prism pr --format json --output evidence.json
```

Use an explicit config:

```bash
go-prism pr --config .go-prism.yml --format markdown
```

For PR-style diff evidence, make sure the base ref is available locally. In GitHub Actions, use `actions/checkout` with `fetch-depth: 0`.

## Configuration

Example `.go-prism.yml`:

```yaml
module: github.com/example/project

checks:
  gomod:
    enabled: true
  api:
    enabled: false
  vuln:
    enabled: false
  downstream:
    enabled: false

policy:
  fail_on:
    gomod_parse_error: true
    new_replace_directive: false
```

## Sample Report

```markdown
## Go Prism

Decision: WARN
Suggested release impact: unknown

### Blocking

None.

### Needs Maintainer Review

- `go.mod` contains replace directives.
- Go directive changed from `1.21` to `1.22`.
- 1 direct requirement change(s) detected.

### Informational

- Module path: `github.com/example/project`
- Go directive: `1.22`

Generated from deterministic evidence. AI text, if enabled in a future version, is advisory only.
```

## GitHub Action

The GitHub Action wrapper is planned. Target usage:

```yaml
name: Go Prism

on:
  pull_request:
    branches: [main]

permissions:
  contents: read
  pull-requests: write

jobs:
  go-prism:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - uses: Athena900/go-prism-action@v0
        with:
          config: .go-prism.yml
          comment: true
```

## AI Guardrails

`go-prism` is deterministic first.

- Deterministic evidence is the audit source.
- AI output cannot change pass/fail status.
- AI output cannot lower severity.
- AI output cannot mark a blocker as passed.
- AI summaries must cite evidence item IDs.

## Limitations

- API/SemVer, vulnerability delta, downstream canary, and GitHub Action support are not implemented yet.
- The current MVP checks the current `go.mod` state, compares base/head `go.mod` snapshots, and renders evidence reports.
- The project does not make autonomous merge, release, deploy, or remediation decisions.

## Development

```bash
go test ./...
go vet ./...
go test -race ./...
```

## License

MIT
