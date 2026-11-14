# Security

For vulnerability reports, use the repository's
[GitHub Security Advisories private report](https://github.com/777genius/universal-agent-plugins/security/advisories/new).
Do not open a public issue for undisclosed security problems.

When reviewing plugin behavior, treat hook stdin, path-like fields, tool
arguments, prompts, and any generated native config as untrusted input until it
has been validated for the target runtime.

Removal and configuration changes require verified ownership. The installer
refuses unresolved collisions and preserves foreign or externally edited
content. Removal can retain plugin data and shared source caches, and a client
may require an explicit external uninstall. See
[removal safety](docs/REMOVAL_SAFETY.md) for these rules and the distinctions
between deterministic fixture tests and version-bound native evidence.

Security-sensitive release checks before publishing:

- verify release assets are built from the intended candidate SHA
- keep `checksums.txt` with the exact `agentplugins-vX.Y.Z` GitHub release
- verify release attestations for published artifacts with `gh attestation verify`
- verify the active native, Homebrew and npm channels consume the intended
  verified artifacts; historical PyPI wrappers are a separate product boundary
- keep `dependency-review`, `govulncheck`, and `CodeQL` green on the candidate branch
- record any waived live failures in the release notes with scope and reason
