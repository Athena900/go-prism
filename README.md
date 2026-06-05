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
- API/SemVer checker scaffold with `gorelease` execution and release-impact evidence
- Current checkout vulnerability evidence from `govulncheck` JSON output
- Base/head vulnerability delta evidence from normalized `govulncheck` findings
- Local downstream canary checks with temporary `replace`
- GitHub Actions step summary usage

Planned next:

- Additional API/SemVer adapters for `modver` and `go-apidiff`
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
    modules:
      - name: example-consumer
        path: ../example-consumer
        command: go test ./...

policy:
  fail_on:
    gomod_parse_error: true
    new_replace_directive: false
```

When `checks.api.enabled` is true, `go-prism` expects `gorelease` on `PATH`:

```bash
go install golang.org/x/exp/cmd/gorelease@latest
```

`gorelease` compares the currently checked out module against released module versions. `go-prism --base` is primarily a PR Git ref for diff evidence; it is only passed to `gorelease -base` when it is a valid gorelease base value such as `v1.2.3`, `latest`, `none`, or `example.com/mod/v2@v2.1.0`.

When `checks.vuln.enabled` is true, `go-prism` expects `govulncheck` on `PATH`:

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
```

The vulnerability checker runs `govulncheck -format=json ./...` against the configured head workdir and normalizes findings into evidence. Reachable symbol-level findings are blockers, package/module findings are warnings, and scanner failures are reported as unknown instead of pass.

When both `--base` and `--head` are present, go-prism also compares normalized base/head findings. `HEAD` scans the configured workdir, and non-`HEAD` refs are scanned through temporary detached git worktrees. New symbol-level findings are blockers, new package/module findings are warnings, fixed findings are informational, and unchanged findings produce a passing delta item.

When `checks.downstream.enabled` is true, `go-prism` runs explicitly configured local downstream canaries. For each module, it temporarily adds:

```bash
go mod edit -replace=<target-module>=<head-workdir>
```

Then it runs the configured command, defaulting to `go test ./...`, and restores downstream `go.mod` and `go.sum` afterwards. Successful canaries pass, failed commands block, and setup or restore failures are reported as unknown.

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

## GitHub Actions

The current recommended GitHub Actions usage runs `go-prism pr` as a normal Go CLI and writes the Markdown report to the workflow step summary. This requires no PR write permission.

```yaml
name: Go Prism

on:
  pull_request:
    branches: [main]

permissions:
  contents: read

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

      - name: Go Prism Report
        run: |
          go run github.com/Athena900/go-prism/cmd/go-prism@latest pr \
            --base "${{ github.event.pull_request.base.sha }}" \
            --head HEAD \
            --config .go-prism.yml \
            --format markdown \
            --output go-prism-report.md
          cat go-prism-report.md >> "$GITHUB_STEP_SUMMARY"
```

A future wrapper action and sticky PR comment flow are planned. Until then, the CLI path keeps the workflow explicit and auditable.

## AI Guardrails

`go-prism` is deterministic first.

- Deterministic evidence is the audit source.
- AI output cannot change pass/fail status.
- AI output cannot lower severity.
- AI output cannot mark a blocker as passed.
- AI summaries must cite evidence item IDs.

## Limitations

- Additional API/SemVer adapters and sticky PR comment support are not implemented yet.
- The API checker currently supports `gorelease`; `modver` and `go-apidiff` adapters are not implemented yet.
- The vulnerability delta checker requires locally available git refs. In GitHub Actions, use `actions/checkout` with `fetch-depth: 0`.
- Downstream canaries currently support explicit local paths only. Remote clone support is not implemented yet.
- The current MVP checks the current `go.mod` state, compares base/head `go.mod` snapshots, runs selected external evidence tools when enabled, and renders evidence reports.
- The project does not make autonomous merge, release, deploy, or remediation decisions.

## Development

```bash
go test ./...
go vet ./...
go test -race ./...
```

## License

MIT
