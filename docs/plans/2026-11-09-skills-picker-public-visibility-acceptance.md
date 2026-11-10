# Skills agent picker, VS Code public visibility and website-source E2E — 2026-11-09

## User-visible scope
1. Skills picker: do not repeat the same "CLI checks the source and Skill before installation" subtitle on every agent. Retain exactly one shared note next to the install command in both Skill card and Skill detail. Keep unique automatic-selection help and checkbox accessibility.
2. Temporarily hide VS Code from *publicly suggested* agents across the site: plugin targets, standalone Skills pickers, support matrix, homepage/download agent list, agent detail routes and sitemap. Retain its backend adapter, canonical ID, legacy persisted state, internal tests and translations; do not claim it is a newly supported automatic installer. Keep the README support list consistent with visible site.
3. Expand release acceptance beyond the previous pinned 30 plugins / 30 Skills. Add a bounded opt-in, evidence-preserving check of website-curated Skills, reviewed plugin list and Linux native Skills. Use real published npm CLI and genuine supported clients. Unknown sources, missing credentials, failed/native-unqualified installs and unavailable vendor tooling must remain explicit outcomes, never green placeholders.

## Guardrails
- No change to installer runtime, signed catalog, immutable source revisions, vendor login, or model API calls.
- Site presentation must not become install authority; dynamically discovered catalog results are untrusted and cannot become code execution without existing signature/source policy.
- Keep the normal PR smoke budget small; run deeper workflows on manual dispatch / review label or scheduled acceptance. Never block an unrelated PR on unavailable external vendor hosts.
- Do not claim 100% of a live, mutable catalog is verified from a static 30-source sample. Distinguish command eligibility, signed reviewed list, install lifecycle, native activation, and actual agent tool execution.

## E2E acceptance
- Unit + browser checks: picker option count and single shared note; no visible VS Code selectors or support/navigation link; other agents including Copilot unchanged; persisted deprecated VS Code ID cannot generate a new install command.
- Public-CLI E2E: compare live reviewed catalog names with real signed aliases and report any unqualified/missing source; real install/repeat/remove for the eligible set with pinned identities on Linux.
- Six featured Skills: assert exact GitHub repo/slug pairs, generated commands and real native paths/lock/repeat/remove; do not silently skip unresolvable sources. Cross-platform coverage already provided by PR #800 for 30 frozen Skills, supplement Linux.
- CI and review on the exact HEAD, resolve comments, merge only after proof; verify post-merge result.

## Explicit out of scope
- Removing the VS Code engine/adapter or files saved by existing users.
- Publishing a new CLI release solely for site display changes.
- Claiming real model/tool activation without authenticated vendor tests.
