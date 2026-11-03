# BUG-006 — Successful CLI install mislabeled unsupported from diagnostic JSON

## Real reproduction (historical artifact, not final-head qualification)

[Run 41727674724, artifact `remaining-client-cursor-plugin`](https://github.com/777genius/universal-agent-plugins/actions/runs/41727674724), source index 24 `google-cloud-storage`, has:

- `install.exit_code = 0` (the public UAP CLI reported success and wrote 75 native profile file changes);
- `listed_identity = false` in the earlier harness, so this attempt did **not** meet acceptance criteria;
- `status = documented-or-source-unsupported`, solely because the first 1,800 characters of successful JSON contained the word `unsupported` in component support diagnostics.

The source was *not refused* at install time. Classifying the entire successful package install as unsupported is false evidence, and it can hide a missing post-install verification step. Similar false labels could arise from `not detected` / `target_unavailable` and other diagnostic strings inside successful JSON.

## Minimal plan

1. Apply all text-driven failure classifications (containment unavailable, target/client unavailable, source unsupported, test-version policy, suspended distribution) only when the **actual install exit code is nonzero**. An unknown exit/timeout must not be converted to a documented source refusal from incidental text.
2. When `add` returned zero but native/list/lock proof is incomplete, retain `incomplete-native-proof` or the existing more specific positive-install result. Never grant `passed` based on strings, exit code alone or the previous artifact.
3. Add small regression for the predicate with exit `0` and incidental unsupported/target-unavailable text, exit `1` real source refusal, and `None` timeout. Re-run all live cases on final head and investigate remaining incomplete proofs separately.

## Plan critique

- **Correctness critic:** A per-component unsupported capability is not equivalent to a failed package install. Report the actual top-level CLI result first and do not downgrade a success to a source refusal.
- **Security critic:** This only corrects evidence classification. It does not change installer behavior, whitelist unsupported sources, skip security policy or turn incomplete proof into PASS.

**State:** reproduced from original real JSON; minimal classifier fix + current-head verification pending.
