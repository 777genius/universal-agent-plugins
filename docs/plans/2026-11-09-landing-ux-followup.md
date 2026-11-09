# Landing follow-up: alignment, Skills ordering, and video sound

Date: 2026-11-09

## Scope

User feedback after #798:
- Align icons and agent names to the start edge of the first support-table column (respect RTL).
- Keep SkillSpector's "No Skills to check" status height/vertical spacing consistent with scan results.
- Present editorial Skills at the start of **the existing SkillCard grid**, not a separate curated section.
- Begin the explicitly clicked main Mux demo with sound, not Mux's `autoplay=any` fallback that silently mutes playback.
- Clarify ChatGPT Desktop/Web: the separate existing PR #773 already implements this accurately and must not be copied here.

## Technical decisions and constraints

1. Matrix: use a common logical start alignment for header and row-label cells; do not change content or support claims.
2. Assessment: match the disclosure's 32 px line-box without changing verdict/status/accessibility; keep no-Skills informational, not an interactive fake scan.
3. Skills: a **pure source-ID-based stable projection** of actual API listing data on unfiltered Popular only. Do not invent install counts or SkillListing entries, do not request extra APIs, do not reorder searches/Trending/Hot/scan-filtered results, and preserve upstream pagination. Remove the extra curated UI and its CSS. The subset of editorial candidates present in the currently fetched provider page appears first; missing candidates are not fabricated.
4. Video: Mux documents `autoplay=any` as "try sound then silently fall back to muted". After an explicit Watch demo click use normal `autoplay=true`, `muted=false`, and `no-muted-pref=true` if supported by Mux iframe attributes; keep idle 7s preload **without playback**. Browsers can still block audible autoplay in an iframe; do not promise guaranteed sound unless verified in real browser. Preserve end/close/fullscreen.
5. ChatGPT: PR #773 distinguishes Desktop local (Codex backend) and Web remote (signed app binding), and is already Ready with CI in progress. CLI 0.1.102 is published, but website PR #773 is not merged as of this plan. Do not falsely mark remote Web as verified. Treat #773 as separate rollout dependency.

## Review/acceptance

- Read-only current repo and upstream contract review; changes limited to the components/data/tests involved.
- Unit tests assert exact publisher+slug identity, stable tie order, missing picks unchanged, and search/filter/trending unchanged.
- Browser tests assert 1440/390 matrix alignment, no-Skills vs scanned min-height, direct SkillCard grid with curated records first and no additional featured block, no duplicated cards, normal installation commands.
- Mux browser tests assert no autoplay while idle and unmuted play attempt after click; preserve fullscreen, closure, forged event rejection.
- Verify Vue typechecking, i18n, lint, generation, full Playwright and both Pages base paths. Keep Draft until CI/E2E confirm.
- Coordinate #773 independently; do not merge it while its exact-head gates are pending.

## Complexity (subjective 1–10)

| Piece | Complexity | Reliability | Estimated LOC |
| --- | --- | --- | --- |
| Support matrix | 1 | 10 | 6–25 |
| SkillSpector spacing | 1 | 10 | 5–25 |
| Skill source-qualified ordering | 3 | 9 | 60–160 |
| Demo audio attempt and tests | 3 | 7 (browser policy) | 20–80 |
| ChatGPT #773 follow-through | separate PR | requires verified Web/desktop boundary | no duplicated LOC |

No changes to signed Directory, installer, CLI, or external provider APIs.
