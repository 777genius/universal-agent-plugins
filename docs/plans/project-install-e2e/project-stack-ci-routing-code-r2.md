# Project stack CI routing implementation independent review R2

Reviewer: cursor_clients, 2026-10-23. Frozen code:
`7a3513673451c93a3d0e0e332e6719886c9d1f4c`.

## Decision: GO

The exact commit changes only two pull-request branch-filter lines, matching
the independently approved routing plan. Native Client Runtime adds
`feat/project-*` and `feat/source-transports-e2e` alongside main. Core fast
replaces its exact project feature spelling with the encompassing
`feat/project-*` and retains main, refactor/** and the source feature.

All other workflow bytes are unchanged: events/path filters, permissions,
action pins, jobs/matrices/platforms, timeouts, source checkout, credential
handling, actual test commands and evidence uploads. The patch changes no
runner, fixture, assertion, lint baseline, release condition or native guard.
The integrated Codex/Claude source-only selection union and distinct selector
tests remain intact. Temporary stack admission does not create a release
channel or expand native process authority.

Independent bounded verification: `python3 -m unittest discover -s scripts -p
test_native_client_matrix.py` — **18 tests PASS**. Inspected the complete
two-file diff and changed-path list. No native payload was executed or hosted
environment fabricated. Actual Actions execution on published exact client
heads remains pending; this code review does not claim those runs passed.
