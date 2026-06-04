# Contributing

Thanks for helping improve `go-prism`.

## Development Principles

- Deterministic evidence comes first.
- AI summaries are advisory only.
- Existing Go tools should be composed, not reimplemented without a reason.
- Keep checks, policy, evidence, rendering, and CLI wiring separate.
- Do not add autonomous merge, release, deploy, or remediation behavior.

## Local Setup

```bash
go test ./...
go vet ./...
go test -race ./...
```

## Pull Requests

Before opening a PR:

- add or update focused tests
- keep changes scoped
- update README or examples for user-visible behavior changes
- avoid committing local artifacts, secrets, or private repository data

## Security

Do not file public issues for exploitable vulnerabilities. Follow `SECURITY.md`.
