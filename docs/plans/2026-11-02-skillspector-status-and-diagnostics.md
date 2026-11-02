# SkillSpector status coverage and shareable diagnostics — 2026-11-02

## User acceptance
- Every **rendered Skill listing**, its detail page, and each plugin catalog card must make the SkillSpector state visible, including loading, unsupported source, no published scan, previous-only results, partial, and failed. **Never convert unknown to a successful scan.** A plugin genuinely without Skills should say that explicitly, not silently omit the badge.
- The SkillSpector details modal must be easier to skim, without horizontal-rule-style section separators.
- A clear **Copy diagnostics for AI** action exports **all published evidence relevant to the selected listing** as stable, readable, versioned JSON. It must include source identity, scan/publication metadata, actual status, coverage, original records/findings/reasons/attempts/references/ledger diagnostics, and explicit limits. It must not include unrelated repositories' records or imply a security audit. Failed/unavailable and unsupported sources should still be exportable with their unknown status.
- Clipboard failure must be clear, not a false “copied”. Keyboard access, focus return, dialog close, compact mobile layout and localization across nine existing locales must remain working.
- Preserve strict validated report loading, immutable digests, bounded network access, and lazy mounting of modal evidence. No new scan calls, API changes, feature flags, or fabricated producer statuses.

## Engineering plan
1. Audit all Skill and plugin render paths and their source identities; fix silent omission of the badge at the UI boundary without implying that a non-Skill plugin has been scanned.
2. Add a pure diagnostic-export formatter at the display boundary, with explicit schema, subject-scoped original record data and truthful report/scan-availability markers; unit-test no-report, checked/partial/stale, and exact record fidelity.
3. Add localized copy action with error acknowledgement, accessible live region, cleaner modal summary/sections, and responsive affordances. Remove separator-style borders; use spacing and subdued surfaces instead.
4. Add end-to-end browser tests for all cards, scan states, copy fidelity, clipboard failure, mobile and desktop. Extend live Chromium acceptance if low-risk to validate copy on the **published** site.
5. Critique in three lenses (security/evidence, UX/accessibility, performance/maintenance) and resolve concrete findings; execute CI, keep PR draft until reviews and latest-head gates, merge on green, verify post-merge publication and browser smoke.

## Safety / limits
- **No guarantee of actual scans for every catalog Skill.** Scanner capacity and source support are separate from displaying a truthful state on every listing.
- Do not silently match a provider's unverified name hint to a different source path or revision to improve coverage percentages.
- Clipboard contents are user-triggered, never sent to another service by this UI; on failure give actionable feedback.
- Published report groups can be truncated by the producer: explain that export covers **all published** diagnostics available to this listing, not missing unpublished evidence.

## Independent critique checkpoints
- Security: no unrelated report leakage, no fabricated success when no source exists, no unsafe HTML rendering, no fallback that copies without user consent.
- UX: every card visibly communicates status, copy action visually primary enough but does not compete with install; mobile copy and close reachable, explicit success/failure announcement.
- Performance: no eager enormous export serialization while cards render, no per-card heavy payload, lazy dialog remains intact, exact cache sharing across repository cards.
