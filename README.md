# go-prism

[![CI](https://github.com/Athena900/go-prism/actions/workflows/ci.yml/badge.svg)](https://github.com/Athena900/go-prism/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Athena900/go-prism)](https://github.com/Athena900/go-prism/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/Athena900/go-prism.svg)](https://pkg.go.dev/github.com/Athena900/go-prism)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

PR evidence reports for Go modules.

`go-prism` is a maintainer-focused evidence layer for Go pull requests. It turns deterministic Go module checks into a single report that helps reviewers understand release, API, vulnerability, dependency, and downstream compatibility risk before merge.

`go-prism` does not replace `gorelease`, `govulncheck`, `modver`, `go-apidiff`, GoReleaser, or OpenSSF Scorecard. It composes their signals into a maintainer-centered report.

## Current Status

Latest published release: [`v0.2.1`](https://github.com/Athena900/go-prism/releases/tag/v0.2.1).

Use the latest release tag for stable installs and GitHub Actions. Use `@main`
only when you intentionally want the latest development state.

Implemented in the latest published release:

- CLI command: `go-prism pr`
- Environment diagnostics: `go-prism doctor`
- Config generation: `go-prism init`
- Structured evidence model
- Deterministic maintainer summary in Markdown and JSON reports
- Deterministic release notes draft in Markdown and JSON reports when release-note-worthy evidence exists
- Markdown and JSON report renderers
- Stable PR JSON schema marker: `report.v1`
- `.go-prism.yml` config loading
- Current `go.mod` policy check
- Base/head `go.mod` diff evidence for module, Go version, toolchain, requirements, replace, and retract changes
- API/SemVer evidence with `gorelease` execution plus supplemental `modver` and `go-apidiff` classification
- Current checkout vulnerability evidence from `govulncheck` JSON output
- Base/head vulnerability delta evidence from normalized `govulncheck` findings
- Local and remote downstream canary checks with temporary `replace`
- GitHub Actions step summary usage
- Sticky GitHub PR comments for same-repository pull requests
- Composite GitHub Action wrapper
- CLI version output: `0.2.1`
- `go-prism doctor` Git history diagnostics for shallow checkouts
- `go-prism doctor --base` and `--head` ref diagnostics
- GitHub Action preflight `go-prism doctor` with `preflight-doctor` opt-out
- Latest published GitHub Release: [`v0.2.1`](https://github.com/Athena900/go-prism/releases/tag/v0.2.1)
- Published install smoke test using `go install github.com/Athena900/go-prism/cmd/go-prism@v0.2.1`
- External GitHub Action smoke test using `Athena900/go-prism@v0.2.1`
- Public sample consumer downstream canary using [`Athena900/go-prism-sample-consumer`](https://github.com/Athena900/go-prism-sample-consumer)

Planned next:

- Additional troubleshooting docs for common CI setup failures

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

## What Makes It Different

- **PR-centered evidence**: `go-prism` focuses on what a maintainer needs before merge, not only what a release tool needs after merge.
- **Deterministic first**: pass/fail status comes from structured evidence, not model-generated text.
- **Maintainer summary**: reports include a rule-based summary with evidence IDs, so reviewers can scan the main result before reading every item.
- **Release notes draft**: reports can draft conservative release-note bullets from evidence without API keys or external services.
- **One review surface**: `go.mod` changes, API/SemVer signals, vulnerability findings, and downstream canary results are rendered into the same severity buckets.
- **Release-aware output**: reports can surface suggested release impact and migration-note material alongside blockers and warnings.
- **CLI and Action paths**: maintainers can run it locally, in CI step summaries, or as a sticky PR comment.

## Quick Start

Install the latest published release:

```bash
go install github.com/Athena900/go-prism/cmd/go-prism@v0.2.1
```

Install the latest development build from `main`:

```bash
go install github.com/Athena900/go-prism/cmd/go-prism@main
```

Generate a starter config:

```bash
go-prism init
```

Preview the config without writing a file:

```bash
go-prism init --dry-run
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

PR JSON reports include a stable top-level schema marker:

```json
{
  "schema_version": "report.v1",
  "tool": "go-prism",
  "version": "0.2.1",
  "decision": "pass",
  "maintainer_summary": {
    "headline": "No blockers or warnings were found in deterministic evidence.",
    "key_findings": [
      {
        "text": "No go.mod diff: No meaningful go.mod changes were detected between base and head.",
        "evidence_ids": ["gomod.diff.no_changes"]
      }
    ],
    "evidence_ids": ["gomod.diff.no_changes"]
  },
  "items": [
    {
      "id": "gomod.diff.no_changes",
      "title": "No go.mod diff",
      "status": "pass",
      "severity": "none",
      "category": "gomod",
      "source": "go.mod diff",
      "summary": "No meaningful go.mod changes were detected between base and head."
    }
  ]
}
```

When release-note-worthy evidence exists, JSON reports also include a
deterministic draft:

```json
{
  "release_notes_draft": {
    "suggested_impact": "minor",
    "notes": [
      {
        "text": "Public API changes were detected; consider documenting the added or changed API surface.",
        "evidence_ids": ["api.modver.minor_required"]
      }
    ],
    "evidence_ids": ["api.modver.minor_required"]
  }
}
```

Use an explicit config:

```bash
go-prism pr --config .go-prism.yml --format markdown
```

Enable optional checks during config generation:

```bash
go-prism init --enable-api --enable-vuln
go-prism init --enable-downstream --downstream example-consumer=../example-consumer
```

For PR-style diff evidence, make sure the base ref is available locally. In GitHub Actions, use `actions/checkout` with `fetch-depth: 0`.

## Init

Use `init` to create a safe starter `.go-prism.yml` for a Go module:

```bash
go-prism init
```

`init` detects the module path from `go.mod`, enables the deterministic `gomod` check by default, and leaves optional API, vulnerability, and downstream checks disabled unless requested. It does not run analysis, install tools, edit `go.mod`, or create GitHub workflow files.

The command will not overwrite an existing config unless `--force` is passed:

```bash
go-prism init --force
```

Machine-readable setup output is available for automation:

```bash
go-prism init --format json
```

## Doctor

Use `doctor` before enabling optional checks or debugging CI setup:

```bash
go-prism doctor
```

Generate machine-readable setup diagnostics:

```bash
go-prism doctor --format json
```

Validate the same refs you plan to use for PR evidence:

```bash
go-prism doctor --base origin/main --head HEAD
```

`doctor` is read-only. It checks the local Go and git runtime, the target workdir, `go.mod`, config loading, Git history depth, optional base/head ref availability, optional tool availability for enabled checks, downstream canary paths, and basic GitHub Actions environment hints. Warnings exit successfully by default so teams can adopt it without making CI brittle.

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
      - name: public-consumer
        repo: https://github.com/example/consumer.git
        ref: main
        subdir: .
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

`go-prism` also uses `modver` as supplemental API/SemVer evidence when it is available on `PATH`:

```bash
go install github.com/bobg/modver/v2/cmd/modver@latest
```

`modver` compares the configured base and head Git refs and reports the minimum required version bump: none, patchlevel, minor, or major. Missing `modver` is reported as informational evidence so existing `gorelease` users can adopt go-prism without installing every supplemental adapter immediately. Current `modver@latest` may require a newer Go toolchain than go-prism itself; on Go installations with `GOTOOLCHAIN=auto`, installing it may download a newer toolchain. In GitHub Actions, keep `actions/checkout` at `fetch-depth: 0` so both `gorelease` and `modver` have enough history for base/head comparison.

`go-prism` also uses `go-apidiff` as supplemental API compatibility evidence when it is available on `PATH`:

```bash
go install github.com/joelanford/go-apidiff@latest
```

`go-apidiff` compares the configured base and head Git refs and reports compatible and incompatible public API changes. Missing `go-apidiff` is informational evidence, matching the supplemental `modver` behavior. go-prism runs it with `--print-compatible` so compatible API additions can be surfaced as minor-release warnings. It does not enable `--compare-imports` by default, and it compares committed refs rather than uncommitted local worktree changes.

When `checks.vuln.enabled` is true, `go-prism` expects `govulncheck` on `PATH`:

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
```

The vulnerability checker runs `govulncheck -format=json ./...` against the configured head workdir and normalizes findings into evidence. Reachable symbol-level findings are blockers, package/module findings are warnings, and scanner failures are reported as unknown instead of pass.

When both `--base` and `--head` are present, go-prism also compares normalized base/head findings. `HEAD` scans the configured workdir, and non-`HEAD` refs are scanned through temporary detached git worktrees. New symbol-level findings are blockers, new package/module findings are warnings, fixed findings are informational, and unchanged findings produce a passing delta item.

When `checks.downstream.enabled` is true, `go-prism` runs explicitly configured downstream canaries. A canary can point at a local path:

```yaml
checks:
  downstream:
    enabled: true
    modules:
      - name: local-consumer
        path: ../example-consumer
        command: go test ./...
```

or at a trusted public HTTPS Git repository:

```yaml
checks:
  downstream:
    enabled: true
    modules:
      - name: public-consumer
        repo: https://github.com/example/consumer.git
        ref: main
        subdir: .
        command: go test ./...
```

Remote downstream canaries are cloned into temporary directories and removed after the run. Private repositories, embedded credentials, SSH URLs, and token-based auth are intentionally not supported in the current MVP.

For each downstream module, go-prism temporarily adds:

```bash
go mod edit -replace=<target-module>=<head-workdir>
```

Then it runs the configured command, defaulting to `go test ./...`. Local downstream canaries restore `go.mod` and `go.sum` afterwards. Remote downstream canaries discard the temporary clone. Successful canaries pass, failed commands block, and setup, clone, checkout, replace, restore, or cleanup failures are reported as unknown.

Only configure remote canaries for repositories you trust. The downstream command can execute code from the downstream repository.

## Downstream Canary Recipes

Use downstream canaries when a PR can compile and test locally but may still
break a known consumer. Start with one fast canary, then add more only when they
provide distinct review signal.

### Local Sibling Consumer

Use this when you keep a downstream app or library next to the module under
review:

```yaml
checks:
  downstream:
    enabled: true
    modules:
      - name: local-cli-consumer
        path: ../cli-consumer
        command: go test ./...
```

This is the fastest setup for day-to-day local development. go-prism temporarily
replaces the reviewed module in `../cli-consumer`, runs the command, and then
restores the consumer's `go.mod` and `go.sum`.

### Trusted Public Remote Consumer

Use this when a known public project imports your module and should keep working
before a release:

```yaml
checks:
  downstream:
    enabled: true
    modules:
      - name: public-consumer
        repo: https://github.com/example/public-consumer.git
        ref: main
        subdir: .
        command: go test ./...
```

Remote canaries are cloned into a temporary directory and removed after the run.
Use trusted repositories only because the configured command executes code from
the downstream repository.

go-prism also has a tiny public sample consumer that can be used as a concrete
remote canary example:

```yaml
checks:
  downstream:
    enabled: true
    modules:
      - name: go-prism-sample-consumer
        repo: https://github.com/Athena900/go-prism-sample-consumer.git
        ref: main
        subdir: .
        command: scripts/check-go-prism.sh
```

That sample repository installs the `go-prism` CLI from its module graph, runs
`go-prism version`, and verifies `go-prism doctor` reports `Overall: OK`.

### Remote Monorepo Subdirectory

Use `subdir` when the downstream Go module is not at the repository root:

```yaml
checks:
  downstream:
    enabled: true
    modules:
      - name: public-platform-service
        repo: https://github.com/example/platform.git
        ref: main
        subdir: services/go-consumer
        command: go test ./...
```

The `subdir` must be relative and must not escape the clone. This is useful for
platform repositories where one service depends on the module under review.

### Stricter Compatibility Command

Use a stricter command when a consumer has meaningful static checks:

```yaml
checks:
  downstream:
    enabled: true
    modules:
      - name: public-consumer-strict
        repo: https://github.com/example/public-consumer.git
        ref: main
        subdir: .
        command: go test ./... && go vet ./...
```

Commands run through `sh -c`, so keep them short, deterministic, and bounded.
Avoid commands that publish, deploy, mutate external services, or require
secrets.

### GitHub Actions Setup

In CI, keep checkout history available and pin go-prism to a release tag:

```yaml
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - uses: Athena900/go-prism@v0.2.1
        with:
          base: ${{ github.event.pull_request.base.sha }}
          head: HEAD
          config: .go-prism.yml
```

For important compatibility canaries, prefer a known branch or tag in `ref`.
Private repository auth, embedded credentials, SSH URLs, and token-based remote
canaries are not supported in the current MVP.

`go-prism doctor` warns when the current checkout is shallow, because base/head
evidence may be incomplete without full history.

To check the exact refs before running the full report:

```bash
go-prism doctor --base "${{ github.event.pull_request.base.sha }}" --head HEAD
```

## Sample Report

The default Markdown report is designed to be pasted into a PR summary or GitHub Actions step summary. A no-change run on this repository currently looks like:

```markdown
## Go Prism

Decision: PASS
Suggested release impact: unknown
Module: `github.com/Athena900/go-prism`
Refs: `HEAD` -> `HEAD`

### Maintainer Summary

No blockers or warnings were found in deterministic evidence.

Key findings:
- No go.mod diff: No meaningful go.mod changes were detected between base and head.
  Evidence: `gomod.diff.no_changes`
- No replace directives: go.mod does not contain replace directives.
  Evidence: `gomod.replace_none`
- Go directive detected: Go directive: `1.22.0`.
  Evidence: `gomod.go_directive`
- Module path detected: Module path: `github.com/Athena900/go-prism`.
  Evidence: `gomod.module_path`

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

Generated from deterministic evidence. Maintainer summary and release notes draft are rule-based and advisory.
```

When optional checks are enabled, `gorelease`, `modver`, `go-apidiff`, `govulncheck`, and downstream canary results are added to the same severity buckets, so maintainers can scan one report instead of stitching together multiple tool outputs.

When release-note-worthy evidence exists, Markdown reports also include a draft
section before the detailed evidence buckets:

```markdown
### Release Notes Draft

Suggested impact: minor

- Public API changes were detected; consider documenting the added or changed API surface.
  Evidence: `api.modver.minor_required`
```

## Machine-Readable Output

`go-prism` uses explicit schema markers for JSON output:

| Command | Schema |
| --- | --- |
| `go-prism pr --format json` | `report.v1` |
| `go-prism doctor --format json` | `doctor.v1` |
| `go-prism init --format json` | `init.v1` |

For `report.v1`, existing top-level fields and evidence item fields are intended to remain stable. New optional fields, such as `maintainer_summary` and `release_notes_draft`, and new evidence item IDs may be added over time. Removing fields, renaming fields, changing field types, or changing status meanings requires a new schema version.

## GitHub Action

The current recommended GitHub Actions usage runs the composite action and writes the Markdown report to the workflow step summary. Sticky PR comments can be enabled for same-repository pull requests.

For stable usage, pin a version tag or commit SHA. The latest published action tag is `Athena900/go-prism@v0.2.1`. Use `@main` only when you intentionally want the latest development state.

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

      - uses: Athena900/go-prism@v0.2.1
        with:
          base: ${{ github.event.pull_request.base.sha }}
          head: HEAD
          config: .go-prism.yml
          sticky-comment: "true"
          github-token: ${{ github.token }}
```

The action sets up Go by default using `go.mod`, runs:

```bash
go-prism doctor --base <base> --head <head> --format text
go-prism pr --base <base> --head <head> --format markdown
```

and appends the generated report to `$GITHUB_STEP_SUMMARY`. The preflight
`doctor` output is written to the workflow log so setup issues such as shallow
history or missing refs are visible before the report is generated. The sticky
comment path uses the marker `<!-- go-prism:report -->` to update one existing
comment instead of creating a new comment on every push.

Disable preflight only when you intentionally want the report command to be the
first go-prism command in the job:

```yaml
      - uses: Athena900/go-prism@v0.2.1
        with:
          preflight-doctor: "false"
```

For fork pull requests, keep sticky comments disabled unless you intentionally design a separate privileged workflow:

```yaml
      - uses: Athena900/go-prism@v0.2.1
        if: github.event.pull_request.head.repo.full_name == github.repository
        with:
          base: ${{ github.event.pull_request.base.sha }}
          sticky-comment: "true"
          github-token: ${{ github.token }}
```

If Go is already set up earlier in the job, disable the action's setup step:

```yaml
      - uses: Athena900/go-prism@v0.2.1
        with:
          base: ${{ github.event.pull_request.base.sha }}
          setup-go: "false"
```

## Deterministic Output Guardrails

`go-prism` is deterministic first.

- Deterministic evidence is the audit source.
- The maintainer summary is generated locally from evidence items.
- The release notes draft is generated locally from evidence items.
- The maintainer summary does not require an API key.
- The release notes draft does not require an API key.
- The maintainer summary cannot change pass/fail status.
- The release notes draft cannot change pass/fail status.
- The maintainer summary cannot lower severity.
- The release notes draft cannot lower severity.
- The maintainer summary cannot mark a blocker as passed.
- The release notes draft cannot mark a blocker as passed.
- Summary findings and release-note bullets cite evidence item IDs.

## Limitations

- The API checker currently supports `gorelease`, supplemental `modver`, and supplemental `go-apidiff`.
- `modver` and `go-apidiff` comparisons use committed Git refs and do not include uncommitted local worktree changes.
- The vulnerability delta checker requires locally available git refs. In GitHub Actions, use `actions/checkout` with `fetch-depth: 0`.
- Remote downstream canaries currently support trusted public HTTPS Git repositories only. Private repository auth, embedded credentials, SSH URLs, and dependency caching are not implemented yet.
- Sticky PR comments are currently implemented for same-repository pull requests. Fork pull requests still use GitHub Actions step summaries by default.
- The GitHub Action is a composite action. Stable external usage should pin a version tag or commit SHA; `@main` tracks development.
- The current MVP checks the current `go.mod` state, compares base/head `go.mod` snapshots, runs selected external evidence tools when enabled, and renders evidence reports with a deterministic maintainer summary and release notes draft.
- The project does not make autonomous merge, release, deploy, or remediation decisions.

## Development

```bash
go test ./...
go vet ./...
go test -race ./...
```

Before tagging future release candidates, also run:

```bash
git diff --check
go run ./cmd/go-prism version
go run ./cmd/go-prism pr --base HEAD --head HEAD --format markdown
go run ./cmd/go-prism pr --base HEAD --head HEAD --format json
go run ./cmd/go-prism doctor --format text
```

## License

MIT
