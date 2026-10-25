# VS Code 1.143.1: untrusted workspace metadata observation

Genuine hosted collection succeeded on 2026-10-25 at source
`f48d685133105e3ec7922acf811ee30a95830ef8`, tree
`11593c9f96d45f17fc671b53f90299927160fa33`, in
[run 40188310034](https://github.com/777genius/universal-agent-plugins/actions/runs/40188310034),
[job 121153707553](https://github.com/777genius/universal-agent-plugins/actions/runs/40188310034/job/121153707553).
The exact job's official setup log reports Ubuntu 22.04.5, image
`ubuntu-22.04`, version `20261012.322.1`; this is observed image identity,
not an inference from the workflow label or the latest image inventory.

Provider artifact `vscode-1.143.1-untrusted-workspace-linux-amd64`, ID
`12597047500`, is 22,575 bytes, SHA256
`4621b804126db48b64930d66a308f2906c047acaeb0cf3fffb58bfd442407b07`.
The sole outer member is `research.zip`. All 27 raw inner text members are
preserved byte-for-byte here. `provenance.json` records both ZIP identities,
exact source/image identities and every member length/hash. Binary ZIPs are
not committed. No vendor code was executed during artifact intake.

## Verified collection boundary

The worker and ROOT independently verified the provider digest, exact bounded
flat unique member closure, all 14 separate command streams and five merged
native diagnostic streams, source/publisher agreement and the four observation
results. Both frontend and root cleanup completed. All five native child
records report planned stop, proven tree reaping, no overflow and no error.

Each same-process prelude emitted UID/EUID/GID/EGID 1001, empty groups, zero
capability sets, no-new-privileges, a network namespace distinct from its recorded
parent and only loopback. The unchanged publisher invocation contains no
sandbox-disabling or trust-disabling flags. The exact publisher remains
1.143.1 / `e646868227c3e3a9e3f24277c8c7220be42c0be7`; archive and required
ELF/CLI/product digests match the pinned records. This successful observation
does not retrospectively prove the exact cause of the earlier Ubuntu 24.04
sandbox failure, whose original negative evidence remains preserved separately.

The observer ran in test extension mode 3 with exactly the two reviewed proposed
APIs, `mcpServerDefinitions` and `chatPromptFiles`. This is a bounded research
extension on the shipped workbench, not a stable public CLI inventory API or a
production dependency. Every probe selected exactly one local folder and
reported `trusted:false`, `skills_enabled:true`, MCP access `all`, and restrictive
MCP autostart `never`. No trust was granted or policy weakened.

## Actual metadata and its attribution

The base directory below is the exact sealed task root in the raw files.
Every raw native row is retained; names were not collapsed before attribution.

| Probe | Parsed skills | Configured MCP metadata |
| --- | --- | --- |
| A | Exact A `.github/skills/uap-a/SKILL.md`, A's same-name `uap-shared` with `A-SHARED`, and personal `home/.copilot/skills/uap-user/SKILL.md`. Sources are respectively `local`, `local`, `user`. | `uap-a` at `http://127.0.0.1:9/UAP-A` and the distinct user URL; no B marker. |
| B | Exact B `.github/skills/uap-b/SKILL.md`, B's `uap-shared` with `B-SHARED`, and the same personal user marker. | B marker plus the user marker; no A marker. |
| A restart | Observation JSON is byte-identical to A, including actual folder, trust, policy, skill paths and MCP definitions. | A and user markers return in a fresh workbench process. |
| A after declared fixture removal | Only personal `uap-user` and personal `uap-shared` with `USER-SHARED`; A's removed skill paths are absent. | Only the user marker remains; A's removed registration is absent. |

Skill attribution uses actual API-emitted file URIs and source values. The MCP
API emits definitions and marker URLs but no configuration-file origin, so its
source attribution is the explicit inference from unique sealed fixture URLs.
Do not relabel it as an emitted `.vscode/mcp.json` path. The same-name project
skill wins in the A/B table and personal fallback appears after removal; this
does not establish independent simultaneously active same-name versions.

The controller authored these inert fixtures and removed only its declared A
fixture files. No UAP project adapter, ownership receipt, package transaction or
native MCP handshake was exercised by this research collector. Actual configured
HTTP metadata supports the `.vscode/mcp.json` component path/schema gate; SSE
spelling has its separate immutable primary-source schema evidence, not a tested
SSE connection. All skills disable model invocation.

## Interpretation and next implementation gate

ROOT independently gave GO for the reviewed component paths and the conditional
Prepared adapter plan after inspecting the original ZIP, all 19 stream receipts,
five isolation/teardown records, four actual metadata results and both cleanup
records. This permits implementing the separate project skills/MCP backend and
testing its owned file lifecycle. It does not qualify native project activation.

Every original `native_project_qualified`, `native_mcp_initialization_qualified`
and `native_runtime_qualified` field remains false. Connection attempts were not
instrumented. MCP startup/operation, model or skill execution, trust acceptance,
authentication, active sessions, remote/multi-root projects and effective Local
versus Copilot selection remain unqualified. The adapter must retain
Prepared + installed-file verification and require the user's normal client
trust/session decisions. Final assembled-source installer tests and review
remain necessary; later revisions are not implicitly covered by this observation.
