# Antigravity User installation readback extraction

## Goal and source

Deliver the independently useful installation slice from PR #878. User and
Project file installation already exist on `main`; this change adds automatic,
read-only native CLI inventory for a qualified User profile. The owner's
2026-11-14 priority is useful automatic installation, with independent ready
slices merged separately and no requirement for real agent/model calls.

Base: `1e09d0fbc36d1d3ec9d20a983fa593605c7b3606`.
Reviewed source: `a610eadf5436dd3fb2b29fbb069615028b46ad00` in
[PR #878](https://github.com/777genius/universal-agent-plugins/pull/878).
This extraction does not authorize a release.

Prerequisite: the original Windows job `132628350179` at `82fa4933` failed
before Antigravity execution because the release-asset reader treated distinct
full-width filesystem identities as an alias. Main has the same reader blob.
Extract the reviewed two-file `release-assets.js` bigint identity correction
and its causal regression test as a separate fix, retaining alias refusal and
safe numeric JSON sizes. This installation slice must include that correction
in its ancestry before its final Windows run.

## R1: bounded implementation

Keep the existing Antigravity adapter and shared capability seams. Transfer the
reviewed User inventory implementation, original-HOME detection provenance and
User lifecycle hook. Reuse the existing prepared lifecycle resume and confirmed
repair paths, adding only their missing capability-based observation admission.
Preserve the live detector's native root through the saved-install SDK request;
the field is transient and cannot come from stored diagnostic JSON.

Production allowlist: Antigravity `adapter.go`, `inventory.go`, `lifecycle.go`;
usecase `apply_outcomes.go`, `lifecycle_observation.go`; CLI
`add_application.go`, `deps_sync.go`; installer `types.go`, `package_lookup.go`,
`prepare_package_plan.go`. The copied shared hunks exclude plugin enablement,
hosted preparation, native transitions and selected-session work.

Reuse the four reviewed adapter, inventory, public-command and SDK recovery test
files and the existing four-line public-fixture root delegation. Regenerate the
three detector goldens; only Antigravity's original native root may change.
Extend the existing cross-platform native lifecycle with a small User
readback block and its workflow filters. Retain existing native Project tests.
Do not transfer the research collector, its bridge or its historical plan log.

Estimated change: about 610 added production lines (498 in the existing reviewed
inventory implementation), 1,750 test lines and at most 350 native harness,
workflow and documentation lines. Complexity: medium, because saved install
crosses CLI/SDK boundaries. Expected reliability: high once the checks below
pass. R1 confidence: 8/10; composition on the current base still requires R2.

## Invariants and acceptance

- Only the original, unaliased User profile may invoke pinned `agy 1.5.3
  --version` and `plugin list --json`. Admission requires a qualified existing
  config, or the independently preserved original absence on repeated add or
  a verify-only request. Grouped repair without settings remains file-only;
  grouped repair with qualified settings must expose actual inventory guidance.
- Initial file delivery must not bootstrap native settings. Project and
  unqualified SDK profiles retain file-only behavior.
- Keep exact package identity, config/binary incarnation and bytes, bounded
  decoding, common deadline, cleanup and cancellation checks. Disabled entries
  remain disabled; foreign entries and sibling projects remain unchanged.
- Explicit grouped repair may replace only the selected owned directory through
  the existing transaction. Verify its complete entries, modes and bytes, retain
  the original unselected identity baseline, and protect the replacement's full
  identity during the next native inventory call.
- Successful inventory remains **Prepared**. Preview has no native side effect.
  Native failures retain uncertainty and the existing ownership authority.
- No native enable/disable, authentication, model calls, IDE activation, schema
  migration, new state store or changes to frozen clients.

R2 independently reviews the final allowlist and actual composed caller paths.
Run `make lint`, `make test-core`, focused public/SDK contracts and the real
pinned Linux native lifecycle locally. Require actual passing final-commit
Core/Required/security checks and native Linux/macOS/Windows jobs, then verify
postmerge. Each existing source-native job also runs the full Antigravity adapter
tests and the public/SDK `TestAntigravity` methods; Core's Project-only selector
does not cover these new User methods. Queued, cancelled, skipped and unexecuted
cases do not count as PASS.
Publish through an ordinary push; no manual workflow dispatch, rerun or cancel.

## Native inventory schema correction (2026-11-15)

The first actual Linux User readback run on the composed Devin branch
`3f36797cc5ebea2c5ab0f426850d14165d92dabc` reached the initial installation,
absent-config repeat and ordinary native lifecycle, then refused the first
configured repeat with a disabled foreign plugin ([job 132663029840](https://github.com/777genius/universal-agent-plugins/actions/runs/43665239507/job/132663029840)).
Its inventory implementation is identical to Antigravity head `a1d55d6`.
The original error does not expose the failing internal predicate.

A separate disposable-profile reproduction with the exact pinned agy 1.5.3
binary (`7ca212748ca0dfda718efbc39a776a104472e7e9344d4b6ea147c543684f96a3`)
shows the concrete schema mismatch: manifest-only plugins have four mandatory
fields; empty `skills` and `mcpServers` fields are omitted. Skill-only and
MCP-only rows have five fields. All read-only lists preserve the original
profile and foreign entries. The current six-field decoder rejects these
valid rows before matching the selected plugin.

R1 limits the correction to the existing inventory decoder and a new focused
`inventory_schema_test.go`. Keep four mandatory typed fields and exactly two
optional component-array keys. Treat only absence as an empty component list;
explicit null, unknown keys, wrong types, duplicate names and missing required
selected components still refuse. Retain every original identity, ownership,
configuration, executable, process, deadline and Prepared-state check. The
real native fixture remains unchanged and must finish on all three OSes.

Verify the observed four/five-field rows through the configured public adapter
path, preserve all existing negative tests, and run an original-source overlay
that fails the new positive assertions. Require independent R2, full lint/core
commands and actual final-commit CI before merging; this reproduction alone
does not qualify the public native lifecycle.

The final decoder and test allocation cleanup passed independent source review.
The complete adapter ran 45 top-level tests / 213 test events, with no failures
or skips; the four new methods account for 72 passing events. An overlay using
the original decoder compiled and failed the 17 intended positive assertions
(18 failure events including their parent), while 54 negative/control events
passed. The later test-only slice preallocation preserves every test input and
assertion. No existing test or native harness was weakened.

Full `make lint` passed. Full `make test-core CORE_TEST_PARALLELISM=1` completed
unsuccessfully in this local environment: 95 top-level failures / 137 failure
events in 11 packages. The prior 93 failures remain; the two additional unchanged
Gemini hook fixture tests combine Node stdout/stderr before JSON decoding. A
direct Node 22.21.1 fixture reproduction returned valid JSON with exit zero but
also emitted an inherited `UNDICI-EHPA` proxy-agent warning on stderr, matching
the invalid leading-parenthesis failure. These are not reported as a local
full-suite PASS. Final-source Core, Required, security and all three real native
Antigravity jobs remain mandatory before merge.
