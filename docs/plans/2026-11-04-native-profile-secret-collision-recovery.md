# UAP native lifecycle qualification: Claude profiles, Codex MCP, Skill collisions, crash recovery

**Date:** 2026-11-04. **Base:** `87b769a8f546b0bb1c25c4c34422f3f756995697`. **Status:** draft pre-code plan; research/spikes first. **PR:** one bounded PR, no release.

## Goal and acceptance

Close the remaining **proved**, supported-route gaps in [#641](https://github.com/777genius/universal-agent-plugins/issues/641), [#642](https://github.com/777genius/universal-agent-plugins/issues/642), [#629](https://github.com/777genius/universal-agent-plugins/issues/629), and CLI-level post-write crash evidence **without weakening safety**. A verified unsupported/native limitation must remain an honest BLOCKED/UNSUPPORTED outcome, not a fabricated product PASS. Keep unrelated PRs, fixes and release policy unchanged.

## Constraints and existing ownership

- Read `AGENTS.md`, accepted [ADR 0005](../adr/0005-standard-first-agent-plugins-installer.md), [support](../SUPPORT.md), [project matrix](../PROJECT_CLIENT_SUPPORT.md), and `install/integrationctl/agentplugins/installer/README.md` before changes. **Do not edit VS Code** implementation/tests/fixtures. Do not modify legacy `plugin.yaml` authoring, portable Agent Plugins 1.0 interpretation, signed Registry or landing.
- Reuse Go `installer` + `transaction` journal, `nativeconfig` ownership kernel, selected `clients/claude`/`clients/codex` adapters, and existing CLI/Management error path. No second manager, secrets database, owner DB, daemon, feature flags, universal template substitution, new policy DSL or blanket test harness.
- Scope is a single repository PR with **separate reviewable commits** (plan/spikes, independently justified fixes, E2E). Every commit is pushed immediately; leave draft until **all** exact-HEAD gates pass. A green evidence-collection job alone is not a feature PASS.
- Source/target labels distinguish published UAP **0.1.95** (historical probes) from current source. The existing native project CI pins Claude **2.1.263**, Codex **0.153.4** (macOS ARM64 and Windows AMD64); the historical Linux UAP/APM comparison used Codex **0.164.0**. C1/M1 must name the exact binary/version/platform and cannot extrapolate across versions or from the four-OS **portable** catalog. Keep those lanes separate; never use real profiles or credentials.

## Verified baseline and unproved hypotheses

| Lane | Established, do not reimplement | Specific missing proof |
| --- | --- | --- |
| Claude #641 | Historical real A→B→A `skills add`: A succeeds, B refuses indeterminate, A replay unchanged; no A overwrite. Native list parser requires exact `id/scope/enabled/installPath`. | Which B-created files changed after refusal and whether native list is malformed, sibling-visible, or genuine conflict; safe B install, selected B remove/recover. |
| Codex #642 | `nativeconfig.CodecCodex`, project `.codex/config.toml` owner receipts; existing real `TestAgentpluginsCodexNativeTrustedProjectMCP` does direct `initialize/tools/list/tools/call`. | Need/authority for **explicit Codex-only stdio variable-name forwarding** and proof of actual MCP child receiving a random synthetic canary without persistence. |
| Skills #629 | Existing `FOREIGN_DESTINATION` guidance for `nativeconfig.ErrCollision/ErrNotOwned` in PR #678, foreign byte preservation in #617 and later cross-target tests. | Exact source-level reason of released `PLAN_REJECTED` for a **Skill directory**; whether generic, untyped project `requireAbsent` error is lost before picker, and whether current main already fixes it. |
| Crash | PR #654 process exits at *durable post-activation/pre-receipt* fault and recovered kernel reinstalls correctly; `native_recovery` and Management restart coverage exist. | Uncovered **post-state-commit / journal-finalization** transition and real CLI restart/recovery receipt/status; no proof of arbitrary power loss. |

Sources: #633/#634 review protocol, #646 four initial gates, #654, #678, #679, #700, #624 and the published `uap-apm-risk-*` artifacts. Native config docs: Codex `mcp_servers.<id>.env_vars`; Agent Plugins 1.0 §9.2 expands **only** `${PLUGIN_ROOT}`/`${PLUGIN_DATA}`, preserving other placeholders literally.

## Spike-first decision gates (run with disposable HOME, XDG, UAP state and native roots)

1. **C1 / Claude** — independently read sanitized `claude plugin list --json` under A and B, at four separately bounded checkpoints (pre-any-command, post-native-only initialization, post-UAP attempt, post-removal/recovery); record exact selected root, parsed ID/scope/path, exit code and **all changes** to existing/new/deleted files by type, content hash, identity and safe metadata. Include A, B and shared disposable HOME/XDG; never traverse symlinks. Attribute each change to CLI-only initialization, UAP or unknown (do not infer from 5→8 file-count change). Compare A/B physical paths and per-profile `AGENTPLUGINS_HOME`, distinguish **Claude CLI initialization** from UAP effects. Expected result: safe supported cause identified or explicit unknown/blocker. No code fix until this passes. Explicit negative controls: B refused with no receipt; same native plugin ID but different physical roots; modified/linked path after Prepare; malformed/duplicate/contradictory list; edited owned bytes before remove. Confirm cleanup/removal only in B if B acquires an owned receipt; never authorize deletion from A's saved receipt. User-scope Claude list has no documented `--scope` filter; classify its returned fields, not an invented selector.
2. **D1 / foreign Skill** — reproduce the pinned project Scope Codex foreign `.agents/skills/benchmark-note/SKILL.md` with real released UAP and current source in separate roots. Trace the **typed** refusal from `ProjectComponentMutation.requireAbsent` through `PrepareSkills`/review controller; compare actual CLI and picker/JSON outputs. If conflict confirmed but untyped, add **one narrow domain Skill-occupied sentinel** (do not reuse an MCP-specific error whose message is misleading), map it once in the existing CLI/Management error boundary. Preserve generic refusal for unrelated errors; never forward untrusted provider strings. No `--force`, foreign adoption/deletion or public schema break.
3. **M1 / Codex** — reuse the actual existing Codex native direct-MCP RPC fixture. Prove project-local `env_vars = ["UAP_TEST_CANARY"]` is read by pinned trusted Codex and a controlled inert stdio child. **Matched negative control:** same custom canary in Codex parent but *no* `env_vars` must not reach child; selected forwarding must reach only the intended server, never an unrelated server. Avoid direct fixture launcher inheritance and literal config values. Scan explicitly enumerated bounded non-symlink roots and artifacts for raw/base64/hex canary; any unreadable/oversized/truncated observation is inconclusive, not leak-free; record boolean/digest only. **No product feature unless a concrete supported UAP opt-in host caller exists and the trial passes.** If justified, pass strictly validated env **names only** through an **explicit host-consented** target-qualified request/Management seam to `nativeconfig.CodecCodex`, not portable `mcp.json`; bind consent to selected server, package digest, profile and scope. Reject reserved variable aliases, duplicate names and collisions with authored literal `env`; never authorize through untrusted package extension alone, and never opt in an unrelated target or remote executor. No value persistence, unknown-token interpolation, or unsupported nonlocal env source. Reuse existing portable-token regressions; if the opt-in projector changes these paths, assert the **native child** sees unknown `${TOKEN}` literally, single-pass substitution and no portable `env_vars` acceptance. A variable absent **when Codex later launches** is a native runtime-readiness problem: report the required environment setup as Prepared/Blocked without claiming launch-time presence from an install-time check. Never synthesize or persist a value.
4. **R1 / crash** — inventory exact existing `dirswap` fault phases and current four-platform catalog coverage before adding tests. Use a real separate OS process deterministically exiting **after** durable state commit but before journal/backup finalization if not covered. Reopen entirely from disk and invoke existing `Recover` at the **open-journal PhaseCommitPending** boundary; for a **durably committed state receipt**, require retained **desired** owned bytes, finalized committed receipt and journal/backup cleanup (not restoration of old bytes). Uncertain/changed physical authority must refuse. Assert source/foreign hashes, no false native activation, and idempotent clean retry. Add only a **real CLI/Management restart** observation if an existing test-only barrier can prove the phase without a production testing hook. Otherwise keep this sub-gate BLOCKED, never time-based `sleep; kill` pseudo-proof.

### Acceptance for each lane

- **Claude:** A→B→A with truthful native identity, both scopes/paths bound, no foreign or sibling changes; B-only remove + reopen if supported; a refusal remains fail-closed with actionable reason and unchanged foreign content. Capture negative malformed/symlink/duplicate-list tests.
- **Codex:** existing direct-MCP initialize/tool-call kept; explicit name forwarding only with proven consumer need, invalid/missing names negative, selected profile and source pin unchanged, reinstall/remove preserve unrelated entries; raw and common-encoded canary absent from files/artifacts; no model/OAuth call.
- **Collision:** observed cause is exact foreign destination; a targeted *typed* human explanation instead of generic `PLAN_REJECTED`; preserve existing noncollision codes/JSON compatibility, byte identity and negative symlink/race cases.
- **Crash:** true post-write child death (not mock error), full re-open, state/receipt/journal and native neighbor assertions, recovery idempotence; no automatic unknown-state rollback or false `Active`.

## Review checkpoints — two rounds **before code**, repeated post-code

- **Critic round 1 (contract/security):** challenge spec conformance, profile authority, secret environment inheritance, path/symlink/hardlink and cross-OS cleanup, and whether test claims exceed observations. Reject unsafe or nonrepresentable cases.
- **Critic round 2 (architecture/E2E):** independently challenge duplicates against #654/#678/#624/#700, native fixture provenance, missing negative controls, PR size, use of new layers, and assumptions that `Prepared` implies loaded. Resolve each actionable issue in the plan before authorizing production edits.
- After each implementation wave, review actual **current diff**, run focused and cross-platform tests, request CodeRabbit/Strix scrutiny and recheck every finding against source. Fix valid findings; record reason for nonapplicable ones. **Do not claim an external agent review until one actually appears.**

## Implementation ordering and scope budget

1. Commit this plan and pre-code review dispositions on one draft PR; request outside critique.
2. Execute C1/D1/M1/R1 with existing scripts/CI/fixtures; append only concise reproducible evidence and status (PASS/FAIL/BLOCKED/UNSUPPORTED/NOT_RUN).
3. Implement only verified corrections to existing Claude identity, Skill conflict projection or Codex-specific selection; **skip speculative code**. Extend existing tests rather than copy the comparison framework. Keep production changes small and independently justified.
4. Repeat native/disposable and four-platform E2E on **final PR HEAD**. Minimum: `go test ./...`, `make lint`, `make test-install-compat`, portable `scripts/package-manager-e2e.py`, native Claude/Codex project lanes where relevant, Required/CodeQL and independent feedback. The full suite may run on hosted CI where runtime is unavailable locally; preserve proof by exact SHA.
5. Resolve review comments and rerun changed-head checks until no blocking findings. Squash merge to `main` only if every affected feature is proven or explicitly left out as a documented unsupported scope, checked HEAD/base are stable, and **all required** jobs pass; verify post-merge state. Never publish a release implicitly.

### Explicit non-goals

No blanket multi-client rollback, arbitrary power-loss guarantee, vendor login/model calls, new UI flows, changes to published support claims without real proof, new cross-provider secret format, modifications to open unrelated PRs, or changes solely to make a benchmark green.

### Stop/re-scope rule

If a spike reveals that the desired feature requires a new manifest/state schema, adopting foreign files, cross-profile ownership inference or a new standalone framework, **do not implement it in this PR**: retain the existing refusal and report the precise blocker. A known safe refusal is preferable to a false successful install.

## Two adversarial pre-code passes (author review; **not independent agent verdicts**)

**Pass 1 — standard/security, 2026-11-04.** Challenged: (a) the historical R2 artifact only proves that B's *count* changed (5→8), not which files are unsafe; a Claude CLI probe may initialize its own profile. **Correction:** require independent name+digest snapshots before/after *each* vendor probe and UAP write, never infer an overwrite from file-count delta; (b) Codex reads `env_vars` at runtime, not when UAP installs. **Correction:** install-time presence cannot certify future environment; require an explicit user-owned name declaration and a real launched child oracle, with Prepared/Blocked when unavailable; (c) namespace overlap must not be solved by `--force`, path normalization alone, or copying receipts from another profile. **Correction:** preserve exact physical owner proof and refuse unknown identity; no new cross-profile state. (d) a text `PLAN_REJECTED` does not prove a Skill destination collision. **Correction:** demand the raw typed cause in a real supported scenario first.

**Pass 2 — architecture/E2E, 2026-11-04.** Challenged: (a) wrapping Skill collision in `nativeconfig.ErrCollision` would expose an MCP-specific error message and conflate separate ownership domains. **Correction:** prefer one domain-owned Skill collision sentinel and existing CLI/Management guidance, only after source proof; (b) proposing another recovery implementation or duplicate Codex MCP process fixture would violate DRY. **Correction:** extend `process_crash_recovery_test.go` and `TestAgentpluginsCodexNativeTrustedProjectMCP` only for genuinely missing, deterministic subcases; (c) a `go test` PASS with no real client is not native E2E. **Correction:** hosted pinned source/binary+fixture SHA and subprocess trace are mandatory for native PASS; (d) introducing a new E2E workflow would cost too much. **Correction:** wire missing focused tests into existing native client and package-manager pipelines. Any unattainable deterministic barrier remains BLOCKED rather than prompting a production fault-injection flag.

**External review status at publication:** CodeRabbit skipped automatic draft review; manual `@coderabbitai full review` was requested. Strix reported a subscription/billing blocker, so no independent Strix verdict exists. Neither bot's silence is treated as approval. These two passes are adversarial author reviews; external plan/code critique and final exact-HEAD CI remain open gates.

## External plan critic — CodeRabbit round 1, 2026-11-04

[Independent standards/security critique](https://github.com/777genius/universal-agent-plugins/pull/704#issuecomment-6630374690) read the original plan and repository source; **no native binaries were run**. All six substantive findings were accepted, with bounded corrections above:
1. Full pre-native, post-native, post-UAP and post-recovery A/B/HOME snapshots rather than only new B filenames.
2. Explicit host consent bound to one Codex server/package/profile, reserved-name/duplicate/collision checks, and install-time vs runtime env distinction; no remote env proxy.
3. Verify literal portability at the child only if a new projection touches it; preserve existing conformance tests and avoid duplicating every unknown-token permutation.
4. A matched canary-absent negative control and incomplete-scan refusal; claim absence only within observed roots.
5. Claude no-receipt, path-change, duplicate-list and modified-owned-content refusal cases; do not invent `claude plugin list --scope`.
6. Successful durable state commit must preserve desired bytes, while unknown authority refuses and does not claim native Active.

**Disposition:** no production code authorized solely by this review. The reviewer explicitly did not run native E2E. A second independent architecture/E2E round is requested against this revised plan before implementation.

## External plan critic — CodeRabbit round 2, 2026-11-04

[Independent architecture/E2E critique](https://github.com/777genius/universal-agent-plugins/pull/704#issuecomment-6630510087) inspected the corrected plan, code and existing test matrix; **no native binaries were run**. All three high-value objections accepted:

1. **Correct version/platform admission:** current native client matrix is only macOS ARM64/Windows AMD64 and Codex 0.153.4, while the separate historical Linux comparison had Codex 0.164.0. Do not count portable Ubuntu/Windows/macOS Intel/ARM runs as native Codex/Claude process proof. No new version provisioning framework.
2. **Explicit crash phases + CI gate:** the existing four-OS crash selector only admitted the post-activation test. Include the new post-state-commit test in the same strict `go test -json` run/pass/no-skip CI step. The journal-already-removed but receipt-still-state_committed boundary has no qualified current deterministic process barrier; it is **NOT_RUN/BLOCKED**, not silently covered.
3. **Truthful per-assertion status:** preservation can PASS as a negative invariant while successful B install remains BLOCKED; do not infer `UNSUPPORTED` from arbitrary exit=1/timeouts, nor count a missing feature consumer as secret forwarding PASS.

The reviewer also confirmed the `requireAbsent` Skill-specific untyped error was real at the reviewed base and that one domain sentinel is a small viable change. However, the real preparation→public refusal path still requires E2E evidence; the mapping unit test alone is insufficient.

### Admission and crash phase matrix

| Asserted observation | Proven barrier/state | Expected output | Test admission |
| --- | --- | --- | --- |
| Post-activation crash before state receipt | OS child exits at `PhaseActivated`, old state, open journal | Restore original bytes; no false receipt; retry succeeds | `TestDirectoryProcessCrashAfterActivationRecovers` in four-OS Package manager E2E |
| Post-commit crash with open journal | OS child exits at `PhaseCommitPending` after durable `state_committed` receipt | Retain desired bytes, finish committed receipt and delete backup/journal; repeat recovery is no-op | `TestDirectoryProcessCrashAfterStateCommitRecovers` in same four-OS step |
| Journal already removed, state still `state_committed` | No deterministic existing exit seam identified | Exact durable desired-state reconciliation, no fabricated Active | **NOT_RUN** as process-crash proof until a safe existing barrier is found |
| CLI/Management restart after accepted process crash | Not proved by the above kernel tests | Read-only status plus safe explicit recovery | **NOT_RUN**; not covered by orderly #700 restart |
| Native agent runtime activation | Not proved by filesystem recovery | Controlled actual native client tool/session observation | Separate client-version/OS cell |

Status rules, **per test assertion**: `PASS` only when the specific oracle actually ran and passed; `FAIL` on a supported correct invocation with observed invariant violation; `BLOCKED` when prerequisites/cause/observer/barrier were insufficient; `UNSUPPORTED` only with an explicit pinned contract showing unsupported behavior; `NOT_RUN` when not executed. Missing identity, link or cleanup evidence is inconclusive, never a successful protection oracle.
