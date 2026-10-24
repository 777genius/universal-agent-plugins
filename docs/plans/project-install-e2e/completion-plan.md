# Project installation completion plan

Owner continuation: 2026-10-24. The owner authorizes practical spikes, independent
critique and fixes, parallel implementation for all supported clients, independent
code review, incremental commits/pushes, and merge after verification. Publishing
a release is not part of this continuation.

Status: **R1 critique incorporated; awaiting two independent R2 decisions before new production implementation**.
Baseline assembly: `0cf52a6067b03bc36d3bb1f45ac2c6356f95ae52`.
Fresh main: `4555aa53` (includes source transports and the standalone Skills UI).

Read AGENTS.md, PROJECT_INSTALL_OWNER_SCOPE.md and
AUTHORING_CAPABILITY_PRESERVATION.md. Existing source, contracts, native evidence,
merged source and released availability remain separate claims. Use GPT-6.1
high/xhigh workers and critics. Preserve the narrow VS Code exception and useful
legacy source/tests. The client plans and previous reviews remain background;
this document governs the new completion changes.

## Observed spikes and design choice

1. Pinned Claude Code 2.1.263 emits an untrusted-project sentinel from
   `plugin list --json`: `(suppressed)@skills-dir`, scope project, enabled false,
   empty installPath, version unknown and explanatory notes. Exit status is zero.
   Existing strict parsing rejects the sentinel. Nested cwd does not scan the
   parent whole plugin. Native uninstall refuses skills-directory plugins and
   directs directory removal. Project disable writes project settings and can
   affect a user-origin row in that project. No trust was granted or model task
   requested. Raw observations must be preserved with the implementation.
2. Official Codex 0.164.0 exposes project skills and trusted/untrusted config via
   app-server read-only calls. A same-name user skill and project skill coexist
   in its list. Repo marketplace inventory is available through app-server, but
   the CLI plugin add/list path does not provide equivalent project scope.
   Two trusted disposable projects using the same marketplace/plugin/version
   can load the first project's body from shared user plugin cache. Therefore
   retain project component projection; do not replace it with native catalog
   installation or rename portable publisher identity to hide cache coupling.
3. A disposable Vercel Skills local-lock probe confirmed relative project source
   portability and content hashing; a corrupt lock reads as empty. Adopt the
   portable intent concept, but preserve UAP's explicit corrupt-authority refusal.
   APM and Skillshare support separating source intent, native target scope and
   exact owned entries. Do not adopt silent global fallback, process cwd mutation,
   unverified target overwrite, or generic full-file ownership.

These observations establish the tested boundary, not authentication, trust
approval, model execution or support on untested versions/platforms.

## Architecture and invariants

Keep one dependency/environment orchestration layer. Reuse the existing resolver,
manifest/lock formats, immutable source store, security validation, transaction
engine, scoped receipts, namespace guard and recovery. Keep client-specific native
paths, codecs, commands, precedence and activation in the client adapters.
`ProjectComponentMutation` shares exact file/entry ownership where contracts match;
do not rewrite specialized proven transactions solely for visual uniformity.
The existing neutral Codex TOML port and reviewed concrete adapter stay intact.

- Default scope remains user. Project scope is explicit; omitted project-root
  selects the invocation directory, with canonical existing-root validation.
- Resolve immutable TargetContext before source acquisition. A native client user
  profile remains user authority; production never changes HOME/CODEX_HOME to
  simulate project installation or uses package path as scope authority.
- Intent and source pins are portable. Project receipts, locks, journals and data
  under .uap are local authority, bound to selected directory identity. Shared
  immutable package bytes do not imply shared installed ownership.
- Every read, preview, prepared action, apply, update, repair, remove and recover
  uses the same scope, environment ID, project root and original user profile.
- Preserve foreign data, comments, disabled entries and changed managed objects.
  Refuse ambiguous ownership, symlink escape, replaced roots and copied receipts.
- Installed files, prepared configuration, observed native discovery and successful
  MCP operation are distinct. Attestation cannot override negative native evidence.
  Unknown native output remains unknown; do not weaken parsing to achieve green CI.
- No source acquisition or mutable project state for an unsupported explicit target.
  No unsupported-to-user fallback. No credential, account, trust or policy mutation
  in production or in a probe of an actual user profile.

## A. Reconcile the existing work

Create an isolated continuation branch. Merge fresh main, the current project-core
head and reviewed client leaves, retaining both source/Skills additions and project
scope behavior. Review conflicts semantically; retain existing tests and claims.
Include Claude diagnostics and Codex recovery notes; integrate the reconstructed
Grok implementation and reviewed research collectors. Do not merge unrelated open
PRs. Record incorporated source SHAs and any genuinely new integration decisions.

Run the existing core suite before/after integration. The first local baseline
attempt hit host ENOSPC before compilation; that is not a code test failure.
Build dependencies and temporary products can use an isolated temporary filesystem.
Never delete another task's files to make room. Remote CI remains authoritative for
macOS and Windows behavior.

## B. Standalone Skills and UI scope completion

Compose the selected App at Skills command entry before Git/HTTPS acquisition,
including website Skills. Use the existing prepared batch and dependency installer;
do not introduce another install engine. Carry app.scope()/environmentID() through
Coordinator Resolve/Observe/PrepareSync/ApplySync and adapter options. Preflight
explicit target scope and project namespace before reading source or taking locks.

Picker target capabilities and displayed destination must use selected project
layout, not detected user ConfigRoot. An unavailable project target remains disabled
with a precise reason. Terminal and browser use the same preparation/apply API.
Stale-plan checks bind namespace, selected scope, exact inputs, target paths and
client authority. The original scoped App/installer namespace is checked before opening the
project environment lock, and again after acquiring it before cache, intent or
native writes. Rechecking only a disposable preview is insufficient. Rejected
replacement roots must not gain a lock file. Direct StartSkillsPicker/PrepareSkills
entrypoints enforce the same selected-scope/target/namespace admission as CLI entry.
No process-global cwd.

Tests: public CLI Git and website Skills; prepared browser callback route; dry-run
has no durable mutation; rejected target acquires no source; user/A/B isolation;
changed root/receipt/client authority between prepare and apply; locked replay,
update, missing same-pin repair, removal and retained data. Use existing fixtures,
with one representative source-by-client seam rather than a full Cartesian suite.

## C. Claude native observation and owned inactive lifecycle

Keep the projected skills-directory backend and original manifest name. Introduce
project-only typed observation: exact enabled, exact disabled, known suppression,
conflicting winner/shadowed, absent, unknown. Preserve strict user parser behavior.
Parse rows order-independently. Recognize the observed sentinel narrowly, with
exact id/scope/false enabled/empty path/unknown version and bounded nonempty notes.
Contradictory fields or wrong types remain unknown; unknown suppression reasons do
not become a trust claim. The sentinel establishes no package identity.

### Pre-delivery admission is separate from native activation

The project-only RegistryInspector maps known suppression, a disabled exact row
and an out-of-project competing winner to RegistryClear **only for native namespace
ownership**. It does not claim the target is absent or native discovery reconciled.
Existing InspectPreparedRegistry scans the selected project namespace, and generic
receipt/digest/path/namespace validation must independently gate every mutation.
No generic RegistryFinding, combinedIdentityState or recovery change is planned.

| First-install or maintenance situation | Admission |
| --- | --- |
| Known suppression; exact destination absent; project namespace and prepared scan clear | Admit prepared first install; suppression supplies no package identity or ownership |
| Known suppression; exact destination or same-name project artifact exists without matching ownership | Collision/refusal; no adoption |
| Known suppression; matching owned binding and validated directory/digest | Admit the existing owned maintenance path; keep native result prepared |
| Exact disabled project row without matching UAP ownership | Collision/refusal through prepared scan; disabled does not mean absent |
| Same-name user winner; exact project target absent and project scan clear | Admit a prepared shadowed project copy; preserve the user row and settings |
| Same-name user winner; matching owned project target | Permit validated owned maintenance/removal; no native-discovery reconciliation |
| Same-name foreign path inside selected project | Collision, even if native listing is suppressed or disabled |
| Native record claims another project or contradicts selected context | Indeterminate/refusal, never selected-project authority |
| Unknown/malformed listing, target drift, symlink escape or replaced namespace | Existing uncertainty/refusal/recovery path; no adoption or activation |

Decode all rows strictly before selecting a result. Duplicate sentinels, duplicate
expected identities, wrong types and contradictory selected-project rows yield
Unknown. A sentinel plus an exact selected-project record is contradictory and
cannot promote activation. A sentinel plus a user winner is valid blocked evidence;
suppression and shadowing may both be reported. Unrelated valid rows do not change
selected identity. Two matching user/project candidates are ambiguous unless a
future pinned native schema explicitly establishes a winner. A foreign same-name
selected-project path always refuses ownership independently of native ordering.

Filesystem receipt/namespace/path/digest verification independently establishes
owned delivery. After exact owned delivery, known suppressed/disabled/shadowed
native state is Prepared+Installed with actionable next step, never Active.
Only an exact enabled selected-project record confirms native discovery; no MCP
operation claim follows. Unknown or malformed output retains honest uncertainty
and pending recovery as the existing transaction contract requires.

Removal consumes exact owned directory authority even when inactive, suppressed,
shadowed or absent; no native plugin uninstall, force-disable, user settings edit
or trust mutation. Preserve active foreign path and symlink protections. Repair
must not acquire ownership of a same-name foreign target. Tests model user and
project candidates together rather than scanning only the selected fixture root.
Exercise the initial untrusted public add as well as owned update/repair/remove,
unowned exact targets under suppression, malformed mixed rows and both user/project
precedence orders. Assert NativeDiscoveryReconciled remains false for recognized
blocked observations. Preserve operator activation flag rejection and user behavior.

### Saved status, repeated operations and native evidence

Engine.Inspect remains host-free: plain status reports saved Prepared/Installed
and generic next action, not a freshly observed trust/shadowing reason. Precise
blocker text belongs to fresh activation/update/repair outcomes. Runner-count regressions must prove ordinary inspect/status launches no native process. No new persisted
blocker field or hidden native status command is included. If an intact repair
reconstructs its result from a binding, preserve fresh transient guidance narrowly.
Do not globally declare Prepared converged. Verify repeated blocked add/sync and
repair retain exact bytes and honest mutation/no-change classification.

Inactive owned removal uses the ordinary verified transactional deletion path;
the adapter only admits it. A previously Active binding with missing/contradictory
native evidence retains its conservative refusal until a supported confirmed
operation records the blocked state. Do not infer new ownership from old activation.

The genuine untrusted native project fixture must exercise prepared add, repeated
install, update, missing-directory repair and removal using its public owned binding
path. It must verify the native suppression boundary and unchanged user/A/B files.
It must **not** mark native_skill_discovery or native_mcp_initialization as passed
using filesystem reads. Those stages remain blocked/not evaluated. Keep existing
user native runtime requirements unchanged. Trusted project discovery is a distinct
qualification with actual observed output, not a rewritten label or a trust grant
performed to make this gate pass. The harness/evidence validator must encode both
contracts explicitly; a successful prepared journey does not certify loaded plugins.

## D. Parallel client lanes

| Lane | Work and acceptance boundary |
| --- | --- |
| Codex | Preserve .agents/skills + .codex/config.toml route; latest observed native regression and exact existing pinned matrix. Record selected config and MCP startup separately as attempted/passed/blocked/not evaluated; use a local harmless server only through a permitted native observation surface. No catalog-cache ownership claim. |
| Cursor, Gemini, Copilot, Kiro, Kimi, Windsurf / Devin (`windsurf`) | Reconcile implemented adapters; fix only demonstrated shared/client defects; verify complete prepared lifecycle and exact version/harness admission. Preserve specific trust/manual/session guidance. |
| OpenCode | Reuse existing root config and project components; add genuine project discovery/configuration/lifecycle to existing pinned native harness. Do not mix native v1 and v2 schemas. |
| Grok | Integrate existing prepared skills-only leaf; independent final reviews, lint/platform lifecycle checks. MCP remains explicitly unsupported unless a new separately reviewed spike establishes it. |
| Qoder | Execute already twice-reviewed populated user/A/B research at pinned1.1.78, inspect output independently; then review a concrete adapter plan before coding. Use real scoped install/list/uninstall semantics, exact scope/source/path/version/enablement identity and retained-data behavior. Empty list evidence is insufficient. |
| Cline | Execute reviewed pinned3.0.73 collector, independently interpret effective config/skills output. Then review and implement native project skills plus explicit session-MCP preparation at .uap/clients/cline. Show launch action; ordinary startup and operator attestation cannot make session config durably active. |
| VS Code | Implement and review already twice-reviewed pinned1.143.1 help/version collector; obtain publisher evidence, then review a narrow workspace skills/.vscode/mcp.json observation and adapter plan. Preserve user backend; project backend independent of Copilot. No unrelated VS Code work. |
| ChatGPT | Preserve explicit pre-acquisition project unsupported, including --no-save. A native feature absent from ChatGPT must not be invented. |

Phase gates are real: a successful help/version collector does not approve an
unobserved parser or establish native activation. Where the native client cannot
provide a capability, expose the supported prepared/unsupported result precisely.
A client still lacking a required observed installation contract is an outstanding
completion item, not silently counted as fully implemented.

## E. Review, integration and merge

1. Obtain two independent plan-review/fix rounds for this completion plan. For each
   evidence-dependent lane, obtain two rounds on the concrete post-spike design
   before product code. Preserve findings, resolution and reviewed source identities.
2. Implement shared/CLI, Claude, and remaining native/component groups in isolated
   branches/worktrees concurrently. Commit and push small coherent increments.
   Do not force-push over other work. Integrate reviewed changes with original
   authorship/history and preserve per-client PR traceability.
3. Two independent implementation reviews examine scope leakage, ownership,
   precedence, interruption/recovery and native evidence interpretation. Fix all
   blocking findings; re-review changed logic, not just the earlier revision.
4. Run meaningful exact-head gates: core, lint, vet, generated contracts,
   cross-platform builds, three-platform project contracts, affected terminal
   flows, source/Skills lifecycle, all-client composed admission/maintenance and
   pinned native matrix. Extend evidence selection without dropping required cases.
5. Test supported user+A+B outcomes through production composition: add, unchanged
   replay, explicit update, exact-pin repair, status, interruption/recover/remove;
   retain foreign artifacts and plugin data. Verify cross-client shared skill paths
   and separate Copilot/VS Code project MCP authority. Include one actual process-
   death/reopen case after a skill-directory swap and before MCP commit/acknowledgment:
   unresolved native attempt must fence blind retry/adoption and preserve the bytes
   needed for recovery or honest refusal. Reuse existing attempt/journal machinery.
   Distinguish fixtures from native sessions and native configuration from MCP
   handshake/tool execution.
6. Update user docs, help/capability matrix and truthful source availability with
   exact remaining native boundaries. Per client, record implementation contract,
   fixture lifecycle, observed native discovery, observed MCP initialization,
   trust/account/session restrictions and exact tested version/platform. A missing
   installation contract remains an outstanding item. Cover every canonical
   SupportedClientIDs() entry, including ChatGPT unsupported-before-acquisition.
   Do not relabel historic binaries as supporting
   a feature merged later. Record actual commands/results and source SHA.
7. Refresh main, integrate any relevant movement, rerun affected gates on the final
   assembled SHA, then merge the reviewed PR using expected-head protection. Do not
   bypass failing required checks, unresolved reviews or missing implementation
   evidence. Close/supersede included feature PRs truthfully. Report merged SHA,
   verification results and native/product limitations.
