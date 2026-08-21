# Security policy

## Report a vulnerability

Do not open a public issue for leaked credentials, command execution, path
escape, malicious skill instructions, or a package that connects to an
unexpected endpoint. Contact the repository owner privately through the
[GitHub private vulnerability report](https://github.com/777genius/universal-agent-plugins/security/advisories/new).

Never include live tokens, cookies, personal data, or production credentials in
a report. Use redacted logs and a minimal reproduction.

## Scope

This repository packages configuration and skills. It does not operate the
linked vendor MCP servers and cannot fix upstream service vulnerabilities.
Reports about an upstream server should also follow that vendor's security
policy.

Agent Plugins 1.0 does not define a permission system, signature verification,
sandboxing, or a portable secret store. Users must review every server's tools
and configure authentication through their client.

## Release integrity

- Install the catalog from an explicit release ref, not mutable `main`.
- GitHub Actions are pinned to full commit SHAs and checked by Dependabot.
- `main` requires CI and review except for the dedicated Directory publisher
  App's deterministic same-tree marker fast-forward. A separate no-bypass rule
  still requires linear history and forbids force pushes and deletion.
- `v*` tags cannot be updated or deleted after creation.
- Published releases from `v0.1.1` onward are immutable on GitHub, locking the
  tag and assets and generating a release attestation.
- `v0.1.0` predates release attestations. Use `v0.1.1` or newer.
