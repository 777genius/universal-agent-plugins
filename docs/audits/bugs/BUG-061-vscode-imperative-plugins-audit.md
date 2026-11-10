# BUG-061 — VS Code plugin E2E attempts unsupported saved add

## Reproduction and root cause
Original final-head VS Code Plugins ZIP #13423014074 used real SHA-256-verified VS Code CLI 1.144.0, but all 30 attempted adds exited nonzero: `target vscode cannot be saved; retry ... --no-save`. The CLI's `routeSavedAdd` intentionally refuses saved dependency targets for VS Code while preserving the imperative `--no-save` path. This is an incorrect **shared E2E command route**, not a reason to change the frozen VS Code adapter. See PR #698 discussion #6660709205.

## Minimal correction and two critical passes
- **Correctness:** append `--no-save` only for `target=vscode, kind=plugin` in the shared audit. Do not alter its `list`, repeat, removal, native attribution, or foreign-sentinel checks; test that the Codex plugin path remains saved and unmodified.
- **Security:** no new saved authority, feature flags, or changes to production VS Code code, dedicated fixtures, source trust, native process custody or manual activation classification. An imperative prepared package still needs truthful native proof to be `passed`.

## Acceptance
Run focused no-network Python and current-head Required/Core. Qualify original 30-case VS Code Plugins ZIP on the new SHA; do not count package preparation/manual activation as native PASS.
