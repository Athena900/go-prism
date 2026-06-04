# Security Policy

## Supported Versions

`go-prism` is pre-1.0 software. Security fixes will be released on the latest `main` branch and future tagged releases once versioning starts.

## Reporting A Vulnerability

Please report suspected vulnerabilities through a private GitHub security advisory once this repository is public. Until that is available, contact the maintainer directly and avoid posting exploit details in public issues.

Include:

- affected version or commit
- reproduction steps
- expected and actual behavior
- impact
- whether the issue requires a malicious repository, malicious dependency, or untrusted downstream test target

## Safety Model

`go-prism` may run Go commands and downstream canary commands configured by the user. Treat configured downstream repositories and commands as code execution.

Security rules:

- Do not scan or test repositories without permission.
- Do not include secrets in config, reports, issue text, or AI prompts.
- Prefer read-only GitHub permissions.
- Use write permissions only when posting PR comments.
- Do not use AI output as the source of truth for safety decisions.
