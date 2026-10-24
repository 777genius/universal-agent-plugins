# Project installation completion plan

Owner continuation: 2026-10-24. The owner authorizes practical spikes, independent
critique and fixes, parallel implementation for all supported clients, independent
code review, incremental commits/pushes, and merge after verification. Publishing
a release is not part of this continuation.

Status: **draft for independent review; no new production implementation yet**.
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
client authority. Apply checks again before cache publication, intent publication
and native mutation under the correct environment lock. No process-global cwd.

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
Exercise status/update/repair/remove in blocked and unblocked states, including
operator activation flag rejection and project A/B/user precedence.

## D. Parallel client lanes

| Lane | Work and acceptance boundary |
| --- | --- |
| Codex | Preserve .agents/skills + .codex/config.toml route; latest observed native regression and exact existing pinned matrix. Validate selected config and MCP startup with local harmless server where possible. No catalog-cache ownership claim. |
| Cursor, Gemini, Copilot, Kiro, Kimi, Devin | Reconcile implemented adapters; fix only demonstrated shared/client defects; verify complete prepared lifecycle and exact version/harness admission. Preserve specific trust/manual/session guidance. |
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
   and separate Copilot/VS Code project MCP authority. Distinguish fixtures from
   native sessions and native configuration from MCP handshake/tool execution.
6. Update user docs, help/capability matrix and truthful source availability with
   exact remaining native boundaries. Do not relabel historic binaries as supporting
   a feature merged later. Record actual commands/results and source SHA.
7. Refresh main, integrate any relevant movement, rerun affected gates on the final
   assembled SHA, then merge the reviewed PR using expected-head protection. Do not
   bypass failing required checks, unresolved reviews or missing implementation
   evidence. Close/supersede included feature PRs truthfully. Report merged SHA,
   verification results and native/product limitations.
