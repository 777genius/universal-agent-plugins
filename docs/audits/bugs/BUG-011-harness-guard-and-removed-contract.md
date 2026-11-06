# BUG-011 — Harness guard fails CI for non-product reasons

## Reproduced failures on HEAD `82e9de0ef1499176962b66dccf023a6490e4bc82`

- [Required CI job 126291888479](https://github.com/777genius/universal-agent-plugins/actions/runs/41743871931): the removed-contract guard matched legacy OpenCode environment-key names used only as explicit negative-profile controls inside the disposable live harness. They do not restore the removed public API. Keep scanning product code unchanged.
- [Live pilot 126291888571](https://github.com/777genius/universal-agent-plugins/actions/runs/41743872036): all 14 native foreign-neighbor sentinel assertions failed because the harness wrote a terminal newline byte (0x0A), but the test expected a literal backslash followed by `n`. This was a test expectation bug, not native file corruption.

## Narrow plan

1. Add a single explicit `scripts/live-install-audit.py` negative-test harness exception to the removed OpenCode compatibility-name check; preserve all other scanned paths and the forbidden API contract rule.
2. Correct only the unit-test string expectation to newline, retaining all 14 filesystem preservation assertions.
3. Re-run the real Required and live CI on the new head; no installed source can be counted PASS before this.

## Two adversarial critiques

- **Contract critic:** Excluding the test-only harness must never allow a removed product compatibility path in application source. The exception applies to one named E2E script; the product tree remains scanned as before.
- **Evidence critic:** Do not "fix" by making the sentinel contain backslash characters; verify that the assertion checks exact *actual file bytes* as written by `do_case`. A green guard alone is not native E2E evidence.

**State:** fix committed; exact-head CI and original source artifacts pending.
