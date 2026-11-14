# UAP package-manager risk hardening against APM failure classes (2026-11-02)

Status: Draft, evidence-first. **This document is a plan, not a claim that new protections or E2E have passed.**

## Goal and non-goals

Reduce critical supply-chain, integrity, state-isolation, and lifecycle regressions in **UAP's existing architecture** by treating real Microsoft APM incidents as adversarial test cases, not copying APM internals. Preserve UAP's Agent Plugins 1.0/Agent Skills 1.0 contracts, TOML manifest + lock, native Go CLI and existing installer/adapter boundaries.

Non-goals: replacing UAP with APM, introducing an APM-compatible YAML engine, modifying frozen VS Code code/fixtures, new feature flags, rewriting architecture or SDK, changing accepted ADRs, adding runtime daemons or unsupported platform claims. Never weaken existing scanner/ownership/source gates.

## Independently reported APM risk classes to verify against UAP

| APM issue | Failure class | UAP acceptance requirement |
|---|---|---|
| [#2296](https://github.com/microsoft/apm/issues/2296) | Lock-only operation deletes on-disk files | Lock/state-only operations must never mutate agent files; actual removals require an authorized installer with provenance and owner checks |
| [#2368](https://github.com/microsoft/apm/issues/2368) | Authentication env clobbers inherited Git hardening | Git environment construction must retain hardening, prevent credentials leaking to unrelated hosts, and reject malformed overrides |
| [#1935](https://github.com/microsoft/apm/issues/1935) | Source URL + tag move silently reuses stale lock | Source identity must include origin, path, pinned revision, digest and scope; changed identity must fail closed or re-resolve |
| [#1716](https://github.com/microsoft/apm/issues/1716) | Deployed agent files evade drift/integrity validation | All managed target files must be covered by owner/receipt verification, including skills; foreign files are not silently rewritten |
| [#450](https://github.com/microsoft/apm/issues/450) | Repeated install changes lock timestamp | Exact no-op restore leaves lock/manifest/managed bytes untouched |
| [#794](https://github.com/microsoft/apm/issues/794) | Global install writes project lock | Explicit project/global roots and scope isolation; no reliance on unrelated cwd |
| [#844](https://github.com/microsoft/apm/issues/844) | Wrong target MCP schema | Target-specific integration contracts; do not add generic cross-target schema |
| [#615](https://github.com/microsoft/apm/issues/615) | Proxy-only validation escapes network boundary | Acquisition/verification must obey declared source restrictions at each entry point |

APM issues may already be closed and fixed upstream. They are **historical incident evidence**, not claims about current APM release.

## Architecture-first implementation sequence

1. Inventory existing UAP coverage (including already-running package manager and UAP-vs-APM comparison workflows). Mark proven-safe cases and confirmed missing regressions; prefer test-only changes where UAP already protects the invariant.
2. Implement only evidenced gaps in the smallest domain/use-case/adapter boundary. Native Go code should continue to depend inward and use existing ports/adapters; maintain SOLID/DRY, refuse broad new abstraction layers.
3. Add adversarial tests: mutation of source+version, source archive/lock drift, scope-separated state, explicit/implicit destructive operation boundary, Git authentication/inheritance, no-op idempotency, cross-agent isolation. Include failing-before-fix evidence where feasible.
4. Run focused tests, full changed-package tests, race/security and required cross-platform GitHub Actions E2E. **Workflow green that captures evidence is not equivalent to E2E passing**; inspect individual cases.
5. Run multiple independent code-review lenses: security/supply chain, transaction and data-loss safety, Clean Architecture/SOLID/DRY. Fix substantive findings and recheck the final SHA. Do not merge without validated gates.

## Review rubric (0-10; evidence-based only)

SOLID: cohesion, open/closed extensibility, Liskov compatibility of target adapters, minimal interfaces, dependency direction.
Clean Architecture: domain vs adapters, explicit side effects, composition roots, source of truth, testable ports.
DRY: shared validated invariants without accidental coupling, avoid redundant registry/transport/target logic.
Safety: reversible/no-op semantics, source pin/integrity, filesystem ownership, scope isolation, negative E2E.
Score UAP vs Microsoft APM separately and transparently, distinguishing directly inspected code from documentation.

## Rollout / PR hygiene

- Draft from first commit; push incremental, scoped commits into this branch.
- Protect current open UAP PR work (#530, #657, #666, #498) and frozen VS Code scope.
- Source of truth: final branch HEAD SHA and CI logs/artifacts, not old success summaries.

## CI runner routing and latest SHA qualification

On 2026-11-02 the complete four-OS Package Manager E2E on head
`2a9225a357027519eaea298f25b5820e56b9467b` passed with
`suite=full`, `full_qualification=true`, source unchanged and 215 actual
run+pass catalog events per platform, including all three new required
regressions. Evidence is in workflow run 41502246452.

The other Linux-based Required/Core fast/CodeQL/source/govuln jobs remained
**queued** while explicitly hosted Windows/macOS and pinned Ubuntu jobs
completed. Those workflows explicitly prefer `vars.CI_LINUX_RUNNER` when
defined, but have an existing pull-request label fallback to GitHub hosted
Ubuntu via `ci:github-hosted`. Apply that existing label and re-run the
complete PR checks on the **new** HEAD. This is a runner-routing workaround,
not suppression/waiver of any CI gate. Only merge after fresh CodeQL, Required,
Core, Source transports and other applicable jobs are proven on the new HEAD.
