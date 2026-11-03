# Research & staged plan — shared dependency safety after Git pull (2026-11-03)

Status: research / critique first, **not** a claim of E2E success. Draft PR until independent review and final SHA CI qualification.

## Evidence and scope
- Microsoft APM 0.35.0 issue [#3188](https://github.com/microsoft/apm/issues/3188) (open as checked 2026-11-03): fresh teammate `git pull` removes a dependency from shared manifest+lock but leaves agent-visible old Skill, and audit/install/prune may miss it. This is a reported upstream failure on the cited version, **not** proof about a future APM release.
- UAP already passed single-root post-pull removal in [six-risk E2E #634](https://github.com/777genius/universal-agent-plugins/pull/634). The four-OS Package Manager catalog currently separates shared-DAG lifecycle, foreign drift, and cold restore. None proves the *combined* two-root / shared-child / actual-pull / multi-step replay invariant.
- APM [#3391](https://github.com/microsoft/apm/issues/3391) cold committed native Skills, [#3423](https://github.com/microsoft/apm/issues/3423) / [#3425](https://github.com/microsoft/apm/issues/3425) Claude user profiles and [#3393](https://github.com/microsoft/apm/issues/3393) Codex secret forwarding are adjacent but **out of scope** here: separate UAP #641/#642 and pinned native-client qualification are required. Do not make unearned claims.
- This is a behavior/safety qualification, not an APM manifest-format clone, a new install engine, a standalone background audit daemon or a broad architecture refactor. No feature flags; VS Code remains frozen.

## Intended scenario (one four-platform required E2E case; actual Git subprocesses)
1. Create one author repository and one independent teammate clone with actual `git init`, `git clone` and `git pull --ff-only`. Only `uap.toml` and `uap.lock` are tracked. Put all native agent settings, package store and receipts in disposable *untracked and distinct* per-runner locations. Use existing in-process HTTPS/TLS archive fixture, real CLI and production installer; fake only native client responses/security assessment already used by existing portable suite.
2. Install roots A, B, both requiring child C, on both machines. Verify distinct package digests, actual native Skills and source identities, stable lock, and the teammate's existing owner receipts. Seed a foreign settings/Skill sentinel.
3. Author removes B, locks and installs without touching A/C; commit portable pair, teammate pulls the exact new commit. Teammate `deps status` must not itself mutate; one `install --locked` must remove B and retain A and C **with original C ownership, inode and bytes**. No unowned/foreign loss, no unauthorized network/refetch, lock edits or false `synced` before actual install.
4. Author removes last root A, locks, installs and commits. Teammate pulls; before replay, externally edit C's managed file. Locked replay must refuse before deleting edited C or the surviving A, with accurate status/recovery and original user data; clear only the deliberate local edit, retry locked replay. After authorized reconciliation, A and C disappear and no stale active Skills remain. A second replay is byte-identical and does not reacquire sources.
5. Run exactly on Linux, Windows 2025, macOS ARM and Intel via existing `uappm_e2e` workflow, with fail-on-skip, Go run/pass events and inspected `report.json`. No new CI workflow.

## Pre-code critical reviews and amendments
- **Standards/product:** APM's package type, local path and `apm.yml` differ from UAP. For this UAP proof, use supported HTTPS ZIP + portable TOML/lock, not an invented compatible source kind; judge only shared safety outcome.
- **Security/data-loss:** The owner may edit managed native bytes. Refusal must preserve them; reconciliation after the owner deliberately removes that edit cannot silently adopt a foreign file. Verify selected root, separate HOME/state, inode+mode+bytes and retained receipts; do not reduce success to `exit 0`.
- **Architecture/test quality:** Reuse `pmArchiveEnvironment`, `pmArchiveOrigin`, `packageManagerEvolutionCases`, `freshRestoreFiles` and CLI state inspector. One integrated scenario rather than duplicating graph-solver tests, no second oracle or production service. Verify actual Git CLI and tracked-file cleanliness.
- **Critic amendment 1 (pre-code):** Split the first and last consumer removal; this detects both early deletion and invisible orphan. A one-off orphan test would miss ownership handoff.
- **Critic amendment 2 (pre-code):** Add the native-drift refusal *before* last deletion, then explicit user resolution and idempotent retry. It distinguishes fail-closed protection from a stuck package manager.
- **Critic amendment 3 (pre-code):** Make `deps status` read-only before each install and inspect receipts/native Skill bytes independently. Git lock fields alone cannot certify the actual agent-visible outcome.

## Merge evidence
Use GitHub PR reviews (security, transaction, architecture) and existing required CI; fix confirmed source/test defects on same branch. Capture full four-platform portable `run/pass` evidence at the **final HEAD** and verify no open blocking findings before squash-merge. Recheck resulting `main` tree and post-merge CI separately. Public npm releases/tags are out of scope absent a release request.


## Critic round 4 — observed E2E failure and corrected transaction contract (2026-11-03)

The first real macOS ARM Package Manager E2E on HEAD `3c3e1a05` **failed** in the new scenario after a deliberately edited native C Skill. Inspection of the failure's before/after artifacts established that the installer refused C removal but had already committed the independent removal of A, including exact native settings and durable receipts. It did **not** silently delete the edited C. Our first test incorrectly required a whole-batch rollback: the existing coordinator implements per-binding commits and explicitly reports partial outcomes. Suppressing this failure or claiming full atomicity would be misleading.

**Security/data-loss critic correction:** retain strict invariants for C's edited bytes + original binding/receipt, unrelated foreign Skill and user-data, portable intent, no source downloads. Allow A to be either wholly retained or wholly removed, **but** require its native and managed state to agree with durable ownership, no orphan/unknown receipts, and `deps status` to return `partial` or `not_synced`. A later explicit owner correction and locked retry must clean up both A and C; the final repeat must be a real no-op. The test independently checks state, actual files and status. This is **not** permission for silent partial deletion or a success exit after drift; no generic rollback claim.

**Architecture critic correction:** do not replace UAP's transactional coordinator with a new global rollback engine for a test-only parity claim. The observed partial behavior is within the existing published mutation contract. A separately scoped future improvement could preflight native projections for all batch removals, but that changes native verification, race and client compatibility boundaries and must be researched and reviewed independently.

**Review/qualification reset:** previous HEAD `3c3e1a05` is a **failed** E2E and cannot count as successful. The corrected test must pass on all four OS runners at the new exact HEAD, with independent Strix/CodeRabbit review; otherwise do not merge.
