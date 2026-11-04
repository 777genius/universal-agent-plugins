# BUG-020 — native removal oracle misses files created after add

## Independent finding and reproduced source path

[Strix review on PR #698 (2026-11-04)](https://github.com/777genius/universal-agent-plugins/pull/698#discussion_r4463461326) identified an audit-integrity false PASS in `scripts/live-install-audit.py`. On branch HEAD `b26fab66faae20bb991a258e1481c652295b4710`, `do_case()` passes only post-add `native_attribution + unsafe_paths` into `native_identity_residuals()`. A package-specific native config registered by a delayed writer or by the native/remove step after that snapshot can remain on disk after successful CLI `remove` and be omitted from `removal_verified`. The before/after code path and independent reproduction support a **harness proof defect**, not a confirmed product uninstall defect.

## Minimal plan

1. Preserve the existing post-add attribution and unsafe path set, but also take a **fresh snapshot of the disposable native roots after remove** and pass the union of path keys to `native_identity_residuals`; do not treat an old or current missing path as residual unless it still exists.
2. Retain fail-closed symlink, unreadable, non-regular and walk-error handling; keep narrowly documented passive Qoder/Codex regular cache/log exclusions.
3. Add one focused regression that injects a new identity-bearing native config **during the remove** after post-add attribution, verifies clean CLI list/remove and no other fail gate, and requires `removal_verified=false` / `cleanup-failed`; add a genuinely clean control.
4. Execute Python unittest and applicable workflow/E2E checks on the **new branch HEAD** and inspect the real JSON ZIP. Do not reclassify old PASS, unsupported, prepared, or blocked outcomes.

## Critical review round 1 — soundness and security

The new scan must use the already isolated profile roots only, never ambient HOME/APPDATA or links beyond them. A post-remove symlink or unreadable entry cannot be hidden by the passive cache exception. No LintAI/cgroup policy or vendor SHA bypass.

## Critical review round 2 — narrowness and regressions

Do not require deleting unrelated regular native settings that do **not** contain the managed identity. Preserve unchanged foreign-neighbor sentinels and normal removal outcomes; no new feature flags or production adapter refactors. A cleanup-failed classification is correct until disk identity is actually absent.

## Status

Plan recorded before narrow code change. Implementation, focused regression, final-head CI and independent re-review pending.
