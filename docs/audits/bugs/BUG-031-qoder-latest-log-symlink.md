# BUG-031 — Qoder's retained latest-log symlink is misclassified as active plugin residue

## Actual 30-source reproduction

[Final 30-plugin ZIP in GitHub Actions run 41814723215](https://github.com/777genius/universal-agent-plugins/actions/runs/41814723215) at PR HEAD `97378eac646ea6f64686d37b7c564910137a918e` contains **23** `cleanup-failed` entries for Qoder 1.1.81. All 23 have a successful source-pinned `add`, verified `list`, unchanged repeated `add`, successful UAP `remove`, and a structurally valid post-remove list with no managed installation. The *only* recorded `native_residuals` entry on each is exactly `.qoder/logs/latest: symlink`. The original native-link destination was **not** captured, so these are still non-PASS cases pending a verified target.

Qoder's published [settings/file-path reference](https://docs.qoder.com/cli/settings-reference) documents `logs/runs/`, `logs/sessions/` and the `latest` log-run reference as diagnostic logs with independent retention. These are not by themselves plugin installation entries; however their symbolic link target must still be checked for containment.

## Narrow, fail-closed plan

1. Keep `snapshot` recording all symlinks. In the **post-removal Qoder-only** native residual oracle, allow exactly `<isolated .qoder>/logs/latest` *only if* it is a **relative** symlink to a real regular file/directory under the same real `logs/runs/` or `logs/sessions/` tree. Require canonical containment and no other symlink in the target's ancestry. Absolute, dangling, external, plugins/cache, wrong-name, changed-root or chained symlinks must remain a refusal.
2. Keep every existing native registration/list, true plugin identity/file, foreign-neighbor and post-remove proof unchanged. This log exception must not make a package active or override failed installs; it only prevents an unrelated inert, verifiably contained vendor log pointer from failing an otherwise verified removal. No user profile is modified.
3. Add small real-filesystem regressions for the ordinary contained log pointer and a malicious external/linked/dangling/non-log pointer; ensure all other symlinks still fail closed.
4. Run exact-head targeted Python tests and genuine 30-source Qoder plugin E2E, and inspect whether those 23 actually transition to PASS or stay blocked. A successful unit test cannot substitute for inspecting the real vendor's link target.

## Two critical perspectives (plan review, not independent approval)

- **Cleanup oracle critic:** Dropping all symlinks or every Qoder `logs/*` path would let attacker-controlled native registrations escape the cleanup assertion. An exact, verified log-only pointer is the smallest defensible exclusion.
- **Containment critic:** Never trust a string prefix before normalization, absolute link values, dangling targets or symlink chains. The original symlink remains a recorded artifact in the snapshot; the separate matching identity/lock/native removal checks cannot be relaxed.

## Status

Historical evidence and vendor log semantics confirmed. Exact target and final-head qualification **pending**; no product installer behavior or security policy changed.
