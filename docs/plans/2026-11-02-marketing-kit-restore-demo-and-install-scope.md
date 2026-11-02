# Marketing Kit: versioned restore demo and install scope UI

Status: implementation review · 2026-11-02

## Goal

Show a real Marketing Kit 0.1.0 dependency environment that can be reproduced on a teammate's computer, without inventing version-selector syntax, project permissions or detected agents. Deliver the new segmented scope UI, two visually distinct monitors, terminal typing, accessible morphing `uap.toml`, and responsive/i18n QA.

## Verified source and scope (blocking policy fact)

- Official `plugins/marketing-kit/plugin.json` is version **0.1.0**. Its `uap.toml` requires Notion 0.1.0, HubSpot CRM 0.1.0 and Playwright 0.0.80.
- `777genius/universal-agent-plugins-registry/registry/directory.json` publishes the Marketing Kit release and dependencies with signed target scopes **user only**. The CLI's `deps_resolver.go` passes `Scope: app.scope()` to `ResolveDirectory`; `domain/directory.go` refuses a release without the requested scope. Therefore `[dependencies] "777genius/marketing-kit" = "0.1.0"` with `install --scope project` is **not runnable** today. Never show a copyable project install command for this package.
- The real supported exact-version syntax is a user-environment `uap.toml`, not an unverified `add marketing-kit@0.1.0` positional selector. The rendered `[targets]` are `claude`, `codex` and `gemini`; they are manifest declarations **for installed compatible clients**, not browser-side machine detection.
- To restore the locked user environment, use `agentplugins deps export`, copy both `uap.toml` and `uap.lock`, then `agentplugins deps import --install` on the teammate's machine. This flow is documented in `docs/UAP_DEPENDENCIES.md`. Account authorization, credentials, installed agents and native activation still require separate steps.
- The install UI offers a readable `This computer / This project` segmented control; for Marketing Kit the project choice is explicitly disabled with the existing unavailable reason. This prevents the previous misleading implicit Cursor selection. Other project-qualified products retain their existing paths.

## Presentation

1. Use `marketing-kit` and `v0.1.0` on both monitors, with a differently accented teammate monitor and original twilight wallpaper.
2. Type copyable complete user-environment install and teammate import commands, plus the export command in the handoff. Visible line wrapping must not change copied bytes.
3. Preview valid TOML with `"777genius/marketing-kit" = "0.1.0"` and three canonical target IDs. Auto unfold on first intersection with the viewport; fold back into a stable file icon; hover/focus/click/tap reopens, Escape/outside click closes.
4. Respect `prefers-reduced-motion`, keyboard focus, Arabic RTL switch positioning, mobile screen collision constraints and full source/credential boundaries.
5. Scope UI must never claim to know which agents the visitor has installed. The CLI, not the landing page, verifies their availability and authorization.

## Tests / acceptance

- Browser E2E: 280, 320, 390, 540, 768, 1024, 1100, 1440px; EN/RU/FR/AR; both themes; no page overflow or Copy overlaps.
- E2E: exact command clipboard; package/version matches across both machines; visible-on-scroll typing and manifest morph; keyboard/touch, Escape and reduced motion.
- E2E: project option disabled for reviewed Marketing Kit with user-only policy, and saved invalid project intent never falls back to a copyable user command nor shows a Cursor picker.
- CI stores real browser screenshots for desktop, mobile and the expanded manifest. Review these artifacts, correct visual defects and rerun CI on the final SHA.
- Critic/security review with no unresolved findings; successful Required and Landing (including both Pages assembly bases); merge, then Pages build and deploy on the exact merge SHA.

## Future, separate policy work

Full Marketing Kit **project** support requires a qualified new signed Directory project policy for the package and all mandatory dependencies, a compatible published CLI, real native E2E and approval of any accepted ADR changes. Do not bypass user-only policy in a landing change, mutate a signed v1 target scope or silently substitute a skills-only package.
