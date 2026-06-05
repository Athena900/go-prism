# go-prism

PR evidence reports for Go modules.

`go-prism` turns deterministic Go module checks into a single PR-ready evidence report. It is designed for Go OSS maintainers who need to understand release, API, vulnerability, dependency, and downstream compatibility risk before merge.

`go-prism` does not replace `gorelease`, `govulncheck`, `modver`, `go-apidiff`, GoReleaser, or OpenSSF Scorecard. It composes their signals into a maintainer-centered report.

## Current Status

This repository is preparing its first public MVP release, `v0.1.0`. The release contents are implemented and verified locally; create the tag only after the intended release commit is pushed and CI is green.

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
- Sticky GitHub PR comments for same-repository pull requests
- Composite GitHub Action wrapper
- CLI version output: `0.1.0`

Planned next:

- Additional API/SemVer adapters for `modver` and `go-apidiff`
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

Install the release after the `v0.1.0` tag is published:

```bash
go install github.com/Athena900/go-prism/cmd/go-prism@v0.1.0
```

Install the latest development build from `main`:

```bash
go install github.com/Athena900/go-prism/cmd/go-prism@main
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

The default Markdown report is designed to be pasted into a PR summary or GitHub Actions step summary. A no-change run on this repository currently looks like:

```markdown
## Go Prism

Decision: PASS
Suggested release impact: unknown
Module: `github.com/Athena900/go-prism`
Refs: `HEAD` -> `HEAD`

### Blocking

None.

### Needs Maintainer Review

None.

### Unknown

None.

### Informational

- **Go directive detected** `gomod.go_directive` Go directive: `1.22.0`.
  Recommendation: Review Go version increases carefully in release PRs because they can affect downstream users.
- **Module path detected** `gomod.module_path` Module path: `github.com/Athena900/go-prism`.
  Recommendation: Confirm this matches the repository and intended public import path.
- **PR context captured** `pr.context` go-prism captured base/head refs and configuration for this evidence run.
  Recommendation: Use this context when comparing report output across CI runs.

### Passing

- **No go.mod diff** `gomod.diff.no_changes` No meaningful go.mod changes were detected between base and head.
  Recommendation: No go.mod diff review needed.
- **No replace directives** `gomod.replace_none` go.mod does not contain replace directives.
  Recommendation: No replace directive review needed.

Generated from deterministic evidence. AI text, if enabled in a future version, is advisory only.
```

When optional checks are enabled, `gorelease`, `govulncheck`, and downstream canary results are added to the same severity buckets, so maintainers can scan one report instead of stitching together multiple tool outputs.

## GitHub Action

The current recommended GitHub Actions usage runs the composite action and writes the Markdown report to the workflow step summary. Sticky PR comments can be enabled for same-repository pull requests.

For stable usage, pin a version tag or commit SHA. Use `@main` only when you intentionally want the latest development state. Until the `v0.1.0` tag is pushed, replace `@v0.1.0` in the examples below with `@main`.

```yaml
name: Go Prism

on:
  pull_request:
    branches: [main]

permissions:
  contents: read
  issues: write
  pull-requests: write

jobs:
  go-prism:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - uses: Athena900/go-prism@v0.1.0
        with:
          base: ${{ github.event.pull_request.base.sha }}
          head: HEAD
          config: .go-prism.yml
          sticky-comment: "true"
          github-token: ${{ github.token }}
```

The action sets up Go by default using `go.mod`, runs:

```bash
go-prism pr --base <base> --head <head> --format markdown
```

and appends the generated report to `$GITHUB_STEP_SUMMARY`. The sticky comment path uses the marker `<!-- go-prism:report -->` to update one existing comment instead of creating a new comment on every push.

For fork pull requests, keep sticky comments disabled unless you intentionally design a separate privileged workflow:

```yaml
      - uses: Athena900/go-prism@v0.1.0
        if: github.event.pull_request.head.repo.full_name == github.repository
        with:
          base: ${{ github.event.pull_request.base.sha }}
          sticky-comment: "true"
          github-token: ${{ github.token }}
```

If Go is already set up earlier in the job, disable the action's setup step:

```yaml
      - uses: Athena900/go-prism@v0.1.0
        with:
          base: ${{ github.event.pull_request.base.sha }}
          setup-go: "false"
```

## AI Guardrails

`go-prism` is deterministic first.

- Deterministic evidence is the audit source.
- AI output cannot change pass/fail status.
- AI output cannot lower severity.
- AI output cannot mark a blocker as passed.
- AI summaries must cite evidence item IDs.

## Limitations

- Additional API/SemVer adapters are not implemented yet.
- The API checker currently supports `gorelease`; `modver` and `go-apidiff` adapters are not implemented yet.
- The vulnerability delta checker requires locally available git refs. In GitHub Actions, use `actions/checkout` with `fetch-depth: 0`.
- Downstream canaries currently support explicit local paths only. Remote clone support is not implemented yet.
- Sticky PR comments are currently implemented for same-repository pull requests. Fork pull requests still use GitHub Actions step summaries by default.
- The GitHub Action is a composite action. Stable external usage should pin a version tag or commit SHA; `@main` tracks development.
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
