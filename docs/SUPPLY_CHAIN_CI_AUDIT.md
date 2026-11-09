# Supply-chain CI review and local policy

Status: source implementation in the draft supply-chain PR. **Do not present this
as a shipped release until the PR is merged and a compatible CLI is published.**

UAP already has a signed Directory, exact versioned source acquisition,
local security assessment, `uap.toml` and `uap.lock`. This change adds a
human-reviewable *inventory*, not a second trust engine or a runtime sandbox.

## Offline local integrity in CI

From a checkout with the exact cached packages for its pinned agreement:

```sh
agentplugins deps check --scope project --format json > uap-integrity.json
agentplugins deps check --scope project --format json --policy ./uap-policy.yaml > uap-policy-audit.json
```

A zero exit means that the manifest, lock, cached exact package bytes and
dependency graph were verified. Missing cache returns **unavailable** and a
nonzero exit. Never treat empty or partial results as a clean bill of health.
With `--policy`, the same versioned JSON report includes a local admission verdict for source hosts, requested integrity pins and MCP transports. A denial or missing policy returns a nonzero exit and does not mutate native files.

This command does not fetch sources, check current revocations, execute MCP
servers or authenticate an AI model. The machine report may contain local
manifest/lock paths; keep artifacts private or redact them before publishing.

## SBOM export

```sh
agentplugins deps sbom --scope project --type cyclonedx > uap-cyclonedx.json
agentplugins deps sbom --scope project --type spdx > uap-spdx.json
```

Both forms are JSON. They first run the same **offline** strict dependency
integrity check and fail nonzero when any package is unavailable, altered,
invalid or inconsistent. They inventory the entire pinned transitive graph,
including package aliases, version when known, source revision and canonical
UAP tree digest. An exported tree digest is NOT a byte-for-byte SHA256 hash
of the archive and is deliberately not emitted as an SBOM file checksum.

Sorting is stable. The SPDX creation timestamp uses a documented fixed
reproducibility marker, not the time of a vulnerability or signature check.
The result is **not** signed provenance, a vulnerability certificate, an
automated license scan or an attestation of model/tool execution.

## Explicit local source-integrity policy

Use a small, local YAML file on a deliberately selected `install` or
`deps sync` operation; this is NOT an organization policy delivery system.

```yaml
schema_version: 1
sources:
  allow_hosts: [github.com, git.example.com]
  deny_hosts: [untrusted.example.com]
integrity:
  require_pinned_git: true
  require_archive_sha256: true
mcp:
  allowed_transports: [stdio, streamable-http]
```

```sh
agentplugins install --scope project --locked --policy ./uap-policy.yaml
agentplugins deps sync --scope project --locked --policy ./uap-policy.yaml
```

The policy evaluates **all pinned dependencies, including transitive ones**.
An explicitly empty source allow-list denies all sources; an explicitly empty MCP transport allow-list denies all MCP transports. Omitting either list leaves that dimension unrestricted.
A denied source or missing *requested* source pin rejects the selected lock
before convergence, repairs or managed client mutations. It does not bypass
normal signed-source, package integrity or scanner checks. Policy files use a
single bounded YAML document and reject unrecognized fields. No policy file
means existing install behavior remains unchanged.

The `--policy` flag is **not** supported on `install <source>` — this is
intentionally rejected instead of silently disregarding the policy.

**Limits:** Policy currently checks exact source hosts and explicit Git/Archive
pin requirements. It has **no** organization inheritance, hosted policy
discovery, remote scanner or admin override system;
those require separate design and validation. It does not silently reuse APM's
policy schema. Source host allow-lists do not certify trustworthiness by
themselves.

## Testing and evidence

Required before ready-for-review or merge: CLI smoke for each command on a
verified lock; CycloneDX/SPDX JSON schema validation; malformed YAML and
untrusted source regressions; transitive graph and no-write on policy block;
actual Linux/macOS/Windows CI; review of error paths and paths/secrets in
machine outputs. Successful unit fixtures do not replace these tests.

Related reference: [local dependency integrity](dependency-integrity.md).

### Policy admission timing

The opt-in policy is evaluated against the complete resolved lock before any convergence repair, and re-evaluated at the final pre-mutation checkpoint after user confirmation. This guards against the policy being changed between preview and apply. Explicitly empty `allow_hosts: []` and `allowed_transports: []` lists are restrictive rather than equivalent to omitted lists. This remains a local policy; it does not supersede UAP's existing source trust, scanner or revocation checks.
