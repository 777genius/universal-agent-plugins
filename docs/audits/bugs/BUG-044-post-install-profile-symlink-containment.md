# BUG-044 — native audit can traverse a replaced ancestor after running a package

## Source-backed reproduction / impact

In `scripts/live-install-audit.py:do_case`, the initial root creation is guarded, but a subsequent untrusted plugin install or native command can replace `case/home`, `case/project`, or an intermediate native profile directory with a symlink. Immediately after `call([cli, "add", ...])`, the old path reads `snapshot(native_roots)` without revalidating ancestors. The native root **itself** is not necessarily a symlink: an isolated filesystem reproducer made `case/home -> foreign`, with a regular `foreign/.codex`; `case/home/.codex.is_symlink()` was `False`, `.is_dir()` was `True`, and ordinary iteration reached foreign content.

This is a **test harness read/containment defect**, not evidence that production UAP performed an out-of-sandbox write, and not proof that adversarial same-UID subprocesses are isolated.

## Minimal plan (harness only)

1. Retain the pre-source canonical run/case anchors from BUG-041. Add one local, reusable `assert_case_unchanged()` helper to `do_case` that refuses replaced/symlinked runroot/case roots, a missing or redirected `home/project/data`, and symlinks in every native or project-Skill root's descendants.
2. Wrap only the six existing external UAP subprocess calls (add/list/repeat add/repeat list/remove/post list), validating those anchors **both before and after** each call and **before any native snapshot, JSON source proof or filesystem read**. Apply the same post-call guard to the genuine Qoder native list and Codex native uninstall helper. A redirect must raise a deterministic `harness-error` (caught per source), never PASS.
3. Add disposable/no-network negative regressions that, from the mock *add* subprocess, replace `home`, `.config` native ancestor, or `project` with a symlink to a foreign sibling. Assert the very first attempt fails before post-add native scans or additional subprocesses and the foreign directory remains empty. Keep all existing symlink, foreign-sentinel and source-identity oracles.
4. Re-run source-specific Python tests, Required and 30×30 full/remaining CI on the exact resulting commit, inspect original ZIPs. Queued/cancelled workflows are not evidence.

## Critical review 1 — boundary / false-negative

Checking only `native_root.is_symlink()` is insufficient: a parent may be a link even when the leaf is an ordinary directory. Comparison must use immutable initial run/case anchors and reject all moved/relinked descendants *before reading or writing*. A failed check is a test harness error, not an eligible agent/source PASS.

## Critical review 2 — scope / security

This preserves existing domain/installer logic, client versions, signed source SHA, LintAI/cgroup, foreign neighbors and 30×30 fixture inventory; it adds no feature flag, generic file crawler, privileged sandbox or synthetic successful installation. It prevents predictable post-execution harness traversal but does **not** sandbox malicious same-UID subprocesses or eliminate TOCTOU races; full OS containment would require separately approved infrastructure.

**Status:** scoped patch and negative test committed to PR #698; current-HEAD CI and independent review outstanding. No merge authorization from this report.
