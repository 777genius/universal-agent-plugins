# UAP Management API, language clients, and Agent Teams integration

Status: implementation plan v12 — remaining user-scope SDK client qualification.
Owner authorization: 2026-10-24. Implement end to end, open draft PRs early, publish
every commit immediately, review plan revisions and implementation independently,
verify actual Electron interaction, merge only after required gates and resolved
review findings. This is not authorization to bypass repository protections or
publish a new public release.

## Continuation checkpoint — v12 (2026-10-29)

The owner requested local Linux verification of the remaining admitted user-scope
Management SDK clients and merge of the resulting E2E work. PR #500 and the
Claude/Codex continuation [PR #558](https://github.com/777genius/universal-agent-plugins/pull/558)
are merged. The latter qualified Claude 2.1.263 and Codex 0.153.4 through the actual
SDK and Go engine on source `d52f2248fbb4817e46927d914fee343ca1ccf4a3`
([native run 40754108543](https://github.com/777genius/universal-agent-plugins/actions/runs/40754108543)).
Those receipts remain tied to their original source.

This continuation adds local pinned HTTPS fixture journeys for Cline and Gemini
native Skills, Pi native extension auto-discovery, and the existing Kimi/Kiro/
Windsurf SDK configuration lifecycles. Four real regressions are repaired:
legacy user native-config attempts remain inspectable; read-only Cline preparation
refusals report unchanged native state; owned Windsurf MCP entries prove the captured
Management profile; Copilot's actual built-in plugin section no longer makes its
user plugin inventory unrecognizable. The Copilot correction is confined to its
adapter and retains the native runner's tree containment and error evidence.
Selected/project recovery authority and later mutation failure semantics remain
unchanged.

After reconciliation with main `60967b8b6ca79245a61caf28895aa09c2c9d74a2`,
all six local journeys passed on clean published source
`f5b6ed9f409c5c1b1748b7625db5076cba944a80`, using an engine rebuilt from that
commit. The [committed receipt](../evidence/management-sdk/remaining-local-linux-f5b6ed9f409c.json)
records executable/fixture identities, actual native observations, operation states,
process termination and the full Gemini bundle identity. It qualifies those exact
bytes; later documentation or CI wiring commits are not relabeled as that run.

Cline 3.0.73 and Gemini 0.65.0 each passed eleven lifecycle operations, including
an expected edited-projection `partial` result followed by successful recovery.
Pi 1.4.0 passed five operations and fresh extension discovery/command observations;
it qualifies the admitted extension lifecycle, not Skills or MCP. Kimi 2.3.1,
Kiro 2.33.1 and Windsurf each passed four SDK configuration lifecycle operations.
Kimi/Kiro require native authentication and Windsurf requires an editor refresh;
their receipts explicitly record `nativeLoaded: false`. Cursor's existing user
editor route remains manual. The pinned Agent archive/version can now be acquired,
but Agent presence does not establish the separate editor capability or native
loading; see the [Cursor detection boundary](../../install/integrationctl/agentplugins/clients/cursor/README.md).

Copilot 1.0.103, Grok 1.0.67 and Qoder 1.1.78 need Go-managed native processes.
This local container lacks the required `/proc/<pid>/task/<tid>/children` interface;
no production process guard is relaxed. A focused three-client Linux workflow
qualifies only these SDK journeys and binds receipts to the checkout and executable
hashes. It uses a disposable hosted Ubuntu 24.04 runner after the shared AMD64
pool had no active runners and left the native jobs unassigned; repository-wide
runner variables and other workflows are unchanged.
[PR #566](https://github.com/777genius/universal-agent-plugins/pull/566)
records the current three-client results and required repository checks. Independent
review reported no remaining P1/P2 finding; execution gates remain authoritative.
The later Copilot correction has its own actual local
[1.0.103 inventory evidence](../evidence/management-sdk/copilot-1.0.103-linux-listing.json)
and adapter regressions; that diagnostic is not relabeled as a full SDK journey.
Local SDK protocol/owned-process tests passed 46/46 without skips, and the packed
npm consumer passed type checking and runtime imports. CodeRabbit identified a
Copilot observer counting a fixture-named path continuation as a second plugin;
an actual local native run reproduced the failure, and counting only plugin
bullet rows corrected it while retaining malformed/truncated-row refusal.
No App/runtime architecture,
S25/S26 project support, VS Code
changes or public SDK release is included.

## Continuation checkpoint — v11 (2026-10-28)

Reconcile the existing integration with main
`27b96f2468acae1b14b5dfeac905384bc50e8b71`, retaining both the new profile/state
guards and Management's reviewed removal and recovery authority. Two narrow
regressions were found while completing the existing qualification:

- macOS can return EPERM when a process group contains only an unreaped zombie.
  The native evidence helper reaps its exact exited child and retries that group
  once; a live child or persistent denial still fails, and surviving descendants
  must still be terminated. Existing capture limits and cleanup deadlines remain.
- Retained Management Skill plans must preserve the explicitly selected targets
  introduced by main's shared preparation. A real HTTPS regression installs into
  Claude and Codex, edits the unselected Claude Skill, then repeats the Codex-only
  request. The reviewed actions, Apply result, sibling bytes/identity and saved
  intent must stay scoped to that selection.

The completed preceding App 42c/UAP 1d desktop receipt already proves three-client
plugin installation, version-2 update, targeted removal, local Skill recovery,
HTTP authority and S27 admission. Its platform/package gates also completed.
UAP 1d had 62 successful checks and two failures: the macOS helper above and the
separate CodeQL findings check (2 Critical / 10 High). These historical results
do not qualify this reconciled source; fresh Required/native/platform checks and
a matched downstream SDK/native pin with actual Electron evidence are required.

The accepted matrix uses engine, transport, renderer and native evidence at their
actual layers; it does not require duplicating every lower-layer scenario in the
desktop harness. Per-member managed Skill/plugin-MCP consumption and S25/S26
remain separate from the qualified user-scope management lifecycle. The previously
reserved runtime/profile architecture discussion is not resolved by this E2E
continuation. No new installer, runtime workaround, scope capability or public
release is introduced, and the findings gate remains authoritative.

## Previous checkpoint — v10 (2026-10-26)

The integration branch is reconciled with main
`d5c16856cef67a1457704971d46765739141ab08`, including the project implementation
merged through #499 and its #519 evidence cleanup. The old #472/#475/#480 draft
statuses below are historical. Management still advertises and admits only its
qualified user scope; S25/S26 require the separate scoped Management/Host seam.

Shared dependency application preserves the selected environment identity,
pre-lock namespace validation, Skill context checks before publication/native
effects, and durable publication accounting. A default input-check callback must
read the request's current bytes after publication rather than capture its old
value. Independent review identified that merge defect; actual registry and HTTPS
Skill lifecycle tests failed after intent publication before the correction and
passed afterward.

Removal-policy inspection now carries the same scoped target context as actual
deactivation. Project component removal does not inherit user-plugin manual
uninstall requirements. The public single/group Prepare-to-Apply regression fails
when that context handoff is removed and passes with the reconciliation. Existing
user removal rules and the Management scope guards remain unchanged. Directory
swap journals retain the confined same-descriptor read introduced for alert #55.

Local verification covers the generated contract drift check, SDK build/typecheck,
27 SDK protocol/owned-process tests and the packed consumer; Management transport,
local authoring, registry/HTTPS, package-link and journal checks; and real HTTPS
archive project lifecycle/cold restore checks for selected adapters. Local process
execution cannot qualify Git/native-process scenarios because this host lacks the
required `/proc/<pid>/task/<tid>/children` observation primitive. Those checks
remain required on a qualified CI host; no process guard or test assertion is
relaxed. Duplicate proxy-variable casing was removed only from the local test
process environment to satisfy the existing Codex admission contract.

Fresh published-source Required/platform CI, downstream matching SDK/native pin,
Electron/package qualification and individual CodeQL disposition remain gates.
Previous green checks and old artifact receipts do not qualify this merged tree.

## Current checkpoint — v9 (2026-10-25)

The actual Agent Teams desktop journey now passed on head
`42c412648cd513f4ca2d9656aaf5ceff71e2e323`, synthetic merge
`7349c4ded9d9662ae87015efbe72fedf6a63c241`, with executable source 5224:
[run 40189232897](https://github.com/777genius/agent-teams-ai/actions/runs/40189232897).
The [complete receipt](https://github.com/777genius/agent-teams-ai/pull/989#issuecomment-6397195988)
records real OpenCode MCP initialize/list/call, an exact nonce returned to the
scripted provider, and a fresh native session with no MCP connection after UI
removal. Its 19 screenshots and zero renderer errors also cover the existing
install/copy/update/remove/edit/restart/restore and signed-registry journey. It
qualifies those exact bytes, not live-model/OAuth behavior or the changes below.

The screenshot review identified two concrete P2 usability issues, now corrected
without a new history service, scanner policy or installer implementation:

- Same-name local restore points previously had identical labels. Optional
  `recordedAt` and `originalAction` now come from the existing durable operation
  outcome. A restore plan includes the selected, identity-checked `restorePoint`.
  Old observations retain a known timestamp but do not invent an original action.
  These are display fields; the existing private receipt, snapshot, ownership,
  mutation lock and stale-plan checks remain authoritative.
- Non-CLI security review previously returned only a warning count. It now
  projects bounded, trusted static reasons for seven verified lintai v0.1.3 rule
  codes, including the actual insecure-HTTP MCP warning. Raw finding messages and
  paths are not exposed. Unknown rules and unpublished/omitted findings receive
  honest fallback text. Blocking findings and explicit existing CLI acceptance
  retain their prior behavior.

Both slices received independent source review with no remaining P1/P2 finding.
The complete management transport package and actual local-Skill backend tests
passed. A restart regression distinguishes same-name edit/delete observations,
binds the selected review to the same point, covers legacy/invalid identity
metadata and confirms stale Apply refusal. Eleven focused warning/CLI compatibility
tests passed after the new warning assertions failed against the old code. Both
DTOs were produced by the pinned upstream generators; SDK build and typecheck
passed. No generated DTO was edited by hand.

This checkpoint changes executable source and generated SDK types. Agent Teams
must repack that committed SDK, pin its matching engine source, and rerun the
current desktop/package/Full qualification gates. The strengthened desktop
journey checks both restore labels and the exact selected confirmation, plus a
specific warning reason before Apply. The previous 5224 Python/native receipt is
[preserved separately](https://github.com/777genius/universal-agent-plugins/pull/500#issuecomment-6397042144);
it is not relabeled for this new source. Formal CodeQL disposition and the
separate S25/S26 project/Host prerequisites remain open. No protection, scanner
query or approval gate is bypassed.

## Previous checkpoint — v8 (2026-10-25)

The executable/SDK source remains
`5224ebd5f66bbdfab0bef906dba0b089062f6073`, version
`0.1.91+agentteams.5224ebd5f66b`. This checkpoint changes documentation only;
advancing the documentation head does not identify different executable bytes.
The matching SDK was rebuilt and packed from that source, with unchanged SHA-256
`31eb798b31be66d1c4d80d1cfa094934fa786cf2b46313360b4c6a8f6bfdd96c`.

[Required 40186976337](https://github.com/777genius/universal-agent-plugins/actions/runs/40186976337)
passed all six jobs on 5224, including the complete required test lane, generated
contract, SDK lifecycle and actual packed consumer. The retained-plan correction
also has the real-HTTP red-before regression described in v7. Terminal UI Linux
and macOS passed; Windows and the Go CodeQL analysis were still running/queued
when this checkpoint was written. A neutral aggregate missing Go analysis is not
security clearance. The documentation head must satisfy its applicable CI too.

Agent Teams head `501916f1532d0c2bd48eb4867a09228116009c5f` pins these exact
engine/SDK inputs. Its [v10 plan](https://github.com/777genius/agent-teams-ai/blob/501916f1532d0c2bd48eb4867a09228116009c5f/docs/extensions/uap-management-integration-plan.md)
records the implemented sidecar capability, source-reference correction, packaged
MCP cleanup and independently reviewed click-readiness correction. The desktop
sidecar now requires an unforgeable capability before invoking managed methods;
Host/Origin/socket checks remain additional admission checks, not authentication.
Only owned main-frame IPC grants browser authority, distinct from the Go bearer.

The [current desktop lane](https://github.com/777genius/agent-teams-ai/actions/runs/40188255677)
adds real OpenCode MCP initialize/list/call and an exact nonce roundtrip through
a scripted local provider after ordinary UI installation. A fresh session after
UI removal must make no MCP connection. This is actual native tool transport,
not live-model, account-authentication or OAuth qualification. Its clean receipt,
current Full qualification and package matrix remain pending at this checkpoint.
The live PR bodies record subsequent exact-revision results without relabeling
older receipts. Current formal security disposition is still outstanding.

The matrix and E1–E7 below retain their original BBC7/v6 evidence. They are not
retroactively asserted for 5224 or a later app head. S06 multi-profile, S07/S08
moving/bulk update, S16/S17 cancellation, S21 offline, S23 in-flight context switch
and S27 active-run behavior retain their cited engine/renderer/admission evidence;
the desktop journey does not execute all of them. S25/S26 remain blocked on the
separate draft project/Host prerequisites. No unfinished adapter code is imported.

## Previous checkpoint — v7 (2026-10-25)

Continuation starts from published UAP `6f8e2786d046da56d933167129e514fb10ff7450`
and Agent Teams `52fcacc3cc69c6df8882ecbb7310abfb9c4b2c19`. The v6 evidence below
remains tied to its named engine bytes; it does not qualify later native changes.

Independent review found a bounded-session defect: after preparing the allowed
16 reviews, Apply also requested a new review slot and refused every retained
plan with `REVIEW_LIMIT`. Only creation needs a new slot. Apply now retains the
existing busy, process-lock, freshness, durable-acceptance and replay checks
without claiming another review slot. The new real-HTTP regression reproduced
the original refusal for dependency and local-Skill plans, then passed with the
correction. It also preserves the seventeenth-preparation refusal and proves
slot reuse and accepted-request deduplication. The complete management transport
package passed on Go 1.26.10. Current applicable CI and a matched downstream
engine/SDK source pin remain required for this correction.

The previously pending clean desktop run has completed:
[Agent Teams E2E 40183926081](https://github.com/777genius/agent-teams-ai/actions/runs/40183926081)
passed on synthetic merge `1b00532a5db7be78a38824373d984c2ee23d7ded` (head `52fcacc3`,
base `eb67de95`), using the BBC7 engine. Its evidence is retained as artifact
`12595184310`. This qualifies that merge's desktop journey, not the capacity
correction, authenticated model/tool execution, or project/worktree activation.

The substantive App Full qualification run `40183926041` found one packaged-MCP
child-isolation failure; the later edited-event no-op is not substitute evidence.
That failure, the current package matrix, formal CodeQL disposition and the
HTTP-sidecar authority review are being resolved before merge. Scanner rules and
branch protections remain intact. Project/Host prerequisites remain separate.

## Previous checkpoint — v6 (2026-10-25)

This section and the S01–S30 matrix preserve the historical v6 qualification state.
The v4/v5 review records below are historical evidence at their named revisions;
their then-open tasks and counts are not assertions about the current artifacts.

| Input | Historical v6 qualification pin |
| --- | --- |
| UAP source | [bbc7e98da96dd7d77db42f65da7afae84a86f799](https://github.com/777genius/universal-agent-plugins/commit/bbc7e98da96dd7d77db42f65da7afae84a86f799) |
| Engine version | `0.1.91+agentteams.bbc7e98da96d`; compatibility baseline is the published `agentplugins-v0.1.91` tag at `4555aa53844ee0078e9036cd82638afd67f77c16` |
| Linux x64 engine SHA-256 | `fcddfe935e8bfbdbe45b2131aa934240c0e2e2e236fb8c3c84c624a95b9c1b76` |
| Vendored `@uap/management-sdk` tarball SHA-256 | `31eb798b31be66d1c4d80d1cfa094934fa786cf2b46313360b4c6a8f6bfdd96c`; provenance names the same UAP source |
| Agent Teams checkpoint | [e1798241e114fe6b530541e398491a8d14d1323d](https://github.com/777genius/agent-teams-ai/commit/e1798241e114fe6b530541e398491a8d14d1323d) |
| Actual native consumer used locally | OpenCode `1.18.34`, SHA-256 `9ca0b9953d49997601655e54f846a3efa464f237e47c6f1b04716d0f2e64c4c2` |

At this pin, UAP Required and Terminal UI CI, actual signed-registry install/remove,
and unchanged native SDK and independent Python lifecycles passed. The separate
aggregate CodeQL gate is failing with 15 alerts. The [independent security
review](management-sdk-security-review.md) records the complete source disposition;
formal repository disposition remains pending. Final current-app
Electron qualification and package gates remain open; app CI evidence is tied to
its exact revision in E6. Native project/worktree installation remains
dependency-blocked. No public release or completed merge is claimed.

## Outcome and scope

A host application can use the existing Go installer through a typed, language-neutral
management contract: discover plugins or standalone Skills, inspect desired and
observed state, prepare a change, accept exactly that retained plan, observe progress,
update selected/all moving dependencies, restore locked dependencies, repair,
remove, and handle interruption without repeating an uncertain mutation.

TypeScript is the first managed client, not a second installer. Other languages
use the same OpenAPI contract and supported generator. Go installation policy,
resolution, native-client adapters, profile authority, ownership, receipts,
journals, and recovery remain authoritative.

Agent Teams is the first desktop/server consumer. Reuse its Extensions shell,
Plugins/MCP/Skills/API Keys tabs and shared Radix controls. Preserve existing native
Claude plugin identity and local Skills authoring. Add a separate UAP-managed
contract, not a cast into the native marketplace type.

Project installation adapters are a separate owner-directed workstream. This plan
does not recreate or merge unfinished adapter work. Capability negotiation must
accurately reflect the engine used. Unsupported project/local scopes are refused,
never silently downgraded. Complete project/worktree qualification depends on that
workstream being merged and qualified.

## Design baseline and selected tools

Research began at UAP main 4555aa53844ee0078e9036cd82638afd67f77c16 and Agent Teams
main 23617c3e825f775ea27d354d03e17a28bf612586. Those are historical design inputs;
the historical v6 qualification pins and prerequisite heads are recorded separately
above and below. Recheck moving prerequisite status before final integration.

| Concern | Selection | Why |
| --- | --- | --- |
| Contract | OpenAPI 3.0.x | Existing editable schema + generated DTO convention |
| Go transport | standard net/http + oapi-codegen v2.8.0 models | Reuse current loopback service and pinned generator |
| TS wire types | openapi-typescript 7.13.0 | Already adopted; types derive from one schema |
| TS HTTP | openapi-fetch 0.17.0 | Typed fetch, injectable transport, no framework coupling |
| TS runtime/tooling | repository-compatible Node, TypeScript 7.0.2 or newer | New scripts are .ts; do not copy legacy .mts conventions |
| Progress | poll authoritative operation snapshots | Existing service pattern; no extra stream protocol required |
| Electron automation | existing raw CDP harness, pnpm dev:mcp | Real desktop renderer, same maintained harness |
| Future languages | documented OpenAPI Generator recipe | Do not maintain unused language SDKs |

Research sources and inspected examples:

- https://github.com/oapi-codegen/oapi-codegen/tree/v2.8.0/examples/only-models
- https://github.com/oapi-codegen/oapi-codegen/tree/v2.8.0/examples/minimal-server/stdhttp
- https://openapi-ts.dev/openapi-fetch/
- https://openapi-ts.dev/openapi-fetch/api
- https://openapi-ts.dev/openapi-fetch/middleware-auth
- https://openapi-generator.tech/docs/generators/
- https://typespec.io/docs/emitters/clients/introduction/
- https://connectrpc.com/docs/introduction/
- https://google.aip.dev/151
- https://google.aip.dev/155

Alternatives: Connect is a reasonable protobuf-first RPC platform but would add
another authored contract and migrate a working HTTP/OpenAPI foundation. TypeSpec
adds another DSL and its language-client emitters are documented as preview.
Neither improves this first consumer enough to justify the extra toolchain.
Custom JSON-RPC, FFI, WASM, and CLI-output scraping are rejected for this work.

Code generation gives transport consistency, not installation safety. Request
validation, stale checks, idempotency, cancellation, and recovery must be explicit
application behavior with executable acceptance tests.

## Architecture and ownership

1. Composition roots inject the supported client registry and concrete host
   dependencies. Generic code must not import clients/all.
2. Share typed application preparation with CLI and Skills picker. Start from
   PreparedSkillsBatch, environmentSession, dependencyInstaller, and Coordinator.
   Extract only the conversations needed by management; no wholesale CLI rewrite.
3. Management transport owns bounded JSON validation, auth, Host/Origin checks,
   routing, HTTP status, and generated DTO mapping.
4. Management session owns retained prepared handles, expiry, accepted-operation
   observation, and process lifetime. Installer owns native effects and recovery.
5. Pure TS entrypoint owns typed requests, error conversion, wait/onProgress, and
   optional injected fetch. No filesystem, Electron, DOM, React, or Zustand.
6. Separate Node entrypoint starts an explicitly supplied pinned executable without
   shell, validates bounded bootstrap/version/capabilities, retains private bearer
   credentials, and owns only its spawned process.
7. Agent Teams owns current project/profile/runtime selection, actual execution
   cwd, active-run policy, and UI. Both IPC and HTTP call the same feature facade.

Implemented public surface:

- session/capabilities and engine/API version;
- verified catalog search/details and standalone-Skill candidate discovery;
- environment desired/lock/observed state and supported actions;
- plans.prepare for install, skill install, update selected/all, locked sync,
  repair, remove, and recovery;
- plans.apply with opaque plan ID and request ID;
- operations.get, operations.wait in SDK, and explicit operations.cancel;
- fresh inspection and retained recovery preparation.

The authored [OpenAPI schema](../../contracts/management/openapi.yaml) is the wire
source of truth. Shared application use cases remain inside the CLI module; the
existing public Go installer SDK is preserved. No new in-process Go Management
API, daemon, second transport or arbitrary execute-command/filesystem endpoint
was added. Native configuration/authentication workflows remain represented by
capabilities, readiness and required actions, not a universal OAuth/secrets manager.

## Lifecycle invariants

### Prepared authority

Installer Plan has operational fields excluded from JSON. Inspection/recovery also
contains typed authority. Retain the actual Go prepared object/observation and expose
an opaque identifier. Diagnostic DTOs are never replayed as executable authority.
Check again before effects. Unaccepted plans expire after a bounded TTL and are
invalid after process restart. A new plan requires fresh preparation.

### Accepted operation identity and restart

Accepting an operation is a durable boundary. Persist a small acceptance record
with request identity, input fingerprint, environment identity, and correlated
installer operation identities before starting effects. Existing installer
journals/receipts remain the authority for native effects; do not create another
transaction engine.

Same request ID + same accepted input returns the known operation/result or an
explicit interrupted/recovery-required state. Same ID + different input conflicts.
Lost HTTP responses never justify blind resubmission with a new request identity.
After restart, inspect/reconcile prior receipts and require fresh recovery/sync
preparation. Never deserialize and automatically execute a saved plan.

The manifest write is itself a durable effect. TOML publication can succeed before
lock publication fails. Error and cancellation outcomes must cover this case even
when no native binding has committed.

### Cancellation and progress

An HTTP AbortSignal stops waiting, not an already accepted install. Explicit cancel
requests cooperative cancellation. Completion may win a race. Before durable
effects a clean cancellation is possible; after effects report partial/incomplete
or recovery-required with truthful next actions. Do not promise global rollback.

One active mutation per environment is sufficient for v1. Use existing locks and
return busy/conflict rather than inventing a scheduler. Progress reports actual
phase text and binding outcomes, not fabricated percentages. Operation snapshots
are authoritative even if a progress callback or UI poll is lost.

### Sources, updates and ownership

Use the signed directory publication through UAP. Do not parse the frozen registry
index in Agent Teams. Preserve distribution identity, release policy, suspension,
revocation, immutable pins and provenance.

Update selected/all constructs one candidate graph and lock, preserving unselected
pins. Extend the existing single-alias resolver selection; do not implement bulk
update as a TS loop. Exact pins are explicit. Distinguish available/current/pinned/
blocked/check-unavailable states; unavailable is not up to date.

Standalone Skills use the same uap.toml/uap.lock lifecycle. Local authored Skills
remain local. UAP-managed native projections must be protected from legacy local
edit/import/delete on the backend. Offer explicit editable copies with separate
ownership and destination.

### Host context and portability

Hosts provide explicit profiles/roots/executables. Ambient HOME is not an authority
for choosing an app profile. The renderer submits identifiers and intent; trusted
host composition captures and validates context. Navigation does not retarget
accepted work. Context revisions invalidate stale preparation.

A Node caller sends the precise Origin required by the loopback bootstrap contract.
Do not remove existing Host/Origin checks. Tokens do not reach the Electron renderer,
browser storage, command-line arguments, diagnostics, or progress logs.

Browser consumers use their trusted application backend to reach UAP. They do not
launch a binary or obtain arbitrary local filesystem access. HTTP and IPC adapters
must preserve equivalent validation and outcomes.

## Scenario and acceptance matrix — historical v6 evidence

The status names distinguish layers: **engine/API verified** means the cited
deterministic or actual-artifact checks passed; it does not imply final desktop
qualification. **Desktop pending** requires S29/current-app evidence.
**Blocked** requires a separately merged and qualified prerequisite.
Evidence keys E1–E7 are defined immediately below.

| ID | Scenario and required outcome | Current status and evidence |
| --- | --- | --- |
| S01 | Catalog/search/detail; verified distribution and policy identity | Actual signed catalog verified at BBC7: 30 entries, 28 available, freshness current (E3); generated/API gates E1. |
| S02 | Registry plugin install through TS → Go → manifest/lock → native state | Actual reviewed `cloudflare-docs` install and removal passed with native OpenCode JSONC assertions (E3). Desktop catalog flow remains S29. |
| S03 | Standalone Skill discovery, immutable candidate, managed installation | Retained catalog/preview and website byte-freeze tests in [management_skills_test.go](../../cli/internal/agentpluginscli/management_skills_test.go) pass E1. BBC7 Python pinned standalone-Git Skill installation and native-byte proof passed E5; desktop pending. |
| S04 | Local Skill create/edit/import/remove and reversible restore | Go authoring/locking/byte checks E1; actual local create/edit/delete, restart-visible backup and exact restore passed E4. Import/editor UI is part of pending S29, not inferred from SDK calls. |
| S05 | Managed Skill protection and explicit independent local copy | Physical ownership/race tests E1 and actual edited-native refusal/preservation E4. App backend copy/protection regressions and e179's actual local-copy binary-byte check are E6. The complete desktop sequence remains pending. |
| S06 | Add target with retained source pin; isolate captured profile contexts | [Target-only pin preservation](../../install/integrationctl/agentplugins/environment/model_test.go), [registry multi-target identity](../../cli/internal/agentpluginscli/management_registry_test.go), and captured-profile refusal tests pass E1. Separate same-client captures remain the supported model; no multi-profile desktop E2E claimed. |
| S07 | Update one/selected in one candidate graph, retain unselected pins | [Selected graph test](../../install/integrationctl/agentplugins/environment/selected_update_test.go) passes E1; exact-commit update preserves the actual lock in E4. Moving-range and multi-selection desktop interaction are not exercised by the current S29 harness; their cited proof is at the engine layer. |
| S08 | Update all saved moving dependencies, report exact pins/exclusions | Implemented through shared selected-graph preparation and one candidate lock; graph/outdated tests E1. This is not an actual all-clients moving-update E2E claim. |
| S09 | Locked restore into a fresh environment | [Portable fresh restore](../../cli/internal/agentpluginscli/deps_fresh_restore_e2e_test.go) and [mixed-source TOML lifecycle](../../cli/internal/agentpluginscli/deps_toml_cutover_test.go) pass E1. Actual desktop fresh-worktree restore is blocked under S26. |
| S10 | Remove only selected owned bindings, preserve unrelated data/state | Native registry MCP removal E3 and native Skill removal E4 passed; removal/attestation regression coverage E1. Local-copy survival through desktop removal remains S29. |
| S11 | Repair recorded native state while preserving changed/unowned content | Native-projection, typed conflict and rollback tests E1/E2; ordinary repair, edited-byte refusal and fresh recovery after explicit manual preservation passed E4. |
| S12 | Refuse source/alias/authority conflicts before effects | Manifest/graph and [reserved registry authority](../../cli/internal/agentpluginscli/management_registry_test.go) tests pass E1; no borrowing installed eligibility from a future reserved ID. |
| S13 | Lost response/repeated apply observes the same accepted request | Durable replay tests E1; actual lost accepted response, owned-process SIGKILL and same-identity observation passed E4. No automatic mutation retry. |
| S14 | Same request ID with different input conflicts | [Transport identity tests](../../cli/internal/agentpluginscli/management/server_test.go) and SDK structured-conflict tests pass E1. |
| S15 | Restart after acceptance/effect; fresh, proved recovery without blind replay | Deterministic native/local/publication recovery tests E1; BBC7 E4 actually observed an interrupted operation and fresh successful recovery with native-byte/deduplication assertions. |
| S16 | Cancellation before durable effects changes no desired/native state | [Lifecycle cancellation](../../cli/internal/agentpluginscli/management_lifecycle_test.go) and [transport cancellation](../../cli/internal/agentpluginscli/management/server_test.go) tests pass E1; E6 includes the renderer terminal-cancel refresh regression. The current S29 harness does not exercise a real desktop cancellation. |
| S17 | Cancellation after effects remains truthful and recoverable | [Manifest-only failure](../../cli/internal/agentpluginscli/management_recovery_test.go) and [canceled-partial retention](../../cli/internal/agentpluginscli/management/recovery_test.go) tests pass E1; no global rollback guarantee. |
| S18 | Root/profile/manifest or observed-file drift invalidates preparation | Canonical context, local tree/mode and apply-time ownership tests pass E1. Lexical and physical aliases are both checked; unresolved absolute ownership fails closed. |
| S19 | Unaccepted expired plan refuses before acceptance or effects | New `TestUnacceptedExpiredReviewDoesNotConsumeRequestIdentity` passes E1/E2: PLAN_STALE, zero Apply calls, no durable request record, fresh plan can reuse that refused ID. Accepted replay after expiry remains separately passing. |
| S20 | Missing runtime/auth/manual steps do not imply readiness | Availability and readiness tests E1; exact public restart/cleanup instruction tests E2. Success retains distinct materialization/activation/authentication/verification axes; no authenticated model execution claimed. |
| S21 | Catalog unavailable/offline does not erase installed state or mean current | Existing directory/outdated error handling and app independent environment/catalog regressions E1/E6. Actual current signed feed is E3; no complete offline desktop run claimed. |
| S22 | Revoked/suspended/unsupported source retains policy refusal | Signed-directory and recorded-release policy tests E1; new reserved-ID handling preserves bound-state/revocation checks. E3 qualifies one eligible distribution, not every policy branch. |
| S23 | A's late response cannot overwrite B or retarget accepted work | Go stable environment/captured-context tests E1 and deterministic app deferred-response/context regressions E6. Ordinary navigation/reopen in S29 does not qualify an in-flight context switch or multi-project isolation; the current harness does not exercise those cases. |
| S24 | IPC/HTTP share the feature facade, validation and public outcomes | Go protected transport tests E1 and app socket/Host/Origin/CORS parity regressions E6. The e179 partial desktop run proved same-environment HTTP reads, Host/Origin refusal, actual local create/delete and idempotent Apply (E6). This does not make the full S29 run passing. |
| S25 | Qualified native project installation with an explicit project root | **Blocked** on UAP #472/#475/#480 and integration beneath qualified Host ownership (#982); unsupported scope is refused, not counted as installed. |
| S26 | Exact locked environment at actual worktree execution cwd before launch | **Blocked** on the same project/Host prerequisites. Local project Skill authoring and user-scope restore do not qualify native worktree activation. |
| S27 | Active/preparing affected runtime and mutations share admission | App launch/mutation fence, generation and resource-extension regressions E6. Final real active-run desktop proof remains pending; no immutable-run snapshot claim. |
| S28 | Bad executable/version/exit is bounded, owns only its child | SDK actual child/bootstrap/close tests E1 and missing-native-executable tests; matched native artifact started successfully in E3/E4. No implicit latest download. |
| S29 | Actual Electron install/update/remove/reopen, keyboard and narrow layout | **Pending on current app checkpoint.** Earlier real preview/install observations are historical partial evidence; current complete desktop receipt and screenshots remain required (E6). |
| S30 | Pinned SDK/native executable in supported packaged OS/architecture | Linux x64 matched source/tarball/binary staged and executed E3/E4. Historical app signing/package results remain in its plan; current-app package matrix is pending (E7). UAP Terminal UI CI is not Electron package qualification. |

### Historical v6 evidence ledger

**E1 — remote UAP qualification at BBC7.**
[Required run 40176921783](https://github.com/777genius/universal-agent-plugins/actions/runs/40176921783)
passed all six jobs: required tests, four module lint jobs and Management SDK.
The SDK job checks generated contract drift, build/typecheck, protocol/owned-process
tests and a packed consumer. [Terminal UI run 40176921781](https://github.com/777genius/universal-agent-plugins/actions/runs/40176921781)
passed Windows, Linux and macOS. These are results for the named UAP runtime source,
not current-app Full qualification or a public release.
The separate [aggregate CodeQL check on PR #500](https://github.com/777genius/universal-agent-plugins/pull/500/checks)
(`121120190376`) reports **failure with 15 alerts**. The [security review](management-sdk-security-review.md)
traces all 15 locations and the three App annotations, finding no concrete exploit
in the reported paths under the documented authority model. Formal repository
security disposition is still pending; this source review does not clear the gate.
Both language-analysis jobs succeeded; that does not clear the aggregate security
gate. No all-checks or all-security qualification is claimed.

**E2 — focused response and expiry regressions.**
After restoring the exact BBC7 files, Go 1.26.10 passed the five selected tests below
in 3.588s with bounded compiler resources:

- `TestManagementOutcomePreservesPublicManualInstructions`
- `TestManagementManualInstructionsBounded`
- `TestManagementRecoveryPreservesEditedNativeSkillUntilExplicitResolution`
- `TestManagementRepairPreservesEditedSkillBesideRenderedCommand`
- `TestManagementRecoveryPreservesKnownManualReadiness`

The S19 addition was published separately at
[3c3452f48b73363af4871cb178cca03f87ea9523](https://github.com/777genius/universal-agent-plugins/commit/3c3452f48b73363af4871cb178cca03f87ea9523).
Its unaccepted-expiry test and the existing accepted-replay-after-expiry test
passed together locally in 0.047s and are included in BBC7's current CI.
No sleep is used: the retained review expiry is changed under the session mutex.

**E3 — actual signed official registry lifecycle at BBC7.**
The packed SDK consumer observed the current verified directory (30 entries,
28 available), selected the reviewed `cloudflare-docs` `0.4.13-uap.1` distribution
`777genius/cloudflare-docs-bridge`, installed it through actual Go, checked the
authoritative OpenCode JSONC entry and exact remote MCP URL, then removed the
binding and entry while retaining valid JSONC. Install and removal both succeeded.
Local receipt: `uap-official-catalog-bbc7e98da96d-r2-evidence.json`.
This proves native configuration, not an authenticated/model/MCP conversation.
The earlier harness assertion used a client ID where the catalog returns a
captured target ID; correcting that assertion did not change installation policy.

**E4 — actual unchanged native SDK lifecycle at BBC7.**
The first rerun with mutable state under `/tmp/uap-runtime-a0f1bffd2cea` passed,
exit 0 in 116.49s. Source, engine and real OpenCode bytes match the pins above.
Receipt: `uap-native-sdk-bbc7e98da96d-isolated-evidence.json`;
log: `bbc7e98da96d-native-sdk-isolated.log`; run metadata:
`bbc7e98da96d-native-sdk-isolated-run.json`.
All eight scenario checks passed, including exact native Skill/portable-command
discovery, withheld explicit-only command, pinned update, repair, edited-content
preservation and explicit fresh recovery, retained accepted history, removal and
local create/edit/delete/restart-visible restore. Lost accepted response plus
SIGKILL actually produced interrupted operation
`fbda4a6900cb260e510328eae563a6413d446557235df966ff9242f103f844b7`;
fresh recovery `d17c314ca388203daf152c7d886923613f2e5a4ff57d7b278bb8ecd43f4e3979`
succeeded and proved native bytes and no duplicate dependency.
No scenario assertion or production engine code was changed for this rerun.

**E5 — independent Python HTTP lifecycle at BBC7.**
The unchanged standard-library [Python protocol client](../../contracts/management/python_smoke.py)
passed in the isolated temporary root, exit 0 in 23.27s. Client SHA-256:
`c2d85c755ec8c057e313c9ab269d0645fe872c4ef1ee73a18e5204e107ec3940`.
It discovered and previewed a pinned Git standalone Skill, retained its prepared
plan through two catalog discards, applied it, and replayed the same accepted
request. Operation `c5a93f9b13ab4e1c1608d8139f3fff5e2e0fa89e2b8b15941d37b6b88b0a6e0d`
succeeded with one observed entry. Real OpenCode discovered the exact captured
native Skill path; installed bytes matched SHA-256
`07f73f7836f0e929a84bd85943cbac7b26746ee0c4642b4e117b866d681276b0`.
Receipt: `uap-python-skill-bbc7e98da96d-isolated-evidence.json`;
log: `bbc7e98da96d-python-skill-isolated.log`; run metadata:
`bbc7e98da96d-python-skill-isolated-run.json`.
No timeout, scenario assertion or public protocol code changed. The first BBC7
run in synced scratch timed out waiting for an accepted response; its receipt/log
remain failed evidence.

**E6 — Agent Teams implementation and desktop gates.**
Current app source is `e1798241e114fe6b530541e398491a8d14d1323d`, including the
reviewed copy-dialog seed correction and nine passing focused tests. Its remote
[Full CI run 40180747963](https://github.com/777genius/agent-teams-ai/actions/runs/40180747963)
succeeded: Full Qualification (`121132486698`), canonical TypeScript validation
(`121131296750`), all four test shards, three lint jobs and the Windows ledger
passed. These results qualify only that named source checkpoint; subsequent
test-harness changes require their own applicable current-head qualification.
See the [app integration plan](https://github.com/777genius/agent-teams-ai/blob/e1798241e114fe6b530541e398491a8d14d1323d/docs/extensions/uap-management-integration-plan.md)
and [desktop qualification procedure](https://github.com/777genius/agent-teams-ai/blob/e1798241e114fe6b530541e398491a8d14d1323d/docs/extensions/uap-management-e2e.md)
for its owned evidence. The actual e179 partial desktop receipt,
`app-e1798241-desktop/evidence.json`, records native Skill discovery, a 760-pixel
layout without horizontal overflow, source keyboard navigation, same-environment
sidecar reads, rejected Origin/Host, actual HTTP local create/delete and idempotent
Apply, and a managed-to-local copy with exact binary bytes. The run then failed
because the harness clicked Update while that control was still disabled. Its
successful stages remain scoped evidence; the run is not a full S29 pass. The
harness does not exercise moving-range/multi-selection updates, real cancellation,
or an in-flight multi-project context switch. Those behaviors have only their
separately identified engine/renderer proof, not an inferred desktop result.

The separate e179 [remote desktop run 40180747881](https://github.com/777genius/agent-teams-ai/actions/runs/40180747881)
failed with `ENOENT` while reading the copied `assets/sample.bin`; a local
exploratory run reproduced it. The harness could accept the previous install's
Completed state when the copy dialog closed before the new copy operation had
finished. A harness-only completion-correlation correction is being reviewed;
neither this diagnosis nor the earlier local copy-byte success is a full run pass.

The complete current-app Electron lifecycle and platform package matrix remain
pending. At the named ede65 revision, aggregate App CodeQL check
`121128225094` also remained **failure with three annotations**; their source
disposition is in the [security review](management-sdk-security-review.md), with
formal resolution still outstanding. Full Qualification does not replace the
separate platform package or security gates, and a prior screenshot or scoped
check does not complete S29.

**E7 — artifacts and packaged application.**
Agent Teams provenance, the vendored archive and staged `runtime.json` bind the
same BBC7 source, version and digests above. Local artifact handoff is
`bbc7e98da96d-artifact-handoff.json`; SDK package verification is
`sdk-packs/bbc7e98da96dd7d77db42f65da7afae84a86f799/verified-package.json`.
Historical macOS ad-hoc signing and platform package smoke are retained in the app
plan. Current app package/signing checks must pass before claiming S30 complete;
no release-certificate, notarization or public-release result is inferred.

The named local receipts/logs are retained in the task's `build-evidence` handoff;
they are distinct from committed executable tests and linked remote CI.

### Qualification environment correction and retained failures

Earlier BBC7 SDK runs used mutable fixtures under synced `/workspace/scratch`.
One observed interrupted acceptance and then an empty owned Skill directory; a
filtered filesystem trace on another fresh fixture failed earlier, before SIGKILL:
the harness successfully renamed the edited live guide into its preservation
directory, but the old live path reappeared with a different inode and the same
edited bytes. No traced process recreated that path. The echo Skill still matched
its recorded native digest. Background scratch synchronization has a writable
receiver for that tree; it is the leading external-resurrection mechanism.

This is an environment diagnosis, not an engine publication/WAL fix. OpenCode's
publisher already copies and verifies a private tree before exclusive rename.
Ownership refused the externally changed live bytes correctly. The original
failed fixtures, receipts and logs are retained, including
`uap-native-sdk-bbc7e98da96d-r2-evidence.json` and
`bbc7e98da96d-native-sdk-strace-analysis.json`. Trace SHA-256:
`9fb9d1114bd318473bec5fa2557748dda309727fa44c23545fd5f15aa5844248`.
Mutable native/UI/protocol fixtures and live logs belong in an isolated temporary
directory outside synchronized scratch; copy completed immutable evidence back
after process exit. E4 and E5 then passed unchanged. Never weaken content guards or
discard a failed receipt to make qualification green.

### Unmerged project/Host prerequisites

Fresh status check for v6: all four PRs remain **open drafts**. They have not been
imported into this integration.

| Prerequisite | Observed head |
| --- | --- |
| [UAP #472](https://github.com/777genius/universal-agent-plugins/pull/472) | `a0d1b5f30f890c36925caecdda5f59725245b167` |
| [UAP #475](https://github.com/777genius/universal-agent-plugins/pull/475) | `e630a5b0ac1cd94443cf710f93a79acb8274ac38` |
| [UAP #480](https://github.com/777genius/universal-agent-plugins/pull/480) | `2c767ea0a2eb3e3e2640afcff0fbab5cfa31973b` |
| [Agent Teams Host #982](https://github.com/777genius/agent-teams-ai/pull/982) | `3b351f32474111bb9bd7943d1e6b5a06a3da5e84` |

S25/S26 require the relevant adapter behavior and Host ownership to be merged and
qualified together. User-scope success, local project authoring and explicit
unsupported-scope refusal do not satisfy those scenarios.

## Agent Teams implementation and UX

Use src/features/extension-management with only layers that own real behavior.
Keep native EnrichedPlugin and external installations intact. Preserve existing
tab order and pinned project context. Reuse search, Radix primitives, Markdown,
status and diff presentation. Add a small managed-entry view model/hook rather than
rewriting the store or extending frozen oversized extensionsSlice/SkillsPanel.

Show current scope and actual agent runtime near the action. Capability-based
disabled actions include their reason. Prepare as part of showing details; ordinary
install uses one clear action. Review meaningful dependency/permission/target
changes, without a mandatory wizard for every install. Recovery/partial outcomes
must remain accessible after navigation and restart. Put receipts/digests in
expandable diagnostics.

Record a narrow new ADR extending remote managed Skills while preserving the
accepted separate Skills domain and local authoring. Existing draft
docs/extensions/plugin-kit-ai-integration-plan.md supplies product context, but its
obsolete engine/CLI assumptions are superseded.

Host migration #982 remains a dependency to recheck. If merged, wire UAP below its
captured context and mutation owner; do not bypass it with a second writer. Do not
cherry-pick the whole draft migration for this task.

## Contract decisions after independent review

### Immutable context and profile isolation

A management process/session binds one immutable trusted-host context: manifest/
lock root, state/managed roots, scope and explicit client profile/executable mapping.
A second profile for the same client is a separate captured environment unless the
engine explicitly supports multiple profile bindings. Do not add a speculative
profile dimension to the client-ID-keyed dependency graph. Existing physical-root
ownership checks remain authoritative; conflicting roots are refused.

### Acceptance ordering, storage and retention

Serialize acceptance lookup/create with an existing environment/process lock.
Session mutexes alone are insufficient across two UAP processes. Lookup a known
accepted request before resolving its old plan handle or enforcing plan expiry.

Namespace request IDs by the stable Go mutation environment identity, not a hash of mutable host capture details. Keep request ID, immutable
context/input fingerprint, accepted plan ID, correlated installer operation IDs,
disposition and final public result/next actions. Store no credentials, operational
prepared objects, filesystem transaction copies or progress ticks. Persist before
effects; a persistence failure refuses acceptance. It is an observation record,
not an automatically replayed job queue.

Retention policy: keep terminal observations for at least 30 days; bound individual
record/result size and history. Never prune unresolved/recovery-required records by
display-history limits. Expired/unknown observations return a structured not-found/
expired result and require environment inspection; they never imply no effects.
A request ID is never silently recycled within retained acceptance history.
Document exact limits in the contract and test them.

### Bulk resolution policy

Manifest commit, exact SemVer and archive-digest pins remain unchanged with a pinned explanation. A signed registry release selector retains the existing safe release-update policy; its saved sequence is not an immutable user pin. Eligible moving
selections resolve together into one candidate graph; failed acquisition, dependency
conflict or validation refuses the candidate before acceptance. Excluded pins and
unsupported/policy-blocked entries appear with stable reasons in review. An unknown
update check is not current. No largest-installable-subset solver and no silent
best-effort graph loop. Partial results refer to effects of an accepted plan.
An exact lock revision does not mean manifest intent itself is pinned.

### Reproducible artifacts without public release

Build the SDK tarball from an exact reviewed UAP source commit. Vendor that tarball
in Agent Teams with package identity, source SHA and SHA-256, following the existing
vendor/terminal-platform/sdk precedent. Pin the same source commit for native UAP.
Agent Teams build/CI builds that source with its declared Go toolchain and
read-only module resolution, then stages the executable into packaged resources.
End users receive a built executable; no runtime Go build or latest download.

Use the same pins for PR E2E and clean-checkout/package-consumer qualification.
Include license and third-party notices. Do not depend on an expiring Actions URL
or an unpublished registry version. A later move to public release artifacts uses
the separately authorized release process. Refresh source and package pins together
before final qualification.

### SDK lifetime, compatibility and qualification

wait() abort/deadline never kills the child or cancels accepted work. UI navigation
does not close the engine. Node close() is idempotent, asks its own process to shut
down gracefully, waits a bounded interval, and only then terminates that child if
needed. Interrupted accepted work stays discoverable for recovery. No unrelated
native process is killed. Test startup exit, bootstrap timeout, repeat close and
close during accepted work.

Protocol major is independent of package/engine version. Reject major mismatch.
Optional response fields are additive; unknown outcomes/actions are conservative,
never success. New request fields need supported capability/version. Map
materialization, activation, authentication, policy and verification without
flattening them into an installed boolean; include a manual-action response fixture.

CI runs pinned generation with --check. Explicit incoming request validation remains
required. Test the packed TS package from a clean consumer against real Go. Check
pure exports for accidental Node/Electron/DOM-storage dependencies. Add one small
Python standard-library protocol smoke against the real server (handshake, read,
fixture lifecycle and structured error), not a speculative maintained Python SDK.

### Application HTTP admission and live profiles

S24 covers only the new Agent Teams feature, as required by its feature standard;
it does not rebuild every legacy web extension API. Both adapters invoke the same
facade. Existing application CORS settings are not mutation authority. Every
managed sidecar method requires the separate unforgeable sidecar capability
before facade invocation, in addition to exact Host/Origin, loopback peer and
bounded JSON checks. The benign unauthenticated availability response cannot
invoke the facade. Missing/forged credentials and hostile headers are refused.

Only the owned Electron main frame can obtain an authorized browser link through
IPC; HTTP cannot grant itself authority. The browser removes the fragment and
retains authority per tab and exact origin, including port. New grants refresh an
existing client. Stop, restart and failed startup revoke the old capability.
Bug reports omit fragments. The separate Go bearer remains in trusted main
composition and never reaches the renderer. These are feature-local boundaries,
not a new global authentication framework. A hostile process already able to read
private browser/main memory or the same private state remains outside this claim.


Current shared native profile files can be read by active agents. Therefore refuse/
defer affected-context mutations while a run uses those roots, with an actionable
stop/defer message; unaffected contexts are allowed. No false immutable-run promise.

### Completion and dependency gates

API/SDK can be independently mergeable when their advertised capabilities pass.
Full Agent Teams project/worktree completion still requires S25/S26 against a
merged, qualified project engine. Capability refusal tests are fallback evidence,
not a passing project installation. Never merge unfinished prerequisite adapters
or mark dependent scenarios passed to satisfy a checklist.

## Historical delivery sequence and review checkpoints

M0–M5 record the order and review gates used to deliver this work. They are not a
new implementation backlog. Current support and remaining work are governed by
the v6 scenario matrix and evidence ledger, not by historical milestone wording.

M0 Research and plan:
- Commit/push v1 and open draft PRs in UAP and Agent Teams.
- Independent architecture/lifecycle and frontend/UX critics.
- Address each finding explicitly, commit/push v2 before production implementation.

M1 Contract and smallest vertical lifecycle:
- Shared preparation/application seam and versioned headless HTTP startup.
- Generated Go/TS DTOs; pure SDK + Node helper.
- Before the first mutation: durable acceptance/result correlation, request fingerprint,
  installer operation IDs, cross-process acceptance serialization, interrupted state.
- Real fixture install, operation wait, combined lost-response + kill + restart retry.
- Acceptance lookup precedes expired/missing plan lookup; no automatic replay.
- Independent contract/code review before broadening surface.

M2 Complete management lifecycle:
- Catalog/Skills/environment, selected/all update, locked sync, repair/remove.
- Extend M1's acceptance correlation, cancellation and recovery to every lifecycle.
- Shared CLI/picker behavior, generated drift checks and second-language recipe.

M3 Agent Teams host integration:
- Captured context, pinned binary preparation, SDK adapter, IPC/preload/HTTP.
- Ownership guard in legacy Skills mutation path.
- Actual execution-directory seam and explicit capability gating.

M4 Frontend:
- Reused shell/components, managed entries in Plugins and Skills.
- Small normal install flow, update selected/all, partial/recovery UX.
- Independent UX critique and actual Electron clicking/screenshots.

M5 Qualification and delivery:
- UAP focused lifecycle/contract/CLI parity + required core/lint/generation gates.
- TS package typecheck/build/package-consumer test.
- Agent Teams typecheck/lint/build, architectural guards and focused behavior tests.
- Dedicated real Electron management E2E on test-only homes/projects and native
  discovery proof; packaged binary qualification where required.
- Independent final code review; resolve findings and rerun affected checks.
- Current-commit required CI, including Agent Teams Full qualification.
- Refresh prerequisite PR status, exact diff/reviews/checks; merge only qualified
  changes. No main bypass, forced push, fabricated pass, or unsupported feature claim.

Every commit is immediately pushed. Plans and evidence live in repository docs,
not only chat. New review findings are recorded in the next plan revision with
disposition and verification.

## Test and evidence strategy

Use deterministic local source/release fixtures for content versions and failure
barriers. Fixtures may provide input packages and isolated agent profiles, never
fake a successful SDK result or prewrite the installation being asserted. Test
public boundaries and filesystem/native observations; avoid mirrored unit tests.

Real Electron uses pnpm dev:mcp, its own discovered CDP port, existing CdpClient,
test-only userData/HOME/XDG/projects, screenshots, console errors, and actual
native configuration checks. No native folder picker, no user projects, no web-dev
substitute. A small native-agent handshake complements materialization tests.

Existing CI does not automatically click the new UI. Add a focused management E2E
entrypoint and workflow path triggers. GitHub Full qualification remains mandatory.

The original full-disk environment and temporary in-memory checkout are historical.
Current runtime fixtures must avoid the synchronized scratch tree as documented
above. Coordinate bounded heavy jobs in the shared environment; a stopped or
resource-constrained full check is not a passing check. Record exact source pins,
terminal outcomes and actual native observations before claiming qualification.

## Current implementation decisions and regression boundaries

| Decision | Implemented authority / evidence |
| --- | --- |
| Shared application seam | CLI-internal typed preparation and existing installer/coordinator; [management backend](../../cli/internal/agentpluginscli/management_backend.go), not a new public Go management facade or second installer |
| Contract and process | [OpenAPI source](../../contracts/management/openapi.yaml), generated DTOs, one loopback HTTP transport, pure TS client and separate owned-process Node entrypoint |
| Accepted identity / recovery | Durable observation records plus original installer receipts/journals; retained typed plans are never serialized executable jobs |
| Bulk updates / portable restore | Existing selected resolver policy, one graph/lock; selected graph and fresh-restore tests cited in S07–S09 |
| General local ownership | Canonical CLI state plus shared locks; Go local authoring with physical-path checks, dirswap and exact retained undo |
| Profile authority | Immutable captured host context; stable durable environment identity remains visible across context changes |
| Runtime/project ownership | Existing Agent Teams mutation/launch admission; native project/worktree behavior remains blocked on the table above |
| Artifacts and language portability | Same-source vendored TS package/native build; independent Python uses the same public HTTP contract without another maintained SDK |

The published follow-ups preserve these concrete boundaries:

- **Physical paths and modes:** lexical containment and physical canonical
  comparisons both apply to persisted CLI ownership, including aliases with a
  missing leaf. An existing dangling/unresolvable absolute ownership path fails
  closed. Non-path native selectors do not become filesystem ownership. Newly
  desired Windows modes match what that filesystem reports; observed read-only
  modes remain intact and mode drift invalidates a local review.
- **Registry identity:** a reserved ID for an unbound future install is not proof
  that an installed signed release exists. Multi-target preparation shares its
  reserved installation correctly; missing existing bound authority remains stale
  and another release cannot borrow old installation eligibility.
- **OpenCode configuration selection:** Skills-only effects do not own a config
  choice that OpenCode itself may populate as schema-only JSONC. Actual new,
  replacement or final-removal MCP effects still check the recorded config
  selector. The regression covers all four cases and preserves unrelated config.
- **Native content and readiness:** complete recovery checks every owned Skill and
  MCP projection. Typed changed-content refusal crosses the public boundary only
  with proven effect classification; failed rollback remains uncertain.
  Materialization and successful installation do not imply authentication.
- **Public follow-up instructions:** adapter-authored `ActivationOutcome.UserActions`
  already become `installer.Result.ManualActions`. Management now retains each
  valid public instruction as a bound `NextAction{kind: manual}`, including exact
  restart and separate cleanup advice, while preserving successful outcome state.
  Validation permits at most 32 instructions per native binding result and 4,096
  Unicode code points per instruction; invalid input uses safe generic guidance,
  never a truncated command. Private native reasons, errors and `LocalActions`
  remain filtered. No client-name heuristics, adapter rewrite or new schema was
  introduced.
- **Expired unaccepted reviews:** S19 proves refusal before backend Apply or durable
  acceptance, without consuming the request identity; known accepted replay is
  resolved before expiry and remains unchanged.

These corrections are covered by the current E1 gates and focused E2 evidence.
They do not expand unsupported project capabilities or authorize changes to the
frozen VS Code implementation.

## Anti-overengineering budget

Keep one wire contract, one transport, one Go engine, one mutation per environment,
polling, existing journal/receipt authority, and one maintained language SDK.
Do not add a daemon, scheduler, dual transport, generic remote callbacks, global
atomic rollback, universal OAuth/configuration framework, five unused SDKs, a new
installer, or a complete frontend rewrite.

Original planning budget: roughly 3,500–6,500 new handwritten production LOC across
both repos, separate generated schema/types and focused tests. This is a planning
range, not a target; prioritize shared behavior and delete unnecessary wrappers.
Any material expansion requires a recorded reason and critic review.

The reviewed checkpoint exceeds that original combined estimate. An exact GitHub
comparison of UAP BBC7 against main `a6beceba`, and Agent Teams e179 against main
`eb67de9`, excludes unrelated main merges and records:

| Handwritten runtime/build scope | Files | Added lines | Removed lines |
| --- | --- | --- | --- |
| UAP Go/SDK production and build tools | 90 | 6,027 | 331 |
| Agent Teams runtime/feature code | 54 | 4,072 | 182 |
| Combined | 144 | 10,099 | 513 |

The combined net change is 9,586 lines; the maintained TS SDK itself accounts for
339 handwritten lines at this checkpoint. These named-checkpoint counts exclude
tests, generated files, schemas/configuration, localization, documentation and
the vendored SDK archive; they are not an estimate of future revisions.

Independent review accepted the additional scope because durable acceptance and
reconciliation, Go-only local Skill editing with retained undo, and shared host
mutation/launch authority implement concrete required behaviors. The result still
has one native engine, one wire contract and one maintained language SDK, with no
daemon, second installer or new frontend framework. This records the expansion
and its rationale rather than treating the original planning range as achieved.

## Historical review log

These entries preserve the evidence and decisions available at each revision.
Their counts, candidate pins and then-open gates are historical; v6 status is the
matrix and evidence ledger above. In particular, v4/v5 are retained rather than
retroactively relabeling old tests as current-artifact qualification.

- v1: source/primary-tool research incorporated; three read-only research reports
  identified HTTP parity, native identity preservation, backend Skill ownership,
  retained recovery authority, manifest publication effects, and durable request
  correlation. Independent review of this written plan is the next checkpoint.

### v2 critic dispositions

- Lifecycle B1/M1: moved durable correlation and cross-process acceptance into M1;
  specified retry lookup before missing/expired plan checks.
- Lifecycle M2: immutable context, separate same-client profile environments.
- Lifecycle M3: explicit batch pin/refusal policy; no best-effort subset solver.
- Transport R1 / lifecycle M4: reproducible vendored SDK + same-source native build.
- Transport R2/R3/R4/R5: combined restart retry, shutdown semantics, actual Python
  smoke, packed-consumer/generated drift qualification.
- Transport R6: new-feature IPC/HTTP parity only, mandated by app standard.
- Lifecycle M5/M6: dependency completion gate and limited observation persistence.
- UX: enforce active-profile mutation refusal and strict management HTTP admission.
- Compatibility/result terminology: conservative unknown handling and readiness axes.

No critic requested another transport, installer, scheduler or frontend rewrite.
Mandatory lifecycle/transport findings known at v2 were incorporated;
implementation review remains a separate gate.

### v3 final UX critic dispositions

- F1/F2/F4/F5 are covered by v2 active-profile admission, narrow HTTP authority,
  exact-source artifact delivery and separate foundation/full-integration gates.
- F3: ownership has three states: managed, proven-unmanaged, unknown. An engine
  failure is unknown, never proof of local ownership. Compare canonical physical
  destination identities, including symlink/alias handling. Revalidate at local
  edit/import/delete apply and serialize app-managed UAP changes. Before claiming
  external-CLI race protection, inspect current projection layout and environment
  locks and implement the narrow shared ownership/reservation seam if needed.
  This is an implementation gate, not permission to add a general filesystem lease
  service. Explicit copies use a distinct proven-unmanaged destination and existing
  validation. Test unavailable engine, alias/symlink, concurrent same-destination
  install, import overwrite, backend delete and successful copy.
- F6: Plugins includes Install from source for engine-advertised source forms.
  Skills includes source URL -> candidate preview/selection -> scope/runtime ->
  install. Keep local import/authoring separate. Reuse one small source dialog when
  semantics match; expose unsupported/private-auth limitations from capabilities,
  never arbitrary CLI flags or executable paths. Real Electron E2E covers this path.
- F7: desktop UX qualification includes keyboard-only action/result flow and a
  narrow window. Assert accessible labels/focus and visible usable scope, disabled
  reasons, partial outcomes and recovery controls through existing CDP.
- Active-profile admission must coordinate launch and mutation at the actual effect
  boundary; two disconnected activity checks alone do not close a start/update race.
  Reuse the narrow runtime ownership boundary; no run snapshot framework.

Plan direction is approved for implementation subject to these explicit gates.

### v4 implementation evidence and critic dispositions (historical)

- Separate stable `environmentId` (canonical state root + existing user environment
  + scope) from immutable `contextId` (dependency root, captured targets, profile
  roots and executables). Changing a binary or a manifest location must not hide
  unresolved accepted work. Each observation retains its original context and
  dependency root; recovery cannot settle a different root by inspecting an empty
  replacement. Recovery proof can settle all compatible native pending records in
  one reviewed operation, without rewriting their original outcomes.
- Agent Teams uses the canonical UAP CLI state root and its environment/native
  mutation locks. An app-private receipt store cannot prove that a default-CLI
  managed Skill is locally owned. Trusted embedders may supply explicit isolated
  roots, but this does not establish custody across arbitrary unregistered stores.
- Local Skill authoring uses the same Go retained-plan/acceptance lifecycle and
  existing `dirswap.Manager` under the shared installer locks. The host captures
  allowed roots on stdin; HTTP accepts root IDs, relative names and expected file
  digests. No JavaScript check-then-write lease. Authoring in an explicit project
  Skills directory does not enable native project-scope plugin installation.
- Local changes retain reversible bytes in private engine storage outside native
  Skills discovery. Recovery references the exact prior local operation/root/name;
  generic native recovery never settles uncertain local file effects. Backups and
  journals are evidence for a fresh review, not executable serialized plans.
- Cancelled operations with mutations or unresolved next actions remain pending.
  The Go admission boundary also refuses new ordinary work while durable pending
  work exists; an SDK caller cannot bypass the app's runtime fence after restart.
- Graph updates preserve the existing minimum-compatible SemVer selection policy.
  Re-evaluating a range is not a promise to install the numerically greatest version.
  Selected ranges resolve together while unselected bindings keep exact locks.
- The app preserves an uncertain apply request ID across panel remounts. Definitive
  stale-plan rejection is interpreted only after durable replay lookup. Network,
  observation and acceptance persistence failures retain the fence and identity.
- Verified so far: generated SDK build/typecheck and 21 protocol/owned-process
  tests; application retained lifecycle/recovery/Skills tests; 12 transport tests
  for durable replay, locks, cancellation, malformed history, category-specific
  reconciliation, strict HTTP admission and stale/discarded reviews. The actual Go
  binary builds. Actual SDK/Python installation, Electron interaction, packaging,
  full repository gates and final independent review remain separate open gates.
- Rechecked prerequisite PRs #472 and Agent Teams #982: still open drafts at the
  original pinned heads. Project/worktree scenarios S25/S26 remain dependent on
  the separate qualified adapter workstream. Do not reinterpret a disabled scope
  control as completed project installation.

The implementation retains one HTTP contract, one native engine and one language
SDK. No daemon, generic file lease, parallel installer or public release was added.

### v5 final implementation review and remaining qualification (historical)

Independent final review found four blocking issues; its minimal corrections are
accepted without adding another installer, scheduler, persistence layer or protocol:

- **Stale admission observation (P1):** Agent Teams records the shared admission
  generation before inspecting an environment. An older empty response cannot
  release a fence acquired by a later accepted mutation. Deferred-response tests
  must complete acceptance before delivering that stale response.
- **External removal attestation (P1):** the authoritative native adapter declares
  whether an actual prepared removal requires explicit external-uninstall
  confirmation. Installer plans expose that fact; management returns an existing
  non-confirmable review with `EXTERNAL_UNINSTALL_REQUIRED` for the exact bindings.
  Missing confirmation must refuse Check/Apply before acceptance and before desired
  TOML/lock publication. No management client-ID policy table is introduced.
  An explicit fresh review may carry `externalUninstalled: true` only when its
  actual native actions include a requiring removal. This includes transitive
  pruning during update/sync, not only a top-level remove. Such a review identifies
  its exact attested bindings with `EXTERNAL_UNINSTALL_ATTESTED`. A receipt-only
  recovery or a graph with no applicable removal refuses the flag with
  `EXTERNAL_UNINSTALL_NOT_APPLICABLE`. Full fresh convergence can therefore finish
  an older accepted partial removal without bypassing the pending-operation gate.
  Confirmation is never an automatically stored preference or carried to a
  different review. Existing CLI removal semantics stay unchanged.
- **Discovery lifetime (P2):** expose idempotent `skills.discard` through the SDK
  and both app transports. Release replaced/closed discovery catalogs. Prepared
  plans retain their own source snapshots and remain independent of disposal.
- **Definitive local-delete refusal (P2):** preserve the same request while its
  acceptance is uncertain or its effects need recovery. A definitive pre-effect
  refusal, or an observed terminal no-effect result, releases the local deletion
  handle so the next explicit Delete can prepare a fresh review.

Additional reviewed corrections:

- Native recovery requires fresh verification of the owned native projection,
  including every Skill and MCP entry. A private receipt/artifact alone cannot
  prove that a missing native file was restored. Existing installer authority and
  environment/native locks remain the source of this proof.
- Edited owned Skill directories are preserved. The typed changed-content outcome
  reports unchanged effects only after successful rollback; a failed rollback
  remains uncertain and keeps recovery data. A wholly missing directory can be
  restored from the retained pin. Manual native readiness stays distinct from
  materialization and authentication.
- A website candidate's first preview digest is immutable for that discovery
  session. Changed publisher content requires rediscovery and another review;
  concurrent previews cannot silently replace the bytes a user already reviewed.
- Code signing changes executable bytes. macOS packaging updates the UAP digest
  after nested signing and immediately before the outer app seal. Windows updates
  it after signed resource copying. Final checks are read-only. Reuse
  electron-builder's existing signer, options, retry policy and packaging hooks.

Evidence is recorded against the revision actually tested:

- UAP `08a9218c0ecf9bc44e951c3921f20abd9d3b275b`: Required test lane, generated
  artifacts, workspace vet, conformance, SDK generation/build/protocol tests and
  the packed consumer passed remotely. Installer and CLI lint passed; four root
  fixture style findings were subsequently corrected.
- UAP `7fb4695002310f72a4ed43193ddd82ab733b2b3b`: SDK generation, protocol and
  packed-consumer gates passed. Final native/test and lint qualification still
  belongs to the subsequent corrected commit.
- Actual matching 08a SDK/native tests passed local Skill create/edit/delete,
  binary-byte preservation, stale-hash refusal, restart, visible undo and restore.
  Actual Python HTTP tests acquired and installed plugin and standalone-Skill
  fixtures and verified native OpenCode files. These results do not qualify later
  native-recovery changes without rerunning the affected smoke.
- Agent Teams `a213b80e8a4a97b613ffbe35df9f42f4ed7b090e` passed Fast feedback,
  including the full pinned TypeScript check. Actual Electron has reached the
  managed panel and HTTPS Skill preview. Complete installation/recovery/keyboard/
  narrow-window UX and final signed packaging are still open at this revision.
- Rechecked #472/#475/#480 and Agent Teams #982: all remain open drafts at the
  recorded heads. S25/S26 remain dependency-blocked; this revision does not claim
  qualified native project installation.

Next checkpoint: finish the scoped corrections, independently review their exact
diff, refresh the SDK/native source pin together, run the real Electron lifecycle,
and record final S01–S30 evidence. Current-commit required CI is still a merge
gate. The macOS test uses an explicitly unsigned native fixture followed by real
ad-hoc signing; it is not a claim of release-certificate notarization.
