# Project stack CI routing amendment

Research and plan, 2026-10-23. This changes only which pull-request base
branches receive existing CI jobs; it does not broaden native runtime authority.

## Observed gap

The native client workflow currently matches only pull requests targeting main.
The existing core workflow additionally matches the exact project/source feature
bases, but misses temporary project client and integration checkpoint bases.
Consequently a source-only native project test can be authored and reviewed in
a client leaf without ever running in its stacked pull request. Retargeting that
leaf to the project feature alone would still not trigger the native workflow.

## Proposed change

1. Extend both existing pull-request branch filters to the task-owned
   `feat/project-*` stack and `feat/source-transports-e2e`. Preserve main and the
   core workflow's existing `refactor/**` filter.
2. Retain all path filters, events, permissions, action pins, timeouts, jobs,
   platforms, exact source checkout and credential handling.
3. Preserve the existing disposable GitHub-hosted runtime guard and all
   historical release/draft/source-selection restrictions. Never fabricate
   runner environment locally, dispatch native payloads locally, or skip a
   required native test to obtain a passing result.
4. Carry the additive routing change to the active client leaves through true
   merges so their next publication triggers the applicable jobs. Reconcile
   the Codex and Claude source-only selections by union; retain both existing
   exact-selection tests and the legacy required suites.
5. Keep successful package configuration, genuine native discovery, MCP runtime
   and release qualification as separate evidence. A queued or absent run is
   not a pass. GitHub client PR merges remain into the project feature only.

## Validation and review

Two independent plan review/fix rounds are required before changing workflow
code. Verify the final diff is an additive base-routing change, run the native
matrix selector/guard contracts, and observe actual pull-request Actions runs
on the exact published client heads. Cross-platform project contracts and
native source tests must retain their original assertions and failure behavior.
No test, lint, baseline, schema digest, trust, or authentication gate is weakened.

## Scope

The temporary checkpoint branch is internal to this task. It creates no third
feature PR and is not a release channel. The two deliverable feature PRs remain
sources and project installation; each client retains its own reviewed stack PR.
