# P0 — Windows long TEMP / Remotion: root-cause E2E

## Starting point
Reuse diagnostic draft PR #780, published UAP `0.1.101`, exact frozen Remotion Skill source and native pinned Codex. Existing short `RUNNER_TEMP` PASS is not evidence of Windows default-long-TEMP support. Long-profile refusal `SKILL_REJECTED` must remain visible until the real cause is corrected.

## Work
1. Extract original JSON/ZIP evidence from #780 without modifying source fixture; record original path lengths, `core.longpaths` behavior, Git checkout/extraction failure stage, tree/manifest and native result. Verify which component owns the temporary path.
2. Reproduce both disposable profiles using the **same** public CLI, native client, immutable commit and checkout options. Control for case sensitivity/Windows policies and read-only comparison. No real HOME mutation.
3. Fix only the responsible path handling (prefer short private staging root *within* verified per-run private directory if justified, or a narrow Windows Git extraction correction). Preserve scope/ownership, immutable source, junction/symlink traversal refusal, lock bytes, foreign files, and normal short-path behavior. Keep #780 diagnostic-only if needed; production code fix in a focused distinct PR after a proven cause.
4. Add regression: both long and short profiles, with `core.longpaths` toggles, actual Skill add/repeat/list/remove, pinned hashes, no leftovers; still reject deliberately malicious traversal and mismatched source.
5. Independent review → exact-head Windows native E2E → merge only if real long-path PASS and no weakened security policy. No new release or fake success.

## Acceptance
Exact source `Remotion` installs from the long default Windows TEMP profile with genuine published or explicitly source-built CLI (clearly separate evidence); original failure reproduced before fix. Any required public-release qualification is a **later** gate, never silently inferred. No duplicate copy of #780's diagnostic harness.
