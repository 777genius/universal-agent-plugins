# Threat Model

This page records trust boundaries for the preserved `plugin-kit-ai`
implementation. For current installation, also read the
[installer support boundary](SUPPORT.md#universal-agent-plugins-installer),
[removal safety](REMOVAL_SAFETY.md) and source-specific acquisition contracts.
A historical runtime contract does not qualify the current release surface.

This document captures the current trust boundaries for shipped `plugin-kit-ai` surfaces.

## Trust Boundaries

| Boundary | Untrusted Input | Trusted Component | Current Mitigation | Remaining Gap |
|----------|-----------------|-------------------|--------------------|---------------|
| Runtime payload decode | Claude stdin JSON, Codex argv JSON | descriptor-backed codec and runtime dispatch | explicit decode functions, typed handlers, decode error path, 1 MiB payload ceiling | no per-event tighter ceilings yet |
| Invocation args/env | CLI args and environment variables | resolver and process envelope builder | explicit invocation resolution, unknown invocation failure | ambient env remains broad outside targeted ports |
| Scaffold/config files | local mutable repo files | generated scaffold + validate rules | required/forbidden file checks, schema-level validation for `plugin.yaml` and `launcher.yaml`, `go build` validation | target-native extra docs still rely on per-surface parsers |
| Release assets | GitHub release metadata, archives, raw binaries | installer selector/checksum/fs pipeline | checksum verification, asset selection policy, atomic writes, GitHub artifact attestations on release assets | installer path does not yet enforce attestation verification |

## High-Signal Risks And Coverage

- malformed Codex notify payload: covered by runtime regression tests
- malformed Claude hook payload: covered by subprocess and runtime decode tests
- checksum mismatch or missing checksums: covered by installer tests and exit-code contract
- asset ambiguity: covered by installer selector tests
- mixed scaffold markers causing wrong platform assumptions: covered by validate tests

## Accepted Gaps For This Phase

- no installer-side attestation enforcement
- no per-event payload ceilings below the global 1 MiB limit
- no new public debug API
- real external CLI smoke still depends on opt-in local auth and network conditions
