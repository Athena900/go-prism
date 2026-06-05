# Changelog

All notable changes to this project will be documented in this file.

## Unreleased

### Added

- Deterministic maintainer summaries in Markdown and JSON PR reports.

### Changed

- Replaced the previous paid-provider summary direction with an API-key-free,
  rule-based maintainer summary.

## v0.1.0 - 2026-06-05

Initial public MVP release candidate.

### Added

- `go-prism pr` CLI command for PR-ready Go module evidence reports.
- Markdown and JSON report rendering from a deterministic structured evidence model.
- `.go-prism.yml` configuration with check toggles and downstream canary settings.
- Current `go.mod` policy evidence for module path, Go directive, replace directives, retract directives, and v2+ module suffix checks.
- Base/head `go.mod` diff evidence for module, Go version, toolchain, requirements, replace directives, and retract directives.
- API/SemVer evidence through `gorelease` when `checks.api.enabled` is true.
- Vulnerability evidence through `govulncheck` when `checks.vuln.enabled` is true.
- Base/head vulnerability delta evidence from normalized `govulncheck` findings.
- Local downstream canary checks with temporary `replace` directives and automatic `go.mod` / `go.sum` restore.
- GitHub Actions step summary integration.
- Same-repository sticky PR comments using the `<!-- go-prism:report -->` marker.
- Composite GitHub Action wrapper in `action.yml`.

### Verified

- Local CLI tests, vet, race tests, version output, Markdown report generation, shell syntax checks, workflow YAML parsing, action linting, and diff whitespace checks.
- GitHub push smoke run `26998476164` dogfooded the composite action through `uses: ./`.
- GitHub PR verification runs `26998633172` and `26998677758` dogfooded the composite action on `pull_request` and verified sticky comment creation/update behavior.

### Known Limitations

- API/SemVer support currently uses `gorelease`; `modver` and `go-apidiff` adapters are not implemented yet.
- Vulnerability delta checks require locally available git refs. GitHub Actions should use `actions/checkout` with `fetch-depth: 0`.
- Downstream canaries currently support explicit local paths only. Remote clone support is not implemented yet.
- Sticky PR comments are limited to same-repository pull requests by default. Fork pull requests should use step summaries unless a separate privileged workflow is intentionally designed.
- Deterministic maintainer summaries are not implemented yet.
- The project does not make autonomous merge, release, deploy, or remediation decisions.
