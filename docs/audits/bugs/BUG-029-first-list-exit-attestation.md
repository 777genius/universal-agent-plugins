# BUG-029 — Failed first `list` exit can qualify an installation

## Reproduction (audit code, HEAD d5b778ee)

In `scripts/live-install-audit.py`, the first post-add `list` is parsed and fed to `matching_installations`. The `installed` decision checks the **add** exit code and the parsed matching row, but not the **first list** exit code. A CLI response with `exit_code=1` and an otherwise matching JSON `data.installations` therefore reaches repeat-add and removal. With successful later calls, the harness can certify `passed` without a successful initial list. The later repeat-list and post-remove exit checks do not repair the missing initial-list observation.

The latest genuine ZIPs for [run 41807091758](https://github.com/777genius/universal-agent-plugins/actions/runs/41807091758) contain successful list observations for passed cases. They **do not** prove this failure happened live; reproduce the precise counterexample with a mock subprocess regression before changing the oracle. This is an evidence-harness correctness defect, not a production CLI bug claim.

## Minimal plan

1. Add a regression using `do_case` on a disposable isolated profile: successful plugin add, first `list` exit 1 but parseable matching JSON, and potentially successful subsequent commands. Assert no repeat-add, no remove and never `passed`.
2. Require `listed["exit_code"] == 0` in the existing `installed` gate, preserving the captured list JSON for diagnosis. Keep a successful zero-exit list lifecycle covered by existing tests.
3. Run `python -m unittest scripts/test_live_install_audit_env.py` and applicable CI on the exact pushed HEAD; inspect retained JSON artifacts before any final-head credit.

## Critic round 1 — evidence integrity

A JSON payload from a failing CLI process can be partial, stale or diagnostic. Matching a row is not a successful observation. The first list must be successful even if the repeat list later succeeds. Do not silently classify it as installed.

## Critic round 2 — scope and safety

Change only the harness qualification gate and one counterexample regression. Preserve the diagnostic output and strict fail-closed statuses. No changes to product install/remove, vendor bootstrapping, LintAI, cgroup, host profiles or security policy. This fix cannot improve any historical ZIP verdict retroactively.

## Status

Regression committed in `9653665e54ae2d325b7438852d7f2fc6c0159477`. Harness gate fixed in `736af6d330e987ad23b6bcb51286ac79e37aa11d`. Both commits pushed to PR #698. The counterexample now asserts initial list exit=1, matching parsed JSON, **no repeat add**, **no remove**, and `incomplete-native-proof`. CI and fresh full-lane ZIPs on the final documentation HEAD are pending; no final-HEAD PASS or merge authorization.
