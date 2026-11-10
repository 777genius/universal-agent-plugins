# BUG-057 — Grok project Skill E2E must not opt into a custom home

## Reproduced contract contradiction

Current test harness `scripts/live-install-audit.py` sets `GROK_HOME=case/home/.grok` for every isolated child. Production [`clients/grok/detect.go`](../../../install/integrationctl/agentplugins/clients/grok/detect.go) marks **any** nonempty `GROK_HOME` as a detected `grok_custom_home` surface, even when it spells exactly the default path. [`clients/grok/scope.go`](../../../install/integrationctl/agentplugins/clients/grok/scope.go) refuses **project scope** when that surface is present, because compatibility sources outside the default profile cannot be exhausted safely. Thus new real Grok Build 1.0.70 CLI from BUG-056 would still produce false `PLAN_REJECTED` for the 30 project Skills if the harness forces `GROK_HOME`.

This is a test-environment mismatch, **not** a product bug and **not** success evidence. The real client naturally locates `HOME/.grok` when `GROK_HOME` is absent. `HOME` was already bound to a newly allocated disposable case with no links to the user's profile. The incoming ambient `GROK_HOME` is already dropped by the inherited-env allowlist.

## Narrow correction and review

**Correctness critic:** Omit only the harness's `GROK_HOME` override, retaining the source client ID, all project/user filesystem roots, real vendor `--version`, native CLI selection, exact source-lock-repeat-removal oracles, and active profile safety policy. The project adapter then observes the ordinary disposable `HOME/.grok`, not a fake custom profile.

**Security critic:** Never inherit the caller's `GROK_HOME`: the existing child environment allowlist excludes it. `HOME`, XDG, temporary root, Git config and the actual UAP CLI remain private to the per-case directory. Do not relax `grok_custom_home` rejection in production or create a special debug/feature-flag escape hatch. Do not count the native 1.0.70 version probe as any source-case PASS.

Add a hermetic no-network regression with a hostile ambient `GROK_HOME`, `HOME` and XDG; assert the child has **no** `GROK_HOME` key, its `HOME`/XDG remain under the case, and nothing is created in the default .grok location. The original general profile isolation regression remains unchanged.

## Acceptance

Run focused Python + full Required/Core fast on the **new** PR SHA, and inspect original Grok Plugins and Skills 30-case ZIPs. Only observed add/list/repeat/remove+lock/native/foreign invariants qualify lifecycle PASS. A native refusal or unqualified plugin stays non-PASS. No vendor auth, LintAI or native process-custody bypass.
