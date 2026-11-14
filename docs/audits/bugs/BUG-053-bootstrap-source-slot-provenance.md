# BUG-053 — sparse failed-bootstrap JSON lacks source/index provenance

## Reproduced on exact PR HEAD `02370a076004af998c49794c234ffb59a9d4a230`

Original source ZIPs from runs **41970634253** and **41970634689** contain all 30 selected identities per lane. However, the failed-vendor-bootstrap branches of `scripts/live-install-audit.py` create only `{target, kind, status, identity}`; ChatGPT unsupported Skills create only `{status, identity}`. Six blocked lanes (**Cursor Plugins, Grok Plugins/Skills, VS Code Plugins/Skills, ChatGPT Skills**) have 180 records with no `index` or original `source`. They correctly say `cases_attempted=0` and `full_coverage=false`, but a downstream reviewer cannot join the records to each immutable source/fixture slot, particularly where an explicit GitHub source maps to a declared Plugin identity.

These are **non-execution placeholders**, not 180 native attempted tests. Do not call them PASS or Prepared. The vendor bootstrap/cgroup/LintAI refusal remains unchanged.

## Narrow plan and regression

1. Introduce one pure helper constructing these non-execution rows from the already-validated selected fixture and declared Plugin names, with exact `index`, original `source`, `target`, `kind`, `identity` and unchanged `status`.
2. Permit only `client-unavailable` and `documented-unsupported` through the helper; an attempt to record `passed` is rejected. Reuse it in the two vendor-bootstrap exits and ChatGPT Skills unsupported exit. Keep `cases_attempted=0`, `full_coverage=false`, exit 2, and all vendor verification guards.
3. Add no-network regression for indexed, pinned explicit GitHub Plugin source / declared identity, Skill source, absent executed-install fields, and refusal to construct PASS. Run focused Python tests, LintAI and current-head original full-source ZIP qualification.

## Two critical perspectives

**Evidence accuracy:** A source-indexed refusal is not an installation. A 30-row blocked ZIP may establish that each input was considered but cannot establish native add/list/repeat/remove. Keep source and install identity distinct.

**Security/privacy:** Copy only public, pinned fixture source strings, never environment variables, native settings, authentication headers, or CI secrets. Do not execute previously unverified vendor binaries, supply invented native profiles, or bypass containment.

**Qualification:** fix is code and test coverage, not completion. New HEAD must show exactly the same honest non-PASS outcomes with source/index included, all required checks successful, and independent final-head review before merge.
