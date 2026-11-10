# PR #815 E2E addendum — native lifecycle and MCP

## Scope

Keep production ownership and activation policy unchanged. The public grouped repair contract refuses a modified package; it can recover the recorded package when its complete owned directory is absent. The current native smoke incorrectly expects a partially deleted package to be repaired.

## Changes

1. Keep the partial Skill deletion and require the public repair command to refuse it with the established ownership error. Compare the remaining package, source and foreign sentinel before/after refusal.
2. Remove only the disposable fixture's exact-owned complete package directory, then require public repair to restore the exact recorded manifest, Skills and MCP projection.
3. Add portable stdio and Streamable HTTP MCP declarations to both native fixtures. Assert the native `mcp_config.json` transport mapping, package/data path expansion, and selected helper bytes, then run the pinned real `agy` strict validation and isolated import/list/uninstall.
4. Verify User/Project isolation and cleanup as before. Prepared remains the expected status; native model authentication and live MCP execution are not attested.
5. Validate script syntax locally; use the final-commit Linux/macOS/Windows native CI as the execution oracle.
6. Reject conflicting or malformed `url` / `serverUrl` aliases during compatibility import, before native files are written. Preserve exact single-alias destinations and add public projection regressions.
7. Integrate current `main`, preserving its public VS Code exclusion and updated native fixtures. Retain the already generated 17-client SDK matrix; do not edit generated data by hand.
8. Keep Hermes and Antigravity out of public install actions until a qualified CLI release contains them. The latest published GitHub CLI release, `agentplugins-v0.1.102`, has neither target. Reuse the existing public-client exclusion list, retain source metadata/translations, and keep the 14 published support rows synchronized with README.

## Estimate

Complexity: 3/10. Reliability: 8/10 pending native CI. Scope: approximately 150–220 lines of native test changes plus a small endpoint guard and its focused regressions. No production safety exception, new installer state or release is introduced.
