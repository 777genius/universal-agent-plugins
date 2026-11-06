# BUG-059 — live audit harness exception erases source identity

## Reproduced root cause
The `do_case` exception path in `scripts/live-install-audit.py` only reports kind, target, slot index, exception string and `harness-error`. It omits the immutable pinned `source` and expected `identity`, even though ordinary and bootstrap-refused records preserve them. This breaks an independent 30×30 source-slot audit specifically when an unexpected error occurs.

## Narrow correction
Copy exact fixture `source` and declared `identity` from the already-validated selected item into the existing exception record, without changing the result status or catching additional errors. Do not log arbitrary profile contents, credentials or native client settings.

## Regression and two critical passes
- Correctness: force a no-network `do_case` exception through `main()` and assert exact source, identity, index, kind, target, status, and attempted count.
- Security: test uses a disposable inert CLI placeholder, no native process or user profile; exceptions remain `harness-error`, never PASS.
- CI: focused Python regression and full exact-head audit source/provenance requalification required.
