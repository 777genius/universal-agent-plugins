# Marketing Kit: team restore demonstration and installation scope

Status: implementation plan · 2026-11-02

## Goal / bounded scope

Revise the landing's restore demo and the Marketing Kit catalog install journey without inventing installer capabilities. Preserve signed Directory verification, project scope authority, and native client constraints. Use the real reviewed Marketing Kit package (manifest version 0.1.0) and deterministic content.

## Verified authority

- `examples/plugins/marketing-kit/plugin.json` and signed Directory listing declare `marketing-kit` 0.1.0.
- `[dependencies]
"777genius/marketing-kit" = "0.1.0"` is the supported exact-version requirement form in `uap.toml`.
- Actual UAP target ID for Claude Code is **`claude`**; Codex `codex`, Gemini `gemini`. Never show `claude-code` as a TOML target.
- This version is *package* metadata; the lock pins exact source, publication and digest. `marketing-kit@0.1.0` as a positional `add` argument is **not proven CLI syntax**: never advertise it until independently exercised in the released CLI. Current CLI's project `add` path requires concrete project agent selection and the v1 Directory policy is user-only. Prefer demonstrably supported manifest-first project install with `agentplugins install --scope project` and `agentplugins install --locked` for teammates, without a fake target-free `add` command. In browser copy buttons, surface the full valid command only.
- A static landing page cannot read which agents are installed on a visitor's computer. Present choices as capability, not observed installation.

## UI

1. Change scope labels across all maintained locales: `This computer` and `This project` (localized). Segmented two-option control with a single selected state, clear focus ring, disabled-state semantics, no overlapping focus/copy controls. Keep unqualified packages project-disabled.
2. For Marketing Kit, project example is declarative: a preview of actual project `uap.toml` declaring `"777genius/marketing-kit" = "0.1.0"` and three target IDs `claude`, `codex`, `gemini`; do **not** default to Cursor or print Cursor-specific setup copy. Show the project install command driven by this manifest, rather than falsely claiming runtime agent discovery or user-only Directory policy as project support. Preserve truthful authentication/follow-up warnings in detailed documentation; do not falsely claim provider login is complete.
3. Restore section uses the same Marketing Kit package/version on both monitors. Labels: `My computer` / `Teammate's computer` in English, natural localization elsewhere. Second monitor artwork distinct but visually related to the first.
4. Terminal lines type progressively on entry, with a stable baseline, cursor and deterministic complete state. Do not animate inaccessible/offscreen copy text; copy complete command regardless of animation. Respect `prefers-reduced-motion`: instantly show complete content.
5. `uap.toml` preview starts readable, folds to a compact file icon after the intro, reopens with hover, focus and tap; close with Escape/outside click. Keep layout space stable and readable on mobile. `uap.lock` must remain visible; no claim the manifest alone contains its lock's digest.
6. Animations remain narrowly scoped to the demo; avoid global timers, expensive scroll loops, layout shift and new animation dependencies.

## Review and E2E gates

- Create draft PR immediately, push in incremental commits, run two independent critique passes for truthful CLI semantics, accessibility/RTL, responsive layout and transition behavior, fix all blocking findings before ready/merge.
- Extend E2E to cover all nine locales' copy where applicable; screen sizes 280, 320, 390, 540, 768, 1024, 1100 and 1440; both themes; normal/reduced motion; typing end state; copy before/after typing; keyboard/touch manifest preview; valid semantic target IDs and absence of Cursor-only selection.
- No horizontal overflow or collision (including Copy and version text), no visible fake SHA or unsupported versioned `add`.
- Verify Landing CI + assembled Pages browser checks, Required CI and reviewers on exact HEAD, then squash merge only when green. Verify Pages build **and deploy** for exact merge SHA.
- Do not claim genuine native agent activation or public deployment without specific CI/deployment evidence.

## Decision record

Keep scope narrow: no new installer command/parser behavior in a marketing landing change. For a version-pinned package select the existing, actually supported manifest/lock path. If future CLI explicitly releases `add marketing-kit@0.1.0` without `--target` under project scope, the demo may adopt it after dedicated CLI E2E; until then use manifest-first truth.
