# Bounded follow-up: #629 native CLI refusal and post-crash CLI/Management observation

**Date:** 2026-11-04. **Base:** `71565b1c2e6422f5841fe38b1db1f3c3fe9ceed3`. **Status:** draft plan; no implementation or E2E pass inferred. **PR:** one follow-up draft PR; #642 excluded from production changes.

## Goal and non-goals

1. Close [#629](https://github.com/777genius/universal-agent-plugins/issues/629) only after proving the pre-fix failure with the genuine **released** UAP 0.1.95 CLI and a **detected genuine Codex binary**, then verifying the fixed **current-source-built** CLI against the same occupied *foreign* project Skill. Compare exit codes/typed error/bytes and validate a helpful **safe project-relative** destination path. The source build is not a released binary: do not claim that a fix has shipped to npm.
2. Extend existing crash/recovery qualification to a **separate-process CLI/Management** restart after a deterministic accepted process death, if there is a safe existing phase barrier and an existing public recovery/status command. Reuse `dirswap`, `statev2` and tests, not a second engine. Existing real child `PhaseActivated` and `PhaseCommitPending` recovery on four OS is established by PR #704; do not duplicate those or claim arbitrary power-loss proof.
3. Reconcile existing PR #704 post-merge `main` workflow results separately. No new workflow, additional repository PR or unrelated CI/release change required just to monitor it.
4. Keep [#642](https://github.com/777genius/universal-agent-plugins/issues/642) **OPEN / BLOCKED** by native Codex `shell_snapshots` persistence; no env-value forwarding, opt-in schema or vendor configuration alteration in this PR.

## Contracts, limits and implementation order

- Read `AGENTS.md`, `docs/ARCHITECTURE.md`, `CONTRIBUTING.md`, `docs/plans/2026-11-04-native-profile-secret-collision-recovery.md`. Preserve installed manifests, lock state, foreign Skill and neighbor bytes, owner identity, CLI `--no-ui`/`--yes`/`--scope`, JSON contract, cancellation and negative symlink/race refusals. No `--force`, foreign adoption/removal, changes to frozen VS Code, feature flags, new persistence/recovery manager or release.
- **Plan/critique first.** Two author-critical passes below; request independent CodeRabbit plan critique on the draft PR before code. Independently verify suggestions against current implementation; do not conflate author critique and external approval.
- **Native E2E:** extend the existing pinned Codex native matrix/repotest fixture rather than writing a new launcher. Probe exact native `codex --version` under a disposable HOME and project, run current-source `agentplugins skills add` using real CLI detection (not an injected detector), and compare with the pinned previously released UAP executable if available from the existing, digest-verified release acquisition. Require the same selected foreign `.agents/skills/<leaf>/SKILL.md`, native profile, exact source, typed `FOREIGN_DESTINATION` and zero modified bytes. No model/OAuth call. Do not relabel a successful test if the release binary cannot be authenticated.
- **Refusal path:** the code already projects `ErrForeignSkillDestination` to a safe generic message. If the real test proves one unambiguous collision, convey a **validated project-relative** target derived from trusted scope/layout/leaf (never echo raw provider error, absolute host path, arbitrary candidate name or a symlink-resolved path). Keep JSON keys and stable code unchanged; if a safe relative location cannot be proven, retain generic message and mark issue blocked.
- **Crash:** trace the actual CLI/Management construction and startup/recovery call before adding test. Build on the existing deterministic child `os.Exit` phase: a subsequent new *CLI process* must be observed via its public status/recovery command, with state/receipt/journal/foreign bytes inspected externally and a second idempotent call. Do not add a production fault injection knob or synthesize `sleep; kill`; if the kernel fixture cannot be meaningfully opened by the public CLI, record this sub-gate `NOT_RUN / BLOCKED` and do not fabricate coverage.
- Make focused commits; immediately push; repeat `make test-core`, `make lint`, public CLI tests, relevant native Codex Windows/macOS and four-platform crash/recovery E2E on the **final exact HEAD**, Required/CodeQL and independent reviews. Squash merge only when supported behavior and all applicable gates pass; close #629 only after the actual acceptance and merge. Keep #642 open. Verify post-merge main on any follow-up commit.

## Critic pass A — contract/security (author, not independent)

- **Challenge:** the words "released CLI" could be misrepresented by a Go test binary or a fake `Detect()` fixture. **Correction:** report released 0.1.95 as a reproduction baseline, and current-source candidate as a distinct qualification; admit real client detection only with pinned genuine Codex and recorded hashes/version.
- **Challenge:** the existing error wraps a logical Skill name that may originate in untrusted source. **Correction:** accept only a trusted validated leaf and fixed project-relative root; never interpolate lower-level `err.Error()` or absolute paths. An ambiguous client/target stays generic.
- **Challenge:** a native Codex discovery invocation could create user files. **Correction:** use a disposable HOME/XDG and inventory before/after; do not touch user-native profiles or require authentication.

## Critic pass B — architecture/E2E (author, not independent)

- **Challenge:** a second "crash manager" or a new workflow would duplicate the tested `dirswap` kernel. **Correction:** reuse `process_crash_recovery_test.go` and an existing public CLI/Management entrypoint, only if one deterministic supported boundary is available.
- **Challenge:** applying `Kernel.Recover` directly in the test child is **not** a CLI restart E2E. **Correction:** demand evidence of a newly started public executable and public status/result plus exact on-disk receipts/journals; do not conflate orderly `deps status` with crash.
- **Challenge:** new commits can invalidate earlier green PR #704 evidence and other contributors' main changes. **Correction:** don't mutate main; compare source SHA and optional merge SHA for each job/artifact; close only tested issue, not #642; avoid expanding to unrelated adapter support.

## Current admission ledger

| Assertion | Status | Required evidence |
| --- | --- | --- |
| #629 released CLI baseline on genuine Codex | NOT_RUN | Exact binary version/digest, native detection, original generic refusal, unchanged foreign files |
| #629 current CLI with genuine Codex, typed refusal | NOT_RUN | Public command output/JSON, physical tree snapshots, no fixture detector, exact source commit |
| #629 safe relative path UX | NOT_RUN | Bounded relative path and malformed/foreign path negative tests |
| CLI/Management new process recovery | NOT_RUN | Real child crash + public CLI process and durable state/journal before/after |
| #704 post-merge main | IN_PROGRESS | Actual push-to-main Required/Native/Package/Docs results, no queued/cancelled counted as pass |
| #642 Codex secrets | BLOCKED | Remain open; vendor native persistence proof is separate |

**Stop rule:** If released binary acquisition, safe relative path, or deterministic CLI recovery is not qualified, keep that lane blocked with evidence rather than modifying production safety or manufacturing a PASS.
