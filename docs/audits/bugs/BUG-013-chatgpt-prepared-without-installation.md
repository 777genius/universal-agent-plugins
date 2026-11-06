# BUG-013 — ChatGPT manual setup reported prepared on CLI exit alone

## Reproduction (harness defect; no product defect asserted)

On the current PR head `acd7c0b054c72f3cb7359beb826e897b74bba21e`, `scripts/live-install-audit.py:do_case` assigns `manual-setup-prepared` whenever a ChatGPT plugin `add` returns exit code 0. It does so *before* checking the parsed `list` installation identity, removal result, and foreign-file sentinels. The deterministic regression supplies an add command with exit 0 but an empty installation list: the old branch incorrectly reports preparation. The second counterexample records a matching installation but a failed removal: the old branch again reports preparation despite unfinished lifecycle.

This is a false-positive evidence classification. It does not demonstrate a defect in real ChatGPT or UAP product installation.

## Minimal plan

1. Require the already calculated `installed`, `removal_verified`, and `sentinels_preserved` conditions for the ChatGPT manual-setup state. Keep it a **prepared** classification, never a native activation or `passed`.
2. Add one focused `do_case` regression with three outcomes: successful add + no registration is `incomplete-native-proof`; registered add + failed removal is `cleanup-failed`; exact matching registration + successful verified cleanup is `manual-setup-prepared`.
3. Run `python3 -m unittest discover -s scripts -p 'test_live_install_audit_env.py'` on the new commit through actual CI; inspect fresh JSON artifacts. No security bypass, model call, feature flag, or unrelated product change.

## Critical review round 1 — evidence integrity

A zero-exit command alone does not establish installed, listed, attributable state. Reusing existing `installed` ensures a verified target-specific managed record and immutable-source checks, without an additional ad hoc heuristic. The regression must also fail against the original branch.

## Critical review round 2 — safety and scope

ChatGPT has no native plugin filesystem root: requiring a native-file delta would incorrectly make *all* remote/manual setup claims impossible. Instead require verified list identity and cleanup while preserving the explicit `manual-setup-prepared` limitation. An earlier special case must not hide cleanup or foreign-file damage; no external app activation is inferred.

**State:** narrow classification correction and three-way regression included; real final-head CI and independent reviewer clearance still required. Never count a synthetic test as real-source installation proof.
