# BUG-027 — relocated Skill payload may bypass removal proof

## Evidence and reproduction
Strix's independent review on PR #698 (2026-11-04, HEAD `c7fbfecf8db6`) found a real **harness oracle** gap: removing or rewriting `SKILL.md` and relocating `scripts/payload.sh` into another directory under the same adapter's project Skill root leaves no identity named `SKILL.md` and no original installed path. The old `removal_verified` could then be true while executable payload bytes remained.

## Minimal plan
- Capture content-hash snapshots of the adapter's project Skill roots after the unrelated foreign fixture is created but before the first add.
- After remove, compare the whole post-remove snapshot with the exact baseline; **both added/changed and missing paths** invalidate the result.
- Keep existing identity/path/lock checks. Record changed path names only in `post_skill_tree_delta`, never file contents.
- Regression: relocate payload without `SKILL.md` must be `cleanup-failed`; a genuinely clean remove must still pass. Do not alter LintAI, cgroup, authentication, or product semantics.

## Critic round 1 — correctness
A name scan is not an integrity oracle; exact tree differences cover relocated/renamed residuals. Comparing both directions also catches deletion of unrelated files. Legitimate client changes inside project Skill roots must remain visible, not silently waived.

## Critic round 2 — isolation / scope
Roots are those already declared in `PROJECT_SKILL_ROOTS` under the disposable project. The existing snapshot refuses symlinks without following them. Do not traverse another user's profile, accept a bypass, or reinterpret unsupported/blocked sources as PASS.

## Status
Implemented with a positive/negative regression in the PR branch; current-head CI and independent Strix review must be rechecked. No merge authorization.
