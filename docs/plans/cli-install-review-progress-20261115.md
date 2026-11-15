# Consistent CLI installation review and progress

Status: native installation completion in progress; final qualification pending.

## Owner follow-up: complete native setup (2026-11-15)

The owner clarified that a successful file copy followed by generic "setup
required" guidance is insufficient. First qualify ordinary Skills and plugin
installation in real Codex and Claude Code without model requests, subscriptions
or API credentials. Perform available discovery/setup automatically, while
preserving the client's actual trust and policy decisions. Then review, merge,
and qualify the same behavior in other supported clients. This supersedes the
earlier assumption that all native setup can remain a manual next step.

Use the existing client adapters, bounded native command runner and lifecycle
result fields. The CLI presents returned evidence; it must not upgrade a file
copy to native readiness or branch on individual client IDs. Reuse the same
installation/verification path for raw Skills and plugins containing Skills.
Interactive trust is explicit; do not fabricate trust flags or model execution.

| Follow-up | Production LOC estimate | Test LOC estimate | Complexity /10 | Reliability /10 | Confidence /10 |
|---|---:|---:|---:|---:|---:|
| Codex native Skills discovery through its existing app-server protocol | 150–250 | 150–250 | 5 | 9 | 9 |
| Claude native setup/discovery correction, after reproduction | 40–120 | 60–120 | 5 | 8 | 7 |
| Evidence-based final summary and loader error isolation | 30–70 | 50–90 | 3 | 9 | 9 |

Critique 1: avoid a second setup engine or client-ID switches in CLI code. Native
protocol handling stays in each adapter. Missing/disabled/mis-scoped discovery
cannot count as success; deadlines and cancellation must release the owned child.

Critique 2: installation confirmation is not blanket project trust. Qualify the
real client's supported no-model path before implementing automation. Keep
noninteractive refusal honest and distinguish credentials required by an MCP
server from credential-free installation of Skills. Do not retain a blanket
"unverified" footer when the adapter has actually verified the installation.

Codex and Claude explicitly qualify authentication for native installation of a
single normalized raw Git/HTTPS Skill: the validated immutable source and plan
must contain only that admitted Skill, with no MCP, app, command or extension
components and no catalog policy. For that narrow delivery, `not_checked` becomes
`not_required`; native discovery must still succeed separately. This means no
login is needed to install and discover the Skill. It does not authenticate a
model or any service a Skill may use later. Explicit catalog requirements,
general plugins and every other client's authentication policy remain unchanged.

Initial source verification also found a corrupted tail in
`skills_progress_test.go` that prevented the CLI test package from compiling.
Restore its prior intended assertions before treating earlier test reports as
evidence. The npm progress observer must not be able to crash a verified download.

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
- Optional `Details []string` on the internal prompt request exposes exact
  action/path mappings using `d`; toggling resets scroll and preserves Yes/No.
  Preserve the public three-field `installerui.ConfirmRequest` shape, including
  existing unkeyed consumers; use additive `Terminal.ConfirmWithDetails` instead
  of adding a field to that exported request. A consumer qualification test exposed
  this compatibility constraint during implementation; it must remain unchanged.
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

## Implementation evidence

- Plan published first: `8b343f2`; Claude compatibility/plugin activity slice:
  `0ca6b77`. Public three-field confirmation requests remain source-compatible.
- Claude adapter: complete package passed with the race detector. Current
  seven-field response, negative schemas, foreign state and artifact proof are
  tested; trusted activation is not inferred from suppression.
- Confirmation: six real PTY scenarios (80x24 / 40x12, accept / decline / cancel)
  passed, including details toggle, scrolling, preserved choice and visible
  controls. Combined-budget fallback and the unchanged consumer contract passed.
- Skills progress: focused race tests passed for duplicate events, partial / early
  failure, cancellation, unchanged, setup/sign-in, observer failure isolation and
  final-result reconciliation. Actual Darwin PTY binding-count/resize test passed.
- Plugin activity: focused race tests passed, including warnings before cleanup,
  theme forwarding and writer failure propagation.
- The existing live plugin board now also covers one selected agent; a real PTY
  regression passed for that path. Multi-agent behavior remains on the same board.
- Upstream's rounded confirmation card and explicit Skills discovery/activation
  evidence notices were retained during integration; narrow Details views use
  the compact layout so consent controls remain visible.
- Installer UI, prompt adapter and prompt-port suites passed with exactly the two
  existing sandbox-restricted `/dev/tty` scenarios explicitly excluded. They are
  not counted as passing.
- This local sandbox denies Darwin process-group inspection in the contained Git
  runner. Git-backed Skills installation E2E is therefore delegated to the
  existing `Core fast / test-core` CI gate on the exact PR head; queued work is not
  a successful qualification. No extra workflow or second test engine is added.
- Actual Go diff relative to the pre-plan PR head: 747 added / 121 removed
  production lines and 879 added test lines. This exceeds the original changed-
  line estimates, principally due to preserved public API compatibility, real PTY
  coverage and integration with the newer upstream confirmation layout.

## Follow-up: owner reproduction with two Skills

The owner reproduced a failure with `docx` + `frontend-design`, five agents,
using the current local `go run` entry point. Earlier single-directory Claude
qualification does not establish that a multi-directory response is accepted.

- First verify actual Claude Code and Codex installations in disposable projects;
  capture the bounded native observation and independently check installed files.
- In parallel verify installed Cursor/Gemini clients and Copilot availability.
  Separate real native runs, deterministic fixtures and environment restrictions.
- Reproduce `d` in the complete selection → review flow, including input handoff,
  rather than relying only on an isolated confirmation test.
- Make every grouped action heading use the same style; retain exact content.
- Extend Claude regressions with multiple suppressed project directories and
  preserve fail-closed handling of unsupported native output.

Additional estimate: 40–100 production LOC and 100–200 focused test LOC, subject to
actual diagnosis. No new renderer, installation engine, or model calls.


### Follow-up qualification

- Root cause reproduced with real Claude Code 2.1.325: the two-directory native
  trust note uses plural grammar. The earlier singular-only match rejected it.
  Match canonical positive counts >=2 plus the exact observed suffix; keep typed
  metadata and every other native-state validation unchanged. A captured two-
  directory JSON fixture and malformed-count regressions cover this response.
- Actual Codex project installs: two independent local Skill packages installed,
  both native files independently byte-verified; no model calls. Actual UAP
  Claude group installation stopped before mutation because this sandbox denies
  Darwin process-group inspection. Real Claude read-only two-directory output
  and the complete Claude race suite passed; this is not a full native install.
- Actual Cursor, Gemini and Copilot project configuration installs: two independent
  local Skill packages each, exact installed bytes, repeat first package produced
  no change. Copilot CLI is absent; runtime discovery is not claimed. These local
  source runs do not qualify the public-Git raw-Skills transport.
- Codex, Copilot, Cursor and Gemini adapter packages passed. Focused Skills
  review/progress/result and actual progress PTY tests passed. Full Git-backed
  installation fixtures remain restricted by the local contained-process sandbox.
- Uniform action headings and the full Skills → agents → review keyboard flow
  are covered at 80x24 and 40x12, with Latin d, Russian в, uppercase D and enhanced
  keyboard input. Ctrl+D cancellation remains separate. The exact reason for the
  owner's original d key not responding is not proven; ASCII d also passed before
  this change. Layout-compatible shortcuts close the reproduced Cyrillic gap.
- Follow-up actual Go additions relative to the previous implementation commit:
  126 production and 156 test lines. Production includes lint-driven extraction
  of existing result/review logic into smaller helpers, not just new behavior.

## Automatic native completion qualification

The owner's follow-up requires the ordinary Skills flow to finish supported
native setup itself, then report the observed result. Native file discovery is
qualified without vendor credentials, model requests or silent trust changes.

- Codex now uses a bounded, finite `app-server` initialization and `skills/list`
  exchange for supported project Skills. It checks the exact project, profile,
  enabled names and owned paths, then rechecks owned bytes. The existing process
  containment remains required; the new finite exchange does not change Kiro's
  duplex execution.
- Claude now registers an owned project-scoped local marketplace containing the
  existing sanitized selected projection, then uses native install/list/cache
  evidence. A different project must not enable or load that registration.
  Same-version content updates refresh the native cache.
- A removed Claude project binding retires only its exact receipt-verified current
  cache version. Real Claude 2.1.263 and 2.1.325 add a native `.orphaned_at`
  timestamp on uninstall. Only this validated metadata is normalized in a
  private bounded snapshot; the remaining digest must match the original
  receipt. Foreign entries, other versions and plugin data remain protected.
  Retirement moves that verified version under an exclusive private
  `.agentplugins-cache-retire-*/retired` directory and retains its bytes. It
  does not recursively delete the retired tree: an already running native
  client may still hold a cwd or file descriptor and append new data. The
  active version path must be absent before retirement reports success, so a
  same-identity reinstall can create a fresh native cache without adopting
  retained bytes. No automatic cleanup command or new ownership state is added.
- Pure normalized raw Skills have an explicit Codex/Claude no-authentication
  installation policy. This does not change arbitrary plugin, catalog, MCP or
  other-client authentication claims.
- Repeating a completed Skills selection reobserves its exact native binding
  without rewriting state. An incomplete selection reviews an existing repair
  or projection-refresh operation before consent, preserving source pins,
  receipts, stale-plan checks and ordinary dependency-sync policy.
- Terminal and browser results use the actual per-item native evidence. The
  browser shows completed setup, remaining setup and partial failures distinctly;
  repair/refresh actions use the generated API contract. TypeScript is pinned to
  7.0.2 with only its official platform packages changed in the lockfile.

### Observed results and remaining gates

- Real Codex 0.153.4 and 0.167.0-alpha.4 independently listed the installed project
  Skills without account credentials or a model request.
- Real Claude 2.1.263 and 2.1.325 passed adapter project/user lifecycle probes.
  Project probes include install, unchanged verification, same-version refresh,
  modified-cache refusal, removal, fresh same-ID reinstall without previous
  native receipts, and final removal. Independent review inspected the native
  transcripts and found no remaining cache-removal blocker.
- The public native repotest now additionally requires two immutable raw Skills
  to report ready, appear exactly once in credential-free SDK initialization in
  project A and never in B, preserve state on repeat, and complete removal and
  reinstall with the same owned identity and paths.
- Native SDK initialization itself creates profile metadata and plugin-usage
  counters. The untrusted fixture captures its original baseline before UAP,
  validates trust on every observation and permits only the observed native
  usage field to change. Authored foreign files retain their exact byte checks.
- Focused ownership, raw authentication, saved repair/refresh, readiness,
  generated contract, TypeScript/model and native evidence-parser tests passed.
  The full core run also exposed obsolete prepared-Claude fixtures, which are
  updated in the candidate to the current scoped native contract; the combined
  hosted run remains required.
- Local ordinary UAP execution cannot satisfy this sandbox's process-containment
  prerequisite (`/proc/.../children`); Unix sockets are also denied, including
  browser startup. These failures are not counted as passes or bypassed.
- All eight golangci passes completed successfully before the execution
  environment interruption. The baseline then found growth in two legacy files;
  their activity/error adaptation has been moved into existing cohesive helpers.
  Final hosted lint, full core, native lifecycle, terminal/browser visual checks
  and independent review must pass on the published candidate before merge.

## Integrated setup and E2E corrections

The existing review is the consent boundary for persistent project trust and
selected MCP approval. Native warnings name the exact project or explain when a
supported native MCP inspection starts all enabled servers in the selected
profile/project. Skills and plugin review use the same warning path. No extra
prompt, installer protocol or source restaging was added.

- Pure Codex Skills discovery does not change trust or read project configuration.
  Selected MCP discovery requires stable 0.162.0-or-newer metadata from the same
  finite app-server process before sending the selected-server filter. Current
  native matrix pins use official 0.162.0 archives and upstream SHA-256 digests.
  The independent published/live npm 0.153.4 audit remains separately pinned.
- Gemini and qualified OpenCode V1 defer native MCP connections in verify-only
  mode. A fresh owned-state marker permits the existing reviewed repair path;
  confirmation then allows the disclosed native startup behavior. OpenCode V2
  and unknown profiles cannot enter the V1 observer.
- Conditional trust/approval writes preserve typed native effects after failures,
  including uncertain rollback. Subsequent probes stop on an uncertain effect.
  Existing ownership, locks, receipts, deny policy and source pins remain required.
- Terminal/browser copy says that installation was verified without claiming a
  model turn. Concrete per-agent setup actions survive partial results and are
  sanitized and deduplicated. Generic doctor advice is only a fallback.
- Windows Remotion case roots normalize a short TEMP spelling only after a fresh
  directory has been created and its canonical identity checked. The auditor's
  strict path, configuration-byte and SQLite checks remain intact.
- Native failure diagnostics retain the original failing assertion. A human
  repeat is diagnostic only; it cannot convert the failed operation to success.
  Repeat-state deltas expose bounded paths and digests, not file contents.

### Qualification status for the integration candidate

Before the execution environment disconnected, the local integration completed
all eight golangci passes and the lint baseline, the npm suite passed 167 tests,
and the Codex audit unit suite passed 22 tests. The full core run failed: it
included unavailable local containment/socket capabilities and unresolved
Codex fixture/golden mismatches. These results do not qualify the later source
reconstruction.

The 42556f4 CI evidence includes successful Windows native/PowerShell terminal
flows and the ordinary browser suite. The separate native browser chain stopped
before Apply because an unscoped locator matched the hidden sidebar. The
candidate scopes the review and loading checks to the visible `.bottom-bar`;
the native installation and inventory assertions remain required. Native Claude
and Codex E2E, Linux terminal fallbacks and the live/Remotion audits exposed
remaining failures. A native Codex initial install that stays Prepared still
requires its private diagnostic cause; removing the later raw-Skill trust
requirement is a separate integration correction. The Claude MCP removal
foreign-profile mismatch also requires the
actual native before/after evidence. No failure, skip, queued job or source-only
review is recorded as an E2E pass.

The execution environment was restored on 2026-11-15. The reconstructed candidate
is checked separately at
`/workspace/scratch/3e101078f946/uap-pr932-candidate`; the interrupted integration
worktree remains untouched. From the candidate's `scripts` directory,
`python3 -m unittest -v test_codex_runtime_audit test_live_install_audit_components`
passed all 33 tests (23 Codex runtime-audit tests and 10 live-audit component
tests) in 15.435 seconds, with zero skips. This includes the real orphan-process
negative, bounded exit/stderr diagnostics, SQLite and exact profile-byte guards,
and physical-root identity checks. The exact source hashes and full output are
recorded under
`/workspace/scratch/3e101078f946/candidate-qualification/python-audit/`.
These focused Python results do not qualify a native client installation,
Windows process execution, or the candidate's browser chain. Historical passes
above are not transferred to this candidate.

With the lock-identical cached dependency tree and TypeScript 7.0.2, the new
candidate also passed `npm run typecheck`, `npm run test:model` (two tests, zero
skips), and `npm run bundle:go`, all with exit code zero. The output is retained
in `/workspace/scratch/3e101078f946/candidate-ui-validation.log`. Local browser
startup remains blocked by `AF_UNIX` `EPERM`; the actual browser installation
chain still requires its hosted run.

The candidate reconstructed through Git objects must receive fresh compile,
lint, core, native lifecycle and visual results on its own published commit.
Independent source review covers the reconstructed changes; merge remains
conditional on resolving actual failures and completing the required checks.
