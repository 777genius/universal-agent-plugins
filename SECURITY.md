# Security

For vulnerability reports, use the repository's
[GitHub Security Advisories private report](https://github.com/777genius/plugin-kit-ai/security/advisories/new).
Do not open a public issue for undisclosed security problems.

When reviewing plugin behavior, treat hook stdin, path-like fields, tool
arguments, prompts, and any generated native config as untrusted input until it
has been validated for the target runtime.

Removal and config-ownership behavior is fail-closed: the installer changes only
files it can prove it owns, refuses collisions instead of overwriting foreign
configuration, and `remove` deletes exactly what was installed. See
[docs/REMOVAL_SAFETY.md](docs/REMOVAL_SAFETY.md) for the rules and the
end-to-end evidence across all supported clients.

Security-sensitive release checks before publishing:

- verify release assets are built from the intended candidate SHA
- keep `checksums.txt` attached to the root GitHub release
- verify release attestations for published artifacts with `gh attestation verify`
- verify Homebrew, npm, and PyPI channels consume the same release assets
- keep `dependency-review`, `govulncheck`, and `CodeQL` green on the candidate branch
- record any waived live failures in the release notes with scope and reason
