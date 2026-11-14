# ChatGPT Desktop local marketplace — implementation plan

Status: 2026-11-05. Scope: personal/repo local marketplaces in ChatGPT Desktop; workspace distribution is guidance only. Existing branch: feat/chatgpt-desktop-local-marketplace-20261105. Separate from PR #722.

## Product goal

A UAP user browsing an eligible Agent Plugin can choose a clearly labeled **ChatGPT Desktop (local)** path, copy a working CLI command, and open the exact local plugin card in ChatGPT Desktop with minimal manual navigation. A workspace admin can use the already supported GitHub marketplace format, without fictitious account or cloud automation.

1. Personal flow: install via existing `npx -y universal-agent-plugins add <plugin> --target codex`, which manages Codex's local plugin marketplace and can verify `codex plugin list --json`. Show a deterministic local-desktop handoff link and steps to refresh ChatGPT Desktop, select Personal and finish installation/enablement in the app.
2. Project flow: use existing `--scope project` only where the active Codex release supports it. Do not claim this implies a ChatGPT Web installation or a globally available marketplace.
3. Organization flow: document official Admin > Plugins GitHub marketplace import/sync and access policies. UAP cannot bypass workspace-admin roles or OAuth prompts.
4. Cloud flow: `--target chatgpt` remains **unchanged** and gated by verified app binding; it must never silently alias to Codex.

## Platform contracts (as checked 2026-11-05)

- https://developers.openai.com/plugins/build/plugins : repo marketplace `$REPO_ROOT/.agents/plugins/marketplace.json`, personal marketplace `~/.agents/plugins/marketplace.json`; ChatGPT Desktop displays local sources and installs copies into `~/.codex/plugins/cache`. Local marketplace discovery is not installation, activation or runtime verification.
- https://learn.chatgpt.com/docs/cli/reference : `codex plugin marketplace add/list` and `codex plugin add/list` handle local sources and installation.
- https://learn.chatgpt.com/docs/app/commands : `codex://plugins/<plugin-name>?marketplacePath=<absolute-marketplace-json-path>` opens a local plugin detail, not unattended installation. No automatic app launch by default.
- https://help.openai.com/en/articles/20001504-importing-and-syncing-plugin-marketplaces-from-github : workspace GitHub import, sync, policy and role access are administrator operations.
- https://developers.openai.com/plugins/deploy/desktop-only : local execution requires the desktop app; public desktop-only MCP submissions have additional restrictions.

## Code authority / architecture decision

- `install/integrationctl/agentplugins/clients/codex` already owns the local marketplace registration, installation, status, update/repair and removal via a separately validated CODEX_HOME, managed physical artifact IDs, and native CLI verification. Reuse it rather than create a new `chatgpt-desktop` ClientID, state store, configuration writer or installer.
- `install/integrationctl/agentplugins/clients/shared/marketplace.go` owns deterministic managed namespace naming.
- `cli/internal/publicationexec/generate.go` already emits `.agents/plugins/marketplace.json` for authoring. Do not introduce a duplicate renderer into the installer.
- `install/integrationctl/agentplugins/clients/chatgpt` represents remote ChatGPT with registered app bindings. **Never weaken this contract.**
- `landing/components/registry/InstallPanel.vue`, `RegistryPluginCard.vue`, `landing/utils/commands.ts` and `landing/composables/usePluginInstallSelection.ts` own source-resolution and selection. Avoid a separate command builder or capability allowlist.

### Implementation (bounded)

A. Add a **pure** local-desktop deep-link helper under the existing Codex adapter. Input: validated managed active marketplace root and plugin name. Validate absolute, clean, control-free paths and safe plugin identifiers, percent-encode name and JSON path. No shell interpolation, arbitrary path traversal, OS-specific direct configuration edits or UI auto-open. Include optional next-step output only when a user-scoped local Codex package can be verified; do not turn local Codex verification into ChatGPT runtime proof.

B. Add a small separate ChatGPT Desktop (local) help/CTA in the existing installer and on the client information page when an **active user-scope Codex** distribution exists. Reuse the existing install command for `--target codex`. Keep the cloud ChatGPT option visibly unavailable when app binding is missing, with a link/explanation rather than enabling a broken command. Dark/light theme and 390px / 1440px layouts use existing tokens.

C. Document one clear personal installation, update, repair, remove flow with troubleshooting (no Codex CLI, desktop app not running, not visible after restart), optional local deep-link, and separate organization admin instructions. Do not promise a ChatGPT Web installation, project-scoped ChatGPT cloud support, or unattended workspace OAuth.

D. Translate new copy across the nine maintained languages. Avoid growth that violates static HTML payload budget; do not weaken the budget to pass.

### Out of scope

No new persistent database, SDK API, feature flag, installation engine, desktop UI automation, Cloud/Workspace API or sign-in harvesting; no native VS Code changes; no public release without its own qualification. Do not modify native ChatGPT remote app binding semantics.

## Security and failure behavior

Preserve existing installer source trust, signed release resolution, path containment, symlink and foreign-file policies, rollback and receipts. A generated link is informational only: URL-encode untrusted bytes, reject malformed names and noncanonical roots, never auto-launch external URI schemes. Never echo OAuth tokens, account identifiers or private paths in remote links/logs. Codex listing verifies only the native CLI contract. Any missing CLI or unknown listing becomes an explicit manual step, not green runtime status. Workspace admin import requires the administrator.

## Quality and E2E evidence ladder

| Gate | Actual evidence | Acceptance |
|---|---|---|
| Pure | Table-driven encoding / invalid path / reserved characters / null/Unicode / relative path tests | No injection, deterministic URL |
| Native | Existing Codex lifecycle tests with real subprocess fixture | Marketplace add, plugin add, list, update, rollback, remove, CODEX_HOME isolation |
| Synthetic E2E | CLI with disposable HOME + fake but executable Codex CLI, checked ownership and no mutation on refusal | End-to-end local managed transaction; never claim real Desktop |
| Web | Playwright 1440/390, light/dark, source support matrix, scope changes, disabled ChatGPT cloud, copy commands, no overflow | Honest UX |
| CI | make lint + test-core for Go code; Nuxt typecheck/lint/registry/i18n, generate, both Pages bases, Playwright | Exact-head required checks green |
| Physical Desktop | Signed-in ChatGPT Desktop install from local marketplace, actual local Work session uses skill/MCP | Record **NOT RUN** if machine/app unavailable |

## Pre-implementation critical reviews

**Architecture/security critic:** Adding a new ChatGPT client ID duplicates local Codex semantics and risks bypassing remote binding policy. Decision: current Codex client is the transport; separate UX label only. Validate path and identity before displaying URI, never mutate config manually.

**UX critic:** A user could read “installed in ChatGPT” after Codex CLI `plugin list` even when ChatGPT Desktop still requires a restart and confirmation. Decision: label each boundary explicitly (prepared / registered / finish in desktop / verified only by manual runtime test), and use an optional deep link.

**E2E critic:** A synthetic CLI stub cannot prove desktop execution. Decision: test the native transport and UI extensively; separately document a physical macOS/Windows runtime checklist; mark any unrun portion honestly. CI failures and bypassed/queued checks are not PASS.

### Critical review round 2 (2026-11-05): concrete corrections

1. **Native runtime critic:** Codex CLI's `plugin add/list --json` proves Codex registration only. It cannot prove the ChatGPT Desktop app loaded or ran the plugin. **Fix:** display CLI-native status and a separate optional desktop follow-up, never the phrase "ChatGPT installed" without physical evidence. The optional `codex://` link opens the plugin detail, not install completion.
2. **Ownership critic:** Do not touch `~/.agents/plugins/marketplace.json` directly: an existing user or admin may own it. UAP's current scoped Codex adapter manages its own unique marketplace and rollback. **Fix:** build link from the adapter's already-managed active marketplace root; validate identity and refuse to invent a link if the file is missing.
3. **Portability critic:** A `codex://` URL cannot guarantee launch on every OS; Windows and Linux filesystem/URI handling may differ. **Fix:** URL generation is a pure helper; automatic launch is explicitly not part of V1. Add portable fallback instructions that work without a custom URI handler.
4. **Publication critic:** `publicationexec` creates an authoring catalog, while the consumer adapter already creates its own managed release. **Fix:** do not invoke authoring publication from the installer. Share format and tests, not stateful responsibilities.
5. **UX critic:** Only expose Desktop guidance when the *selected* install distribution actually supports Codex in user scope; avoid turning a disabled cloud ChatGPT checkbox into a selectable unsupported target. Keep existing single source of truth for resolution and CLI commands.

6. **Codex-first UX critic:** A new Desktop URL must not replace the ordinary Codex CLI's first successful next action. **Fix:** native guidance explicitly starts with 'start a new Codex session'; Desktop is clearly optional and never launched silently. A subprocess integration assertion preserves this ordering.\n\n7. **CLI result critic:** The ordinary success branch printed only "Installed and verified" and dropped adapter `LocalActions`; the Desktop URI was therefore not discoverable after a successful automatic install. **Fix:** the generic human CLI renderer now shows the existing first optional local action after verified success or no-change. The public JSON still excludes local paths, and the Codex native status remains verified independently. Added coverage for both fresh and repeat installs.\n\n**Decision after review:** implement the existing Codex install lane plus truthfully labeled optional Desktop handoff, one UI guidance component and small docs. No second client, no user-file overwrite, no new persistent state. Full native Desktop E2E is blocked without a signed-in physical desktop; state this explicitly in review and release evidence.

### Review resolutions

One additive surface on top of existing Codex, pure helper, no new adapter/DB, minimal text and no feature flags. Independent external reviewer feedback, if available, is tracked in the PR and fixed before merge. If later evidence disproves shared local state, stop and revise rather than fake compatibility.

## Execution order and done criteria

- [ ] P0 Check AGENTS.md / ARCHITECTURE.md / CONTRIBUTING.md, current main and overlapping PRs.
- [ ] P1 Commit this Markdown plan; complete risk/UX/test critique and revise before code.
- [ ] P2 Implement native helper and precise CLI guidance; test negatives and lifecycle.
- [ ] P3 Implement eligible site UX and docs/locales using existing source resolution.
- [ ] P4 E2E on exact commit: native and browser, responsive/dark-light, signed feed, both Pages builds.
- [ ] P5 Address independent review, push fixes, ensure required CI success.
- [ ] P6 Squash merge only the reviewed exact head; verify main after merge. Report desktop-real E2E separately.

The implementation PR cannot claim ChatGPT Desktop runtime testing in an environment without a signed-in desktop app. Such testing remains a required external qualification, not silently promoted from CLI-fixture evidence.
