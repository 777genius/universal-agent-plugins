# Threat Model

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

## Unreleased Darwin authoring reader v2

The bounded PR 1 reader deliberately reduces the hostile concurrent writer
boundary to **quiescent local APFS**; it does not provide equivalent security.
Static content remains untrusted. Source and ancestor bindings must stay unchanged
from before Reader.Open resolution until Lease.Close cleanup, including the
core-decoding gap. Unrelated outer siblings may change. A hostile writer can
cause a FIFO/device open or an out-of-scope/excluded legacy read before a later
error. fstat, no subsequent Read calls and repeated checks cannot undo that open
or prove atomicity/absence of ABA. Read-only APFS proof does not cover this case.

Metadata-first traversal, contained relative links, hardlink/legacy exclusion,
private bounded capture and fatal observed-change errors remain mandatory within
the declared boundary. Per-thread libc materialization OFF and metadata dataless
rejection protect offline acquisition on supported already-mounted local APFS;
no-cgo builds reject before source access. Autofs/network/third-party redirectors
are outside the profile, and no claim covers unrelated OS background traffic.

The [controlling ADR](./adr/0006-standard-first-authoring.md#darwin-acquisition-clarification-2026-09-07-pr-1)
and [public contract](./AUTHORING_PUBLIC_CONTRACT.md#darwin-acquisition-profile-pr-1-native-proof-pending)
define the exact interval, residual risks and native prerequisites. Native
writable/device/dataless proof is pending; source stays unmerged until native
review/proof. Installer invariants, Linux/Windows security, real stage validation,
transaction rechecks and macOS release gates are retained.
