# BUG-019 — Copilot native settings retain identity after UAP remove

## Reproduction and evidence

The original JSON ZIP [Copilot plugin artifact 13293563101](https://github.com/777genius/universal-agent-plugins/actions/runs/41766817078/artifacts/13293563101) from [GitHub Actions 41766817078](https://github.com/777genius/universal-agent-plugins/actions/runs/41766817078) on historical branch HEAD 5e79cf1a2d34a900f7fb5a542b39d134c8b29057 contains **29/30 cleanup-failed cases** (one source/target unsupported). In those 29, the native Copilot file .copilot/settings.json changed with the installed identity and *still contained the identity after the remove command exited 0*.

Representative atlassian case: add exit 0; listed_identity=true; repeat_verified=true; native_identity_files=[".copilot/settings.json"]; cleanup exit 0; post_remove_list exit 0 and empty; post_list_schema_valid=true; native_residuals=[".copilot/settings.json"]; sentinels_preserved=true; removal_verified=false.

This is reproduced **native cleanup evidence**, not proof that Copilot activates the residual key: the ZIP contains file names and identity checks, not complete post-removal native JSON. It differs from BUG-018's Qoder *passive cache*, because Copilot settings.json is native configuration. Do not relabel PASS.

## Minimal investigative and implementation plan

1. Obtain actual isolated Copilot settings.json before add, after add and after remove in a disposable profile, with unrelated credentials redacted. Confirm whether remaining identity is an active MCP/plugin binding or an inert value, identify the exact native adapter writer/remover.
2. If active, minimally fix the existing Copilot native uninstall adapter to remove **only** its UAP-owned binding, preserving unrelated settings and failing closed on ownership conflicts; never overwrite the entire settings file.
3. Add regression with independent foreign setting, repeat add/remove and native-state inspection; rerun the full exact-head Copilot plugin lane.
4. If the key is inert, document independently verified schema semantics before changing the evidence oracle. Keep all 29 historical results cleanup-failed until verified. No paid model, auth or security bypass.

## Critical review round 1 — deletion safety

A foreign/user-owned Copilot setting must never be deleted. Require exact owned binding identity. Do not remove unrelated keys with broad substring matching.

## Critical review round 2 — evidence

An empty UAP list does not prove the native configuration is clean. Require native-state and exact remove/foreign-file checks on final HEAD. If root cause is not established, leave this open and non-PASS.

## Status

Source-backed negative case reproduced. **Product root cause and fix not yet qualified.**

## Latest real ZIP and narrow forensic work (2026-11-04)

The genuine `copilot-plugin.json` from [run 41788848418, artifact 13303655153](https://github.com/777genius/universal-agent-plugins/actions/runs/41788848418/artifacts/13303655153) reproduces **23 `cleanup-failed`**, **6 `incomplete-native-proof`** and one source-target refusal. For the 23 completed removals, UAP's post-remove `list` is empty but `.copilot/settings.json` is still classified as an identity-bearing native residual. This is **not** 23 PASS, and the source ZIP intentionally does not contain user-profile JSON values. A settings path alone does not prove whether the residual is an active plugin, marketplace record or incidental retained value.

**Minimal read-only next step before a product fix:** instrument the already isolated Copilot audit cases to collect the **same fixed native settings file** at three boundaries: before add, after add and after remove. Retain only bounded match counts for exact plugin-identity keys/values, `identity@agentplugins-<12 hex>` keys/values and other substring references, plus a small allowlisted set of structural root sections. Do **not** emit any settings values, credentials, arbitrary key names, source paths, or snapshots. Reject symlinks, unsafe paths, oversized/unreadable/malformed JSON. Add tests for a genuine qualified-key shape, a benign path substring, redaction, and an out-of-case symlink. The probe is evidence-only: it cannot change `cleanup-failed` into PASS or authorize `--external-uninstalled`.

**Critic round 1 — privacy and containment:** independent JSON values must never appear in ZIP, including unknown nesting; no native file outside the disposable case may be read.

**Critic round 2 — proof semantics:** distinguish a qualified registration-shaped token from an incidental substring; even a token is only evidence for investigation, not self-sufficient native activation or deletion proof. Preserve negative statuses and do not change Copilot/VS Code shared production removal until the exact owner and native schema are independently established.
