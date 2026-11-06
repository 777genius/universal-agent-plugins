# BUG-066 — Grok prepared project Skills skip actual repeat/remove E2E

## Source-backed reproduction

PR #698, source HEAD `40e427a9103c63f87b8fe4c34dcd3ca98ae5d832`, original Grok Skills Actions ZIP [#13450929454](https://github.com/777genius/universal-agent-plugins/actions/runs/42096595214/artifacts/13450929454): all 30 pinned source installs return exit 0, exact SKILL.md Git-blob bytes and lock are present, and `grok inspect --json` succeeds after BUG-063's real isolated Git root. The genuine Grok client reports `materialized/package_validated/activation=prepared` under a generated `skill-<hash>` bundle name, not the raw Skill alias. This is **not** native activation. The strict original audit therefore recorded 30 `incomplete-native-proof` and could not test repeat or remove.

## Scoped correction

Only the shared Python live audit and hermetic regressions change; the production Grok adapter, immutable fixture, signed registry, vendor SHA-256, LintAI and process/profile containment do not. A prepared Grok *project Skill only* may be selected for managed lifecycle testing when one actual CLI list entry exactly matches the immutable `repo/skills/folder` source, full Git revision, per-package tree/manifest digests from the real `uap.lock`, unique stable installation UUID, and `package_validated/prepared` Grok project client. The first native `skills add` JSON must point to the real contained bundle directory with exact pinned SKILL.md, no symlinks, and a successful source receipt; the idempotent repeat must report `unchanged`.

The repeat requires stable installation UUID, identical native/profile and full Grok project Skill tree snapshots, and identical project lock. Remove uses only the verified installation UUID (not the raw Skill alias). Post-remove list rejects the old ID, source or any re-minted generated bundle identity; verify full Skill removal, absence of lock/native residuals, and preservation of foreign sentinels. `prepared-lifecycle-passed` is explicitly **non-PASS** and is never evidence of native activation or execution.

## Reviews and acceptance

Correctness pass: the target/source/revision is tied to the unique actual managed installation, repeat and removal; source/ID reminting or partial output cannot yield false PASS. Security pass: every physical path is checked inside disposable project without symlinks, lock digests bind native receipts, unmanaged files and sandbox/custody policies remain unchanged, and no external uninstall attestation is fabricated. Both passes must be rechecked against the final code and real ZIP evidence.

Acceptance requires focused no-network regressions and a **fresh 30/30 original full ZIP / 900 pinned source rows** on the final PR source HEAD (not the historical `40e427a`), successful applicable CI, source-bound negative statuses, review disposition and eventually a safe squash merge plus successful post-merge CI.
