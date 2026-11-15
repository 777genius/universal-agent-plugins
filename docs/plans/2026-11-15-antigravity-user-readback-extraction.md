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
