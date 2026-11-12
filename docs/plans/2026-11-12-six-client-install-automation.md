# Six-client installation automation — one PR

Date: 2026-11-12. Baseline: `071026c96fe0e937b6e8716037b1b08473e4a5db`.
Status: design and capability qualification; implementation is not yet accepted.

## Owner decision, recorded verbatim in substance

One Draft PR contains the plan, reviewed corrections, implementation, E2E and
merge. Commit and push each coherent change immediately. Do not split this work
into separate client PRs. Do not publish a CLI release as part of this request.

**VS Code is excluded completely:** no adapter, protocol, configuration,
dedicated fixture/test, dependency qualification or UI behavior changes.
**ChatGPT Web is excluded completely.** Neither target is included in new
commands, client actions, native runs or capability/badge promotion. Do not
silently expand this into ChatGPT Desktop/Codex, Kimi or another client.

The six targets are Cline, Grok Build, Google Antigravity, Windsurf/Devin, Kiro
and Hermes. Existing PR #686 owns Kiro authentication research; #877 owns Kimi
MCP. Do not edit those branches or relabel their evidence. Merged #810/#869
already provide Grok preparation and no-model MCP proof; reuse that work.

## Outcome, not a badge change

After ordinary UAP installation, automate the remaining safe, documented
client-specific steps instead of asking users to copy environment variables,
edit configuration or manually repeat an action UAP can perform and verify.
Keep four distinct facts: installed files, native discovery, enabled runtime,
and provider authentication. `Prepared` is an honest result, not a failure to
hide. A configured entry or successful list command is not a tool-call proof.

No model request, purchased subscription or provider key is necessary for the
baseline test plan. A vendor login requirement must be recorded separately;
never invent a session, bypass authentication or copy a real user's credentials.

## Stage 0 — prove the native action before product changes

For each target, capture a pinned official version, hash/identity and exact
help/list/config behavior in disposable User, Project A and Project B roots.
Review current source/docs AND an actual executable. A documented command
name does not establish its scope, output shape or ownership safety.

| Target | Existing implementation | Concrete remaining work | Admission prerequisite |
| --- | --- | --- | --- |
| Cline | User native Skills/MCP; Project Skills plus owned `.uap/clients/cline/mcp.json` | Remove manual per-process environment assembly for CLI; select the exact project MCP source and verify it in a fresh process. Keep IDE route separate. | Real published CLI supports the chosen settings override; whole-file override and user precedence are explicitly preserved. |
| Grok | User lifecycle; Project plugin Skills/MCP; #869 real doctor/list and direct installed-config calls | Explicitly consented, exact-owned plugin enablement and read-only verification, without changing workspace trust. | Native enable action must resolve the exact installed project path, not a same-name User plugin; untrusted Project remains untrusted. |
| Antigravity | Owned User/Project bundle; strict validation and isolated CLI registration evidence | Register/enable through the genuine `agy` interface when identity and scope are provable; verify native inventory. | Determine whether local install duplicates the bundle or changes profile-global state. CLI registration is not IDE activation. |
| Windsurf/Devin | Distinct Devin User/Project native layouts plus legacy Cascade route | Verify the selected Devin inventory after automatic delivery; eliminate obsolete manual import guidance only on that observed route. | Exact executable/profile/version and native list contract. Legacy Windsurf must not inherit Devin claims or new state. |
| Kiro | Automatic configuration/Skill mutation; runtime currently authentication-gated | Finish credential-free file/selection verification and use a genuine read-only observer only when already authorized; retain precise next action otherwise. | No broad custom-agent rewrite, auth bypass or duplicated #686 native-auth investigation. |
| Hermes | User Skills/MCP; trusted Project Skills; Project MCP refused | Prove whether an official selected-profile/session override can safely isolate Project MCP. Implement only that supported route, retaining default User authority. | No write to User YAML for Project; no fake claim that ordinary `hermes` auto-loads a managed isolated profile. No copied tokens, history, model settings or second config engine. |

For an unqualified native action, capture the exact limitation in this same PR
and keep its status incomplete. Do not substitute a fixture executable for the
vendor binary or merge a false capability claim.

## Stage 1 — narrow architecture and consent

Read AGENTS.md, docs/ARCHITECTURE.md and CONTRIBUTING.md first. Reuse existing
client registry, plan/refinement, lifecycle, command runner, native-config
kernel, ownership receipts, locks, transactions and rollback. Operational
client logic stays in `clients/<id>`; generic providers must not branch on IDs.

1. Prefer an existing lifecycle or verifier interface. Add a narrow optional
   interface only if an actual remaining action cannot be represented by one.
   Do not introduce a framework, manager service, service locator, second
   installer/resolver/state store, feature flag or duplicate truth matrix.
2. If Cline/Hermes require a new process, expose an explicit launch operation;
   do not spawn an interactive client on ordinary install or modify the parent
   shell. Reuse existing execution/cancellation and exact target selection.
   A launcher is not advertised as native automatic discovery on plain startup.
3. Plugin enablement is a separate execution permission. Show exact client,
   scope, project, owned package and effect; obtain action-specific consent.
   Ordinary install `--yes`, `--activation-complete` and `--auth-complete` must
   not be reinterpreted as consent to run arbitrary code or grant trust.
4. Unknown executable/version, ambiguous same-name plugins, stale receipt,
   unowned selected path, shadowed configuration, changed plan or unsupported
   action fail before mutation. Revalidate under the existing lock.
5. Never auto-grant workspace trust, change enterprise policy, overwrite user
   disablement, invoke a model prompt or launch every configured MCP server as
   a verification shortcut. Probe only the reviewed owned target when a native
   selective interface exists; otherwise do not claim selective verification.
6. If a native command writes config beyond the owned leaf, determine whether
   the existing kernel can record and roll back that exact effect. No adoption
   of a whole foreign configuration file; no broad `force` flags.
7. Persist only existing supported receipt/status facts. No JSON, lock, wire or
   data migration. Installation remains resumable when a later native phase
   cannot run. Cancellation must not leave a false successful status.

## Stage 2 — incremental implementation in this branch

A. Cline selected-process route and tests, including User/Project isolation.
B. Grok precise consented enable/verify route, reusing #869's protocol evidence.
C. Antigravity exact scope/identity registration and enablement, then real list.
D. Devin-specific observation; preserve legacy Windsurf and mixed installations.
E. Kiro safe automatic delivery/observer path without borrowing auth proof.
F. Hermes isolated Project MCP only after Stage 0 establishes a real native
   route; otherwise retain the hard refusal and document the external gap.

Each coherent commit includes its regression tests and is pushed immediately.
Keep the head stable while genuine native CI runs; fix actual failures in a
consolidated patch rather than restarting full matrices for cosmetic edits.
No separate PRs, releases, registry signing changes or public promotion.

## Stage 3 — tests and independent oracles

For every changed client, test User and Project A with an independent Project B
and foreign-user sentinel. Install through the actual public CLI, not a second
config writer. Reuse pinned local stdio/HTTP probes and existing native helpers.

Required scenarios:
- dry-run and declined/absent consent: no client, config, receipt or process effect;
- first/repeated add, update, missing-package repair, cold locked restore, remove;
- unchanged repeat identity, source pin and data receipt; retained data on remove;
- exact-owned enable success and native read-back; same-name foreign target,
  global/project ambiguity, unknown version, preexisting disabled entry refusal;
- untrusted and unauthenticated boundaries remain unmodified and accurately reported;
- partial user edits are refused, not overwritten to repair a test;
- timeout, child failure and cancellation preserve ownership and recoverability;
- new-process scope selection does not alter parent env or original user settings;
- no paid service or real secret; logs never retain auth material;
- genuine initialize/list/call where the selected native interface allows it;
  an independent installed-config call is explicitly distinguished from native call.

Capture raw bounded output, actual command/exit, nonce/session correlations,
source SHA/tree, native versions/hashes, test counts, effect and isolation
assertions. Verify original artifact digest/CRC and actual checkout. A skipped,
queued, cancelled, mocked or research-only run is not native acceptance.

Reuse existing affected E2E lanes; add a path-scoped lane only for a genuinely
new behavior. Cover Linux/macOS/Windows where an affected native host is
available, preserving unsupported-platform labels rather than inventing success.
For Go changes run make lint, make test-core, relevant rollback/install/polyglot
checks and applicable full Required/security workflows. Do not widen exemptions.

## Stage 4 — review, integration and completion

Perform at least three design reviews before implementation: (1) native API and
scope feasibility, (2) architecture/minimality, (3) consent/ownership and negative
E2E. Record findings and concrete corrections below; these are review passes,
not claims of independent reviewers. Request independent PR review as available.

Before merge, verify every affected file against the allowlist and both excluded
client boundaries; no unrelated catalog/schema/release changes. Resolve actual
review findings, inspect exact-head CI and original native evidence, perform a
SHA-guarded normal merge and verify postmerge native/shared checks. If any of
the six required changes remains unimplemented, do not label the whole task
complete or reinterpret a limitation as successful automatic support.

## Initial sizing (estimates, not a commitment)

| Work | Changed/new production lines | Test/E2E lines | Complexity / confidence |
| --- | ---: | ---: | --- |
| Cline | 120–240 | 220–420 | 3/10 / 6/10 |
| Grok | 100–220 | 180–350 | 4/10 / 6/10 |
| Antigravity | 140–300 | 200–380 | 4/10 / 5/10 |
| Devin / legacy preservation | 150–350 | 250–500 | 5/10 / 6/10 |
| Kiro | 120–300 | 220–450 | 6/10 / 5/10 |
| Hermes Project MCP | 350–900+ | 400–850+ | 8/10 / 3/10 |

These estimates exclude unsupported vendor work, purchases and credential
provisioning. Re-estimate after native feasibility review; do not compensate for
an unavailable safe interface with an oversized wrapper or silent global writes.

## Evidence and review log

- Baseline inspected: main 071026c; existing Grok #810/#869 are already merged.
- Overlap checked: #686 remains the separate Kiro research/auth task; #877 is
  Kimi-only and outside this request. VS Code frozen instructions remain intact.
- Official current references: [Grok commands](https://docs.x.ai/build/cli/reference),
  [Antigravity marketplace](https://antigravity.google/docs/marketplace),
  [Cline CLI source](https://github.com/cline/cline/blob/main/docs/cli/cli-reference.mdx),
  [Hermes MCP](https://hermes-agent.nousresearch.com/docs/user-guide/features/mcp),
  [Kiro commands](https://kiro.dev/docs/cli/reference/cli-commands/).
- R1 native feasibility: pending exact-source and executable checks.
- R2 architecture/minimality: pending.
- R3 authority/negative-E2E: pending.
