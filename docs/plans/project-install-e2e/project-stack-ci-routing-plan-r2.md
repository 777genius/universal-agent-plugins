# Project stack CI routing amendment independent plan review R2

Reviewer: cursor_clients, 2026-10-23. Reviewed pre-implementation plan at
`94be629c7d3a3675828214f6f84659f9ddc7a0b4`:
`docs/plans/project-install-e2e/project-stack-ci-routing-amendment.md`.
Inspected current `.github/workflows/agentplugins-native-clients.yml`,
`.github/workflows/core-fast.yml`, the merged native selector and its tests.

## Decision: GO for additive routing implementation

The observed routing gap is real: the current native workflow accepts only
main as a pull-request base, and core-fast accepts the exact project/source
feature bases but misses task-owned temporary project client/integration bases.
Adding `feat/project-*` and `feat/source-transports-e2e` matches the intended
stack. The wildcard also retains the existing exact project feature base;
main and core-fast's refactor/** filter must remain.

The plan changes only pull-request base admission. Existing path filters,
events, permissions, action pins, platform matrices, jobs, tests, timeouts,
source checkout and credential controls remain intact. It explicitly preserves
the disposable-hosted opt-in/platform guard and all draft, release, historical
and source-selection restrictions. Routing to a branch cannot itself confer
native authority or qualify absent/queued evidence.

The Codex/Claude selector union and both distinct exact-selection tests are
already preserved in integration `daa705ad`. Independent bounded checks passed
18 matrix tests and 42 released-evidence tests. The additive routing patch must
retain those assertions and the original legacy required suites. Carrying it
through true merges into active leaves is consistent with the reviewed stack;
the temporary checkpoint creates neither another deliverable feature nor a
release channel. Client PR merges remain into the project feature only.

No plan blocker found. After both independent plan GO reports are committed,
implementation review must check the exact workflow diff is limited to these
branch-filter additions, rerun the selector/guard contracts, and observe actual
Actions runs on the published exact client heads. An unrun native fixture or a
successful configuration fixture cannot be reported as genuine native proof.
