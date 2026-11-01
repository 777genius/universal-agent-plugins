# SkillSpector: understandable checks, modal details, reliable coverage and demo playback

**Owner goal, 2026-11-01.** Check every accessible Skill as promptly as practical, explain what was and was not verified, and replace narrow inline details with an accessible dialog. Reduce Watch demo text by 1.2×, lightly blur the poster, play fullscreen and exit fullscreen at playback end.

## Verified baseline

- Cloudflare: 13/13 attempted, 10 partial. n8n: 6/15 current results, all 6 partial. These are not equivalent to ten security findings.
- V3 reports distinguish complete, partial, error, pending and historical. Never turn a partial record into a claim of complete analysis.
- Registry workflow has 8 partitions × 128 attempts and 2,100 s worker budgets per six-hour run. Roughly 17k selected roots cannot all be freshly analyzed within one such bounded run. For small affordable plugin groups, try to complete the group in one batch; show remaining backlog truthfully. Preserve resource fairness and safety.
- Registry issue #402 tracks daisyUI directory links falsely classified as missing in the pinned scanner. Phoenix ambiguous standalone names are separate from exact plugin paths.
- Static analysis cannot execute code or certify that installation is safe.

## Product language — requirements

- Card should answer what users care about: **13 of 13 checked · 10 not fully checked**; **6 of 15 checked · 9 awaiting a check**. In dialog: fully checked, checked with gaps, not yet checked, failed, previous check. Avoid the unexplained word “limited”.
- “Checked” means a current scan report exists, not that every file or analyzer was fully verified; always show the incomplete count if nonzero. Do not call a signal a confirmed vulnerability.
- Show a denominator only if inventory is known. Never promise a scan time or claim no findings means safe.
- Trigger opens wide native modal dialog, not details inline in a card. Layout: title/name and explanation, numerical status breakdown, relevant Skills and reasons, original evidence in distinctive accessible collapsible sections, provenance and optional raw technical record. Lazy-mount expanded raw content; preserve original bytes/identity.
- Enter/Space, Escape, Close, focus return, mobile scrolling, no horizontal overflow or interference with installation controls.
- Translate any added keys to all nine locales: EN/RU/UK/ZH/ES/HI/AR/PT/FR.

## Demo

- Divide current demo label font size by 1.2; play button size is unchanged.
- Only blur the background poster slightly. Keep label/button sharply rendered.
- User action requests fullscreen for the video container and starts playback; media end exits fullscreen. Mux iframe is cross-origin: do not assume the parent can read its DOM or ended event. Use a documented Mux player API/event or a safe same-origin video control with fallback. Denied fullscreen must still play normally; Esc should work.
- Cover keyboard, responsive layout, player boundary, fullscreen denied and media-ended cleanup; keep the optional real Mux playback test.

## Registry follow-up as a separate PR

- Analyze immutable reports for Cloudflare/daisyUI/Phoenix/n8n; partition root causes into reference, parser/budget, retry, unavailable discovery and queue capacity.
- Improve bounded per-plugin fairness so small reachable groups can complete together when capacity permits. Keep exact attempted vs waiting evidence and explicit blockers. No unbounded increase in parallelism or claim that all 17k can fit a 35-minute run.
- Fix directory false-positives at pinned upstream/contract boundary with traversal/symlink tests and a coordinated schema/pin rollout, never by rewriting the displayed report.
- Resolve ambiguous names only with exact source identity; retain unknown when still ambiguous. Validate publication signatures, digests and E2E.

## Delivery / exit gates

1. Immediately commit this plan to a draft landing PR, then implement a focused component/CSS/i18n/test diff.
2. Validate scenarios: Cloudflare 13/13 (3 complete, 10 partial), n8n 6/15 (6 partial, 9 pending), daisyUI 1/1 partial, Phoenix 2/5 if current published evidence supports it; stale, error and unknown inventory; EN/RU desktop/mobile, focus, Escape, raw report integrity and video events.
3. Three critique lenses with findings/fixes recorded before ready-for-review: UX/accessibility; security and evidence fidelity; runtime performance/maintenance. Review requests to connected reviewers are supplementary, not a pass.
4. Merge only after latest-head CI and blocking reviews succeed; verify main deployment SHA, live browser evidence and screenshots.
5. Independently open Registry draft PR for queue/scanner work, verify its tests and report publication, then merge on green. No hiding pending counts.

## Three critique rounds before implementation

**UX critic:** “Checked” could sound fully verified. Correction: explicitly quantify gaps, explain what checking covers and avoid “needs attention” where there is no confirmed security problem. For unknown inventory, never show an invented total.

**Security critic:** A modal may render enormous untrusted raw evidence. Correction: lazy mount, bounded source strings and conditional raw records; no HTML interpretation; exact original JSON when expanded; no severity promotion.

**Operations critic:** Owner requests all accessible Skills in one go but bounded GitHub runner capacity cannot guarantee it. Correction: prioritize affordable coherent plugin batches and measurable progress, keep pending honest, do not silently change worker budgets or promised times.

## Exclusions

No security certification, new feature flags, unbounded parallelism, report-schema change purely for text, fake completed checks, installer changes or work on frozen VS Code integrations.