# Agent Plugins 1.0 compatibility release preparation

Status: unreleased work, 2026-09-06. This is a preparation record, not a release
receipt or native runtime claim. Existing version-bound lifecycle evidence is in
[the August 30 record](AGENTPLUGINS_CLIENT_E2E.md); its artifact identity and
limitations must not be reassigned to this candidate.

## Source evidence and limits

| Target surface | Version-bound source fact | Candidate limitation |
| --- | --- | --- |
| Cline extension v4.1.17 | [Schema](https://github.com/cline/cline/blob/v4.1.17/apps/vscode/src/services/mcp/schemas.ts) accepts flat and nested cwd; [McpHub](https://github.com/cline/cline/blob/v4.1.17/apps/vscode/src/services/mcp/McpHub.ts) passes cwd to stdio transport | Source support permits direct projection; native launch with candidate not proven |
| Cline CLI cli-v3.0.61 | [Client](https://github.com/cline/cline/blob/cli-v3.0.61/sdk/packages/core/src/extensions/mcp/client.ts) forwards transport.cwd to spawn | Separate surface; source support does not prove candidate CLI integration |
| OpenCode v1.18.29 | [Connection code](https://github.com/anomalyco/opencode/blob/v1.18.29/packages/opencode/src/mcp/index.ts) starts remote with Streamable HTTP and may fall back to SSE | HTTP-first supports HTTP mapping; explicit SSE selection cannot be preserved by the native config |
| Windsurf / Devin on macOS and Linux | Candidate uses a Unix launcher for cwd, environment and process lifecycle | macOS launcher tests and Linux compilation are not native client handshake evidence |
| Windsurf / Devin on Windows | No Windows launcher backend in this candidate | Stdio explicitly unsupported; native process lifecycle support remains unimplemented |

These are reviewed versions, not inferred minimum versions or guarantees for all
client releases. Projection/unit tests only prove the behavior they exercise.

## Release evidence to collect

Reuse the existing structured E2E transcript convention and release workflow
artifacts. Each completed row must bind exact CLI version, commit SHA, binary
SHA-256, OS/architecture, target product/version/surface, component/transport and
package tree/manifest digests. Use explicit `not evaluated` for missing proof.

| Candidate scope | Schema/loader | Prepared | Installed | Activated | Handshake/tool call | OAuth |
| --- | --- | --- | --- | --- | --- | --- |
| Cline extension stdio with cwd | Pending final candidate report | Pending candidate evidence | Not evaluated | Not evaluated | Not evaluated | Not evaluated |
| Cline CLI stdio with cwd | Pending final candidate report | Pending candidate evidence | Not evaluated | Not evaluated | Not evaluated | Not evaluated |
| OpenCode Streamable HTTP | Pending final candidate report | Pending candidate evidence | Not evaluated | Not evaluated | Not evaluated | Not evaluated |
| OpenCode declared SSE | Valid input may be unsupported for this target | Must report unsupported | Not claimed | Not claimed | Not claimed | Not evaluated |
| Windsurf macOS/Linux stdio with cwd | Pending final candidate report | Pending candidate evidence | Not evaluated | Not evaluated | Not evaluated | Not evaluated |

Windows Windsurf stdio is explicitly unsupported in this candidate. It is not a
pending supported path: a native Windows launcher and lifecycle proof are needed
before that capability can be advertised.

This table intentionally contains no new successful runtime or release records.
Record focused check results against the final commit, then attach reproducible
exact-head CI and release artifact provenance using the existing mechanisms.

Use only new disposable profiles/projects and test identities. A benign fixture
may report cwd/argv/env and speak minimal MCP. Keep HTTP endpoints loopback and
headers secret-free. Demonstrate local and immutable Git installation without
Directory membership, update preserving PLUGIN_DATA, owned-only removal, and
healthy skill/HTTP siblings surviving unsupported transport selection. Separately
prove safe global abort for ownership conflicts. Preserve the exact conformance
corpus pin and strict reporting; it tests loading, not client execution.

Do not fabricate adoption evidence: independent reproducible user cases remain
to be collected. Two or three cases are a practical aim, not an upstream rule.
Release only via the existing verified workflow. Preserve failed release bytes
and evidence; rollback distribution to a known artifact instead of replacing
bytes under an existing version or automatically modifying user installations.

## Updating historical projections

Installing a newer CLI does not itself refresh an existing renderer or launcher.
An update with the same source and revision normally returns `NoChange`. The
unreleased candidate has one narrow exception: explicit update may remove a
proven previously delivered MCP component that is now statically unsupported,
such as a historical OpenCode SSE projection. Add and repair must not reactivate
that component; they direct the user to controlled update. This exception does
not refresh a helper or renderer merely because the CLI changed. An intact old
artifact plus an accepted new package revision can deliver the newer projection
and helper; update still verifies the old artifact before mutation.

Do not use plain `update` as a promised recovery for a damaged historical artifact.
If the current CLI cannot reproduce the recorded bytes for exact repair, recovery
requires a verified compatible older CLI for repair, or an explicitly verified
owned remove/reinstall path that preserves existing data under lifecycle rules.
Removal can itself be blocked by integrity or ownership checks. No cache lookup,
forced integrity bypass or automatic migration is implied by this candidate.

The unsupported-component withdrawal exception is local candidate behavior under
implementation and verification, not shipped compatibility or a reported test
pass. It introduces no state-version migration and bypasses no ownership or
integrity checks. It does not turn damaged historical artifacts into repairable
ones or justify dropping unrelated healthy components.
