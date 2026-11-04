# BUG-035 — Copilot removal evidence does not distinguish enabled from disabled residual keys

## Reproduced evidence and limit

The real [Copilot plugin 30-source JSON artifact](https://github.com/777genius/universal-agent-plugins/actions/runs/41814723215) on historical source HEAD `97378eac` includes **23** `cleanup-failed` outcomes after successful UAP remove. Sanitized snapshots show one exact `enabledPlugins[`*plugin@agentplugins-12hex*`]` key remaining under the selected disposable user profile, even though the generic list/managed receipt was gone. The current read-only reporter counts matching keys but does not record whether the native value is `true`, `false`, or malformed. It is unsafe to infer auto-reinstallation, active configuration, harmless disabled residue, or proper native cleanup merely from the presence of that key.

GitHub's native Copilot documentation describes the `enabledPlugins` section as a user/repository enablement configuration, and an older vendor issue #3268 reports a stale settings key after a *different* marketplace removal command. Neither vendor document independently proves the root cause for UAP's Copilot 1.0.105 behavior.

## Minimal diagnostic plan

1. Extend **only** the existing `copilot_settings_sanitized_shape` read-only report with aggregate `enabled_plugin_value_counts` for **exact** case-insensitive `<selected identity>@agentplugins-<12 hex>` keys directly below the top-level `enabledPlugins` object. Values are bucketed as boolean true / boolean false / non-boolean, without serializing any key/value or native JSON.
2. Keep all existing path/symlink/size checks, bounded traversal, and fail-closed `cleanup-failed` classification unchanged. Ambiguous, duplicate or malformed files do **not** qualify removal. Do not edit vendor settings or change the product adapter.
3. Add a no-network canary regression for exact qualified names and true/false/nonboolean, unrelated keys containing credentials, missing `enabledPlugins`, and strict no-secret output. Run targeted Python tests on native audit CI OSes and compare final-head real Copilot 30-source JSON.
4. Only after exact evidence and independent review, plan a **separate** minimal product remedy if a live auto-enable key is owned and remains. Avoid generic native settings rewriting, implicit deletion of foreign keys, bypassing user policies or promoting removal to PASS on a string match.

## Two independent critical perspectives (author's plan challenge, not external review)

- **Security/ownership critic:** A settings key containing an identity substring can be foreign. Count only an exactly managed `@agentplugins-<12hex>` key, never emit native names, paths, or values. No automatic file mutation is authorized.
- **Evidence critic:** `false` still means native configuration drift and does **not** automatically equal safe, complete removal. This diagnostic merely distinguishes potentially auto-restoring `true` from a disabled or invalid remnant; the acceptance gate remains strict.

**State:** real residual-key evidence confirmed historically; additional sanitized value counts, current-head CI and native interpretation pending.
