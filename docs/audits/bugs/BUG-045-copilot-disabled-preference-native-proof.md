# BUG-045 — Copilot disabled native setting is not an active plugin after verified uninstall

## Real reproduction

[Full Copilot 30-plugin JSON ZIP 13325744274](https://github.com/777genius/universal-agent-plugins/actions/runs/41830588372/artifacts/13325744274), exact historical source `bb574a1eb04bf58feaa54eaeff51d078fc02c967`, contains 23 `cleanup-failed` cases with successful UAP add/list/repeat/remove and no remaining UAP installation. In **all 23**, the isolated `.copilot/settings.json` had one exact managed `enabledPlugins` key `true` after add, then the same family of key with `false` after uninstall; no other post-remove identity matches. No raw credentials or native keys were exported. The old generic native file-residual check treated the disabled preference as an active plugin, without asking the actual Copilot CLI.

The official [Copilot CLI plugin reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-plugin-reference) documents that `copilot plugin list --json` is a flat array of installed plugins with `name`, optional `marketplace`, optional `version`, `enabled` and `source` fields. Its [configuration reference](https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference) documents `enabledPlugins: false` as *disabled* (not installed). **Native absence and immutable UAP receipt** are still required; a disabled setting on its own is not proof of an uninstall.

## Narrow plan: test harness only

1. Query the exact pinned absolute native Copilot CLI's read-only `plugin list --json` after verified repeat add and after UAP remove, with explicit case-local `COPILOT_HOME`. Require one exact matching installed/true `name` + `agentplugins-12hex` marketplace and (if supplied) exact UAP package version before removal, and no same-name plugin at all after removal. An unreadable, malformed, failed, duplicate, foreign, wrong-version or wrong-activation native listing never counts as successful.
2. Preserve existing native residual files in exported JSON; do **not delete** client settings. Permit only the exact post-uninstall `.copilot/settings.json` residual if an initially absent profile gained exactly one qualified `enabledPlugins=true` at add, and the only post-remove identity reference is that same-form `enabledPlugins=false` key with zero active/other identity references. Require independently observed native listing absence, same UAP receipt, repeat-ID/lock, source SHA/digests, foreign sentinels and unchanged unrelated native files.
3. Add small no-network negative regressions (native JSON wrong/foreign/missing ID, wrong version, malformed schema, nonzero exit, true/residual extra sections). Re-run real 30-source Copilot plugin job at final PR HEAD, preserving all not-yet-qualified statuses and any new evidence.

## Two critical reviews (self-critique, not independent clearance)

**Ownership and false-PASS critic:** A single `enabledPlugins=false` byte is never sufficient. A native CLI list might lack a version (optional in official contract), so only an independent verified UAP signed receipt binds the exact source SHA/manifest/tree; native list binds exact client registration. Require both. A foreign name, duplicate, extra managed key or failed list must keep the case non-PASS.

**Security and non-destructive cleanup critic:** Settings may encode deliberate user-level disabled preferences, including policy overrides; broad JSON editing or ignoring the whole `.copilot` directory would corrupt user data or hide active plugins. Preserve raw file and report the narrow disabled-only exception. Native command runs in disposable, case-anchored profile through an absolute prebound executable; it cannot grant authentication or bypass trust/containment. Existing non-active passive cache semantics are unchanged.

**Status:** narrow fixture-only correction and regression pushed in #698; final-head native CLI behavior, independent review and all 30 source ZIPs still unverified. No merge authorization.
