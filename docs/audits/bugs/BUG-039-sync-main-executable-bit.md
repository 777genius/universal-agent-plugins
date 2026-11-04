# BUG-039 — Preserve main's executable mode during PR sync

## Reproduced cause

PR #698 merge commit `e25787b614381a3238dee9e6e421ff07a12bf3fa` synchronized nine non-overlapping files from current `main` (`acedefd5fab319176fa4ff286349eeda0dd1ff62`). A Git-tree integration script copied all blobs with uniform mode `100644`; the source tree actually stores `scripts/check-lint-baseline.sh` as `100755`. Same script contents/SHA, incorrect executable bit. All other eight copied file modes were compared and match main.

## Minimal correction / regression

Restore *only* the existing `scripts/check-lint-baseline.sh` blob mode to `100755`, with identical blob SHA `1179b905aa84d59326fb5304352b2c8bfe6e8029`. Do not change lint rules or skip architecture checks. Verify the new branch Git tree has the same mode and blob as main, and that all nine synced paths retain their source modes. Run Required/Core CI on the correction's exact commit; do not use earlier workflows as final-head evidence.

## Two critical checks (self-review, not external approval)

- **Correctness:** Executable mode is part of Git's file tree. A content-only comparison missed this difference; the source and target must match both blob and mode.
- **Scope/safety:** A mode-only correction is safer than rewriting the shell script or disabling the new lint check. Preserve all original PR changes and both parents of the prior merge. No effect on customer profiles, native clients, vendor trust, LintAI or containment.

**Status:** scoped correction applied in PR; exact-head successful Required/E2E and independent review still needed. No merge clearance.
