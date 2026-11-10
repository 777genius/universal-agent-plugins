# BUG-041 — audit case-root symlink allowed an out-of-sandbox write

## Reproduction and impact

In `scripts/live-install-audit.py:do_case`, the old sequence was `case_root.mkdir(parents=True)` followed by `canonical_case = case_root.resolve(strict=True)`. The child-directory guard compared all subsequent parents against this **already redirected** `canonical_case`. A stale `runroot/plugin-00` symlink planted by a previous untrusted package and pointing to a sibling `foreign` directory was accepted; `(case_root/'home').mkdir(...)` and writing `home/foreign-untouched.txt` touched that outside directory. An isolated direct Python reproducer confirmed this behavior on a temporary filesystem on 2026-11-04, without executing any external program.

This is an **E2E harness containment vulnerability**, not a claim that a production UAP installer bypass was observed. Same-UID malicious package scripts may mutate sibling paths between cases. This guard is not an OS sandbox and cannot prevent an attacker racing all local operations, but a direct *pre-planted root symlink* must never be accepted.

## Minimal fix plan

1. Before creating `runroot` or `case_root` descendants, refuse a symlinked or non-directory parent/root. Require an existing real parent for `runroot`; make only that single root (`mkdir(exist_ok=True)`). Refuse a symlinked case directory before `case_root.mkdir`. On returning from mkdir, verify neither root was replaced with a symlink and `case_root.resolve(strict=True).parent` is the canonical `runroot`.
2. Keep the existing per-child pre-write containment checks for `home`, `project`, `data`, `.config`, `.cache` and `.local/share`. Do not change native permissions or any product client adapter, installer, selector or release policy.
3. Add no-network regressions with disposable symlinks at *case_root* and *runroot*, and an injected symlink immediately after case-root creation. Assert the foreign directory remains entirely empty and the native installer subprocess was never called.
4. Rerun Python harness tests then full 30×30 final-HEAD Actions, inspect the original ZIPs; never treat a skipped/failing case as PASS.

## Critical reviews before code

**Boundary critic:** Comparing a child to the canonical case directory is meaningless if the canonical case has itself already escaped. Anchor the case against the independently created runroot **before** any child write, and refuse pre-existing symlinks at both roots. Merely calling `.resolve()` after `mkdir` repeats the bug.

**Regression/security critic:** Do not add broad directory deletion, `followlinks=True`, privileged containers, or a new sanitizer framework. The tests must plant an actual symlink at the exploitable *case directory*, not merely within it, and prove no content appears in a foreign sibling. Same-UID concurrent TOCTOU is outside this limited fix and remains an explicit limit.

## State

Confirmed by local executable repro. Narrow root guards and a three-case symlink regression implemented in this PR; final-head Python/CI and real-case ZIP requalification are still pending. No merge approval.
