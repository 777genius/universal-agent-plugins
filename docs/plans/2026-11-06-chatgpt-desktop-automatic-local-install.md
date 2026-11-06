# Plan: reliable one-command ChatGPT Desktop local plugin installation

Status: reviewed design; implementation and physical Desktop qualification tracked separately.
Date: 2026-11-06. Target: `feat/chatgpt-desktop-autoinstall-20261106`.
Scope: **personal / this computer / local Codex**. Do not touch ChatGPT Web or organization Admin > Plugins.

## Goal and non-goals

A user choosing an eligible signed plugin in UAP runs **one existing command**:
`npx -y universal-agent-plugins add <plugin> --target codex`.
When the supported Codex CLI exists, UAP must register its owned local marketplace, install the plugin through the native CLI and verify installed/enabled state. Native state and files must survive restarting the UAP process. ChatGPT Desktop shares the local Codex plugin mechanism; the user should not be told to install again after successful native installation. On a first Desktop session, an app restart or provider consent can still be required. Do **not** claim a real Desktop plugin/tool invocation until verified on an authenticated Mac/Windows desktop.

No new Desktop client ID, installer engine, default feature flag, custom configuration writer, package downloader, app UI automation, OAuth bypass, forced app launch, app-store install, or second state database. Never overwrite foreign marketplace/config entries. Do not change VS Code or cloud `chatgpt` app-binding semantics. No public release in this PR.

## Facts / current implementation

- `clients/codex/lifecycle.go` already runs `plugin marketplace add`, `plugin add ... --json`, then `plugin list --json` with exact profile custody; no new installer is needed.
- The app and CLI share the local Codex plugin mechanism; an installed plugin is loaded from a versioned `~/.codex/plugins/cache` copy. Official sources: https://developers.openai.com/plugins/build/plugins and https://learn.chatgpt.com/docs/developer-commands.
- `clients/codex/profile.go` chooses `CODEX_HOME` or `~/.codex` from the detected host. `codex/command.go` explicitly binds the chosen root; a custom root is legitimate for Codex but **does not prove Desktop uses it**.
- `clients/codex/desktop_handoff.go` currently gives a manual “enable in Plugins > Personal” instruction even after verified native CLI installation. The `codex://` link opens only a detail page, not an installer.
- The existing PR #724 native matrix passed nine Codex scenarios on Linux, macOS and Windows. Its success is Codex CLI proof, not authenticated Desktop proof.

## Implementation — deliberately narrow

1. **Keep the real installer** in `codex/lifecycle.go`. Do not duplicate marketplace installation or force local files into app-owned cache. Treat `plugin list --json` installed+enabled as native success, and retain current typed refusal when the listing is missing/unknown.
2. **Correct post-install guidance** in `codex/desktop_handoff.go` / `lifecycle.go`: after a successfully verified native add, state that native Codex installation is complete and persistent in the selected profile; open/refresh Desktop and start a new local session to verify actual use. Do not ask for a second installation. Manual path without CLI still requires manual completion. Preserve the existing short-URI and 390/500-rune truncation safeguards, path/mode validation, zero token leakage and the native Codex first next action.
3. **Handle profile ambiguity honestly:** if the native profile is not the local default `~/.codex` (e.g., custom `CODEX_HOME`), do not claim the signed-in Desktop installation is ready. Preserve explicit profile selection without writing into or silently switching to a second profile. Show a short, safe warning and the selected-profile CLI proof.
4. **Update the site** `ChatGPTDesktopGuide.vue`, corresponding nine maintained locales and the local installation guide to say “automatic via existing Codex CLI” with conditional fallback; do not represent web ChatGPT as supported or offer browser-only one-click native installation.
5. **Targeted tests:** native verified/missing/unknown CLI; default vs custom profile; idempotent add/verify and fresh process list; no-change/repair/remove; foreign/symlink/long URI protections; site command copy at 390/1440 and both themes. Reuse existing native matrix and focused browser regression, not a new harness.
6. **GitHub verification:** `Required` (including `test-core`), lint/CodeQL/security, native Codex Linux/Windows/macOS, `Landing`, two Pages bases. Fix concrete logs on the exact head; two review passes for security and UX; then guarded merge if all required checks finish green. Do not trigger duplicates.

## Acceptance / explicit status boundaries

- **Native-installed**: signed exact package, manager-owned marketplace and plugin, `codex plugin list --json` says installed/enabled in the exact selected profile, repeat UAP process sees the same state.
- **Needs manual setup**: no supported CLI or unverifiable native state, with a clear next action but no fake success.
- **Desktop-verified**: only after a signed-in physical ChatGPT Desktop session actually discovers/uses the plugin. CI cannot attest this. A local URI is convenience only.
- Uninstall cleans UAP-owned entries; foreign entries and unrelated local config remain byte-unchanged.
- The custom `CODEX_HOME` case never declares Desktop-verified status.

## Physical MacBook acceptance (owner later)

1. Open and sign in to the **ChatGPT Desktop app** with local Work/Codex and check that the Codex CLI is available: `command -v codex; codex --version`. Use the same macOS user as Desktop, with no temporary `CODEX_HOME` override.
2. In a Terminal on that Mac, run the real UAP command copied from a currently eligible plugin in the public directory, e.g. `npx -y universal-agent-plugins add context7 --target codex`. Record the final native status, no secret values.
3. Run `codex plugin list --json`. Fully quit and reopen ChatGPT Desktop. Inspect **Plugins > Personal** and open a new local Work/Codex conversation. Verify the installed plugin is enabled and actually invokes its Skill or MCP tool; complete provider OAuth only if requested by the app.
4. Repeat the UAP add command and check that it is idempotent, then `update` / `repair` if needed. Check another `codex plugin list --json` from a fresh Terminal process.
5. Only with explicit consent, test `remove context7 --target codex` and confirm the managed entry disappears from the CLI and the refreshed Desktop without disturbing unrelated plugins. If a step fails, collect UAP status and **redacted** CLI listing, not tokens or local private paths.

## Two critical review rounds — fixes incorporated

**Round 1 — architecture/security critics.** Rejected a separate `chatgpt-desktop` adapter and direct writes to `~/.codex/plugins/cache`: they duplicate the official installer and break ownership. Rejected auto-launch, synthetic app clicks and OAuth automation. Added selected-profile fidelity, persistent process verification, idempotency and exact native status gates. **Round 2 — UX/E2E critics.** The original plan falsely implied that a successful CLI listing proves Desktop activation; it does not. Split native success from physical verification, removed redundant “install again in Desktop” guidance after native success, made missing-CLI and custom profile fallback explicit, kept one-copyable-command UX. Retained genuine cross-OS native proof; physical Mac is a separate owner acceptance rather than a mocked pass.

## Bounded effort

Expected ~200–450 changed lines including nine locale strings, tests and docs **if no unexpected vendor behavior is found**. Do not pad to quota; stop and revise this plan on real native/Desktop incompatibility. No extra check framework or schema.
