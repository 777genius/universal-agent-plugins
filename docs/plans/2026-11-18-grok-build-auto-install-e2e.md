# Grok Build automatic installation — completion and E2E

Date: 2026-11-18. Base: Grok follow-up PR #962 (depends on shared readiness PR #957).

## Goal and scope

Finish **Grok Build only** through the existing UAP client adapter, shared planner, native execution and transaction/receipt ports. A user's selected plugin or Skill should install, remain stable on repeat, update safely and remove without touching foreign files. Report Active/Ready only with trustworthy Grok discovery, never solely from copied files. No new installer, no generic framework and no changes to other agents.

## Verified starting point

- User native: `grok plugin install --trust <owned-path>`, `plugin list --json`, `plugin uninstall --confirm <name>`; already implemented. Confirm exactly selected source, version, enabled state and process closure.
- Project: managed package at `<project>/.grok/plugins/<id>`, plugin-manifest Skills and selected MCP. PR #962 adds bounded, read-only native confirmation of already trusted/enabled pure Skills.
- Explicit Project enablement has an existing limited Linux / pinned Grok 1.0.70 transaction path. Do **not** silently trust a project, mutate arbitrary `~/.grok/config.toml`, or claim MCP execution from filesystem presence.

## Architecture decisions

1. **Reuse** `clients/grok` lifecycle + registry inspector and `agentplugins/usecase` rather than duplicating installation orchestration. Domain remains unaware of Grok CLI flags; Grok adapter owns its native commands and parsing.
2. Keep native trust a separately authorized capability. Missing/unqualified capability produces honest Prepared/needs-action, not an unverified Ready.
3. Fail closed on foreign same-name plugins, native command failure, ambiguous inventory, changed file contents, incomplete process cleanup and missing ownership.
4. Preserve idempotence: no reinstall for a verified unchanged install, but do not skip an actual same-version content replacement.
5. Keep other-client files, published CLI contracts, Directory signatures, consent and ownership unchanged.

## Acceptance matrix

| Scenario | Expected outcome |
| --- | --- |
| User initial add | Real Grok plugin list confirms owned active plugin |
| User unchanged repeat | Read-only native discovery, no reinstall/uninstall |
| User changed source/version | Explicit bounded replacement and fresh discovery |
| User disabled or foreign duplicate | No forced enable/uninstall or false success |
| User removal | Exact owned native registration removed; foreign untouched |
| Project pure Skills already trusted | Bounded native Skills proof -> Ready (PR #962) |
| Project Skills not trusted/disabled | Prepared with actionable trust step, never Ready |
| Project Skills/MCP | No premature Ready from plugin file alone |
| Native probe/cleanup uncertain | No additional side effects or success claim |
| Repeated attempts, two scopes | Project/User isolation and retained foreign sentinels |

## Implementation and E2E

- First investigate any remaining *real* contract gaps after #957/#962; change only Grok-specific adapter or existing test harness affected by a proven gap.
- Add focused Go regressions and ordinary CLI/Grok E2E cases using the existing signed fixture + pinned native Grok path; inspect resulting native ZIP/original runner logs. Do not use a stub-only run as genuine E2E proof.
- Run affected tests, lint and relevant existing native workflow gates on the actual PR head; count success, failure, cancellation, queue and skip separately.
- Review the final diff in three passes: boundaries & DRY; trust/ownership/races; lifecycle UX & E2E. Fix findings and push each commit immediately.

## Integration

This is a small **stacked Draft PR based on #962**, which depends on #957. Merge #957 first, then #962 (after actual green gates), then retarget and merge this PR after its own current-head E2E. Do not bypass protected checks.

## Estimates (initial)

Expected production: ~40–160 LoC if a concrete gap is found; focused Go + harness tests: ~150–400 LoC. Complexity 6/10; reliability target 9/10; present confidence 6/10 until pinned native E2E. The initial plan is not implementation or a PASS claim.
