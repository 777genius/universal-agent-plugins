# Consistent CLI installation review and progress

Status: implementation plan; follow-up to PR #932.

## Goal and scope

Make the first plugin/Skill installation understandable from startup through
review, per-agent application and the final result. Fix the reported Claude Code
project error without weakening native-state validation or claiming an incomplete
installation succeeded. Implement and verify this plan after publishing it.

In scope: plugin acquisition status, compact Skills confirmation, per-agent Skills
progress, accurate partial results, the reported Claude project-list failure.
Out of scope: releases, registry policy, model calls, changes to VS Code adapters,
protocols, configuration or dedicated tests, a new TUI framework, installation
parallelism, deleting user state, or automatic retries after partial application.

## Estimates

Approximate changed production lines and focused test lines, excluding this plan.
Complexity is implementation difficulty; reliability and confidence are /10.

| Change | Production LOC | Test LOC | Complexity | Reliability | Confidence |
|---|---:|---:|---:|---:|---:|
| Compact grouped Skills review + optional paths view | 130–200 | 140–220 | 4 | 9 | 9 |
| Plugin acquisition/detection activity, warning-safe output | 70–105 | 30–60 | 3 | 9 | 9 |
| Per-agent Skills progress using shared progress board | 180–260 | 160–240 | 6 | 9 | 8 |
| Claude project-list diagnosis and minimal correction | 35–70 | 70–130 | 3 | 9 | 10 |
| Actionable final success/partial/failure summary | 40–75 | 50–90 | 3 | 9 | 9 |
| **Total** | **455–710** | **450–740** | | | |

These are estimates, not required line-count targets. Prefer smaller changes when
the evidence supports them; keep final counts separate from estimates.

## Architecture and decisions

1. Reuse `terminaltheme`, Huh/Lip Gloss, `startCommandActivity` and the existing
   `groupProgressBoard`. Do not build a second renderer or lifecycle engine.
2. Keep reviewed authority, policy checks, environment locking, native receipts,
   immutable-byte checks and application ordering unchanged. Progress observes
   real prepare/apply/results; it must never drive the transaction.
3. Keep `PreparedSkillsBatch.Apply(ctx)` compatible. Add an optional observer
   entry point for terminal use; existing browser/SDK callers need no changes.
4. Reuse the dependency installer callbacks/result seam. Add only the missing
   prepare/apply observation hooks; do not add a new core application protocol.
5. Extend the existing board with binding counters for Skills. A row represents
   one selected agent and aggregates every selected Skill for that agent.
   A successful first Skill cannot mark the entire agent successful.
6. Do not invent per-agent download percentages: immutable sources can be shared
   across agents. Show global acquisition status, then actual per-agent work.

## Review

- Default rich view: unique Skills, selected agents, counts of install/update/
  unchanged/remove actions, and all warnings. Use friendly agent names.
- Keep the confirmation and summary in the existing bounded viewport.
- Optional `Details []string` on confirmation requests exposes exact action/path
  mappings using `d`; toggling resets scroll position and preserves Yes/No state.
- Validate Summary + Details against the existing combined 128-row/64-KiB budget;
  individual rows remain bounded and sanitized, never silently clipped authority.
  If the rich budget is exceeded, show the complete existing plain review before
  asking for confirmation instead of hiding actions or failing after review.
- Plain and noninteractive output retain complete action/path mappings. JSON and
  dry-run schemas remain unchanged. No summary is printed twice.
- Warnings/conflicts stay visible in the default summary, not only in details.

## Plugin startup and output ownership

- Cover acquisition for explicit-target, multi-target and prompted plugin paths.
- The prompted path must show discovery and compatibility activity before the
  first agent-selection prompt.
- Source acquisition may write security/revocation warnings. Stop the activity
  before forwarding the first warning; never repaint over it or buffer it away.
- Never animate over confirmation, security consent or native manual actions.
- Human TTY uses the shared theme; plain/dumb terminals use stage text. Preserve
  established JSON/non-TTY behavior and existing native group progress.

## Skills progress and partial results

- Known actions initialize rows/counters; unchanged bindings count as unchanged,
  not newly installed. Start/finish observations carry alias and client identity.
- During application show the active Skill and agent with observed completed /
  planned binding counts. Preserve distinct succeeded/setup-required/failed/
  rolled-back/not-attempted outcomes rather than painting all completed green.
- A prepare failure marks that binding failed; later bindings remain not
  attempted. Failures before binding execution mark no agent installed.
- On finish derive the terminal summary from the returned `SkillsBatchResult`,
  not from animation state. Show which Skill/agent failed and the existing
  recovery/status instructions. Cancellation is not success.
- Resize/narrow terminals degrade safely using the existing board policy.
  Observer/rendering failures must not change installation outcomes.

## Claude failure investigation

Read-only reproduction found the precise mismatch in Claude Code 2.1.325:
under the adapter's bounded environment, the project suppression sentinel has a
new seventh `noteDetails` field. `projectSuppression` requires exactly six fields,
so rejects this otherwise supported untrusted-workspace observation. The user's
receipt confirms only canvas-design/Claude was materialized; activation and
verification failed. No other binding may be presented as installed from that run.

1. Read-only reproduction: installed CLI version and exact project plugin-list
   command/output at the failing scope; sanitize captured output.
2. Trace the failed native operation and native identity/path matching. Determine
   whether the failure is parser/schema drift, transient output, or a mismatched
   invocation. Do not assume the currently successful list reproduces the bug.
3. Accept only the bounded, typed optional `noteDetails` metadata corresponding to
   the observed suppression. Add actual seven-field sentinel regression plus
   malformed/contradictory/foreign/ambiguous negative cases. Retain the original
   six core checks and duplicate-key rejection.
4. Preserve foreign installation ownership and fail closed on unsupported data.
   Do not treat an unrecognized list as empty or rerun mutations automatically.
5. Suppression still means user action is required for trust/reload. Report
   installed/prepared only when independently proved by existing receipts/files;
   never report active merely because the sentinel is recognized.

## Critique, round 1: reject attractive shortcuts

- A separate Skills board would duplicate layout/terminal cleanup: reuse the
  existing board and add counters only where binding multiplicity needs them.
- Fake per-agent download/configuration percentages would overstate evidence:
  use shared acquisition + real binding observations.
- Hiding all paths or warnings makes confirmation weaker: paths are available
  before confirmation and plain fallback retains the full mapping.
- Suppressing Claude's error or ignoring failed list parsing could overwrite
  foreign state: correct only an observed supported case; unknown stays refused.
- Retrying a partial install blindly can repeat native mutations: surface the
  recorded per-binding results and existing recovery workflow instead.

## Verification and delivery

## Critique, round 2: independent review corrections

- Counter identity is the planned binding's client + alias + action, not display
  name; derive totals from prepared actions, including unchanged/remove.
- Final reconciliation covers every planned binding even if prepare/cancellation
  prevented a completion callback. Keep prior successes and unattempted bindings
  distinct when another binding fails.
- On rich-budget fallback, print the complete paths/warnings first and pass only
  a bounded confirmation summary, without the over-budget Details payload.
- Observer callbacks are non-authoritative: isolate observer panics/failures and
  renderer failures without changing receipts or masking the original error.
- Warning-safe output forwards SemanticTheme and writer errors, with idempotent
  stop. Test multiple warning writes and later cleanup, not only one message.

## Verification and delivery

- Publish this plan in PR #932 first; complete an independent plan critique before
  implementation, then update this file with material corrections and outcomes.
- Real PTY at 80x24 and 40x12: compact review, details toggle/scroll, warnings,
  Yes/No/cancel, and visible controls without wrapping corruption.
- Deterministic multi-Skill/multi-agent fixture: successful counters, failure on
  first/second binding, unchanged, cancellation and unattempted rows. Prove that
  adding the observer does not alter effects or partial-result mapping.
- Plugin warning fixture: warning survives activity cleanup; existing plugin
  group-board behavior remains unchanged.
- Claude parser/native transport regression, then disposable project lifecycle
  without model calls if the environment can execute its contained Git/native
  processes. Never mutate the user's installed profiles to obtain test evidence.
- Run relevant UI, adapter, progress and npm regression suites. Distinguish
  pass/fail/skip and sandbox restrictions; do not claim a full E2E pass from a
  presentation-only test. Push each completed slice to the existing Draft PR.
