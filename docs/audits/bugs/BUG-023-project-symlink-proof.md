# BUG-023 — project Skill and foreign-neighbor proof follows substituted ancestors

## Reproduced counterexample and impact

Independent Strix review of PR #698 on 2026-11-04 identified a false-PASS condition in the live E2E harness: a client install can replace an intermediate project directory (for example `project/.claude`) with a symlink pointing outside the disposable test case. `preserved_regular_text` checks only the final file, while `skill_file_present` and `skill_removal_leftovers` do not validate all parent components. A byte-identical foreign-neighbor sentinel outside the case can therefore impersonate retained project state. This is a **harness trust-boundary bug**, not evidence of a UAP production installer defect.

## Narrow implementation plan

1. Require the path to be lexically and physically inside the disposable case root, rejecting symlinks in all components between case and leaf (also missing or inaccessible roots).
2. Apply the shared check to every foreign sentinel and to project Skill discovery and removal residuals. A substituted Skill ancestor is negative removal evidence even when a final SKILL.md path is absent.
3. Add a counterexample with `.claude` symlinked outside the case, a forged identical sentinel and an outside Skill. Retain the clean native and Skill lifecycle controls.
4. Run focused unittest and both real GitHub Actions workflows on **new branch HEAD**; inspect ZIP JSON before any PASS claim.

## Critical review round 1 — containment and security

No follow-through to the foreign account directory or deletion of foreign files. Reject any changed authority chain, not just the leaf. Resolve case-root paths correctly on macOS `/private/var` and respect Windows APPDATA/LOCALAPPDATA isolation. Never bypass LintAI or cgroup.

## Critical review round 2 — scope and correctness

Change only E2E evidence oracles, add focused regression. No production client change, no feature flags, no broad refactor. Do not count a legitimate missing Skill as residual, only a substituted or remaining managed path. External review, full native CI and final-head 15-adapter coverage remain separate gates.

## Status

Plan recorded before code. Final-head runtime qualification and independent review must be repeated after the fix.
