# Decision: short-name User/Project installation without Directory v2 cutover

Owner decision: 2026-11-05. Status: implementation planned; **not released or qualified**.
Supersedes the Directory-v2 release/cutover portion of [the 2026-11-04 plan](2026-11-04-global-project-skills-mcp.md). It does **not** undo the already merged installer/native changes in UAP #707.

## What the owner actually needs

The same package identifier (for example `marketing-kit`) must install a complete Agent Plugins 1.0 Skills/MCP package and all mandatory dependencies, either into the current user or an explicitly selected project:

```sh
agentplugins install marketing-kit --scope user
agentplugins install marketing-kit --scope project
```

The CLI has only one installation lifecycle and one dependency engine. The selected scope controls where owned files, native settings, state and receipts live. The signed Directory is used to resolve short names to reviewed immutable sources and to enforce source integrity/revocation; the local installed client/adapter decides whether a component can be delivered at that scope.

**The original plan's requirement for a separate publisher-issued Project grant is withdrawn.** This is a substantive owner-authorized policy change, not a parser workaround. A signed v1 `scopes: ["user"]` record is historical *publisher qualification* for the user scope, not a blanket prohibition on an end user's explicit project installation. Do not present historical user evidence as proof of project-native behavior.

## Directory v2 status and disposition

- UAP [#707](https://github.com/777genius/universal-agent-plugins/pull/707) **merged** at `10791d250ecc722fb791a23b853a8873f099bd5f`; it includes v2 Go decoding, cache, domain policy and v2 tests, and useful existing Skills/MCP/lock E2E.
- Registry [#420](https://github.com/777genius/universal-agent-plugins-registry/pull/420) and [#421](https://github.com/777genius/universal-agent-plugins-registry/pull/421) **merged**: offline v2 schema/signing/bootstrap preview; they have **not** published a production v2 feed or paired v1/v2 CAS publication.
- Registry [#434](https://github.com/777genius/universal-agent-plugins-registry/pull/434) is an open v2 signer safety correction; do not merge or delete it just because this direction changed. Review its disposition separately with the Registry owner.
- Production CLI still composes the existing authenticated `/registry/schemas/1/` client. **Do not switch the CLI/site to v2, publish a v2 Project grant, build a parallel publisher or start a v1/v2 data migration as part of this task.** Historical signed v1, existing trust key, bounded transport, monotonic sequence, revocation, publication CAS and receipt compatibility remain.
- Directory v2 code already in `main` is **dormant/deferred, not removed** by this decision. Remove unused code only after dependency/consumer inventory and regression proof; do not erase unrelated tests or historical documentation. If a safe narrow removal is not demonstrable, leave code untouched and track deletion separately.
- Agent Plugins 1.0, Agent Skills, `uap.toml` schema 1, `uap.lock` schema 1 and installed states are not migrated.

## Trust and authorization model (security-critical)

1. `--scope project` is explicit user consent to place the package in that project; no implicit `user` fallback on refusal. Unspecified scope preserves existing command behavior.
2. A short-name lookup uses the **verified signed v1** source, not an unsigned site index, unverified search result, or arbitrary Git fallback. Keep source SHA/tree/manifest digests, release status and revocations, minimum installer floor, exact selector identity, authenticated cache and rollback limits. Unknown/ambiguous names fail closed.
3. All required Registry dependencies are resolved and authenticated individually through the same existing resolver. Direct local/immutable Git dependencies retain their existing distinct trust path. Never grant one child permission just because the root was authenticated.
4. For a v1-signed short-name package in an explicitly selected project, the old `user` policy can establish *published source eligibility* only. Independently require the selected target client's genuine project-scoped capabilities and supported delivery for every mandatory component. A client without qualified project Skills/MCP delivery fails before mutation; `prepared` is not `active`.
5. Never transplant `user` runtime, OAuth or client-scope evidence into Project. Do not fabricate v2 Project grants, signatures, grants in persistent receipts, certified badge claims or cross-scope cache authority. Existing project adapter runtime rules and confirmation still apply.
6. Keep every installed `origin` and lock source as the **actual signed v1 publication**; recorded release/pin/sequence must stay usable for authorized locked restore, update and repair at the original installation scope. A missing source, revoked parent/child, digest mismatch or ownership conflict fails closed; no silent fallbacks or partial installs.
7. Preserve user / Project A / Project B isolation, foreign files, transaction/recovery kernel and offline scoped removal. Never write outside the chosen canonical root.
8. The site must display truthful CLI commands for a **released** version only, and must not assert a passed project-native runtime test based on user evidence.

## Smallest implementation

1. Inspect the latest main, overlapping PRs, project capability contract and existing v1 source/target policy. Add one narrow, explicit `project-from-signed-v1` selection path at the existing directory resolution boundary. Keep old v1 user semantics and signed read-only verification untouched. Prefer a tiny helper/purpose that resolves an authenticated source as user *only for publisher identity* and separately performs project adapter admission; avoid modifying the v1 wire validator or globally declaring v1 project grants.
2. Reuse that path **for both root alias and Registry dependency candidates/locked restore**, not just the top-level command. Preserve exact release, binding, recorded state and source hints, including Project A/B independent receipts. Apply project capability admission to actual Skill/MCP components after acquiring the verified package and before writes. Do not bypass a negative evidence/status/revocation check.
3. Reuse existing install, lock, transaction, native client and rollback engines. No new CLI subcommand, feature flag, installer, signing service, database, dependency graph, second cache, standards fields or migration. Avoid cross-client special-case branches.
4. Do not alter `main.go` default v1 composition. Update CLI help/docs and site scope affordances only when backed by real release compatibility; avoid a website-only pretend install state.

## Validation (actual evidence required)

- Focused Go tests for signed alias, unknown/ambiguous name, malformed/expired/tampered/rolled-back feed, revoked/unsupported root or mandatory child; explicit user and project scope, incompatible adapter refusal, inherited legacy user receipts, changed versions, cold project lock and exact-source restore. Check no file/cache/state writes after denial.
- Source-built public CLI E2E (not direct usecase mocks) in isolated HOME + Project A + Project B: marketing-kit-shaped Skills root + Notion, HubSpot, Playwright MCP children; install, idempotent replay, lock, cold restore, update/repair, removal and foreign-neighbor preservation. Also test the **real signed Marketing Kit** by short name when published evidence is available, not just a fabricated signed fixture. Label fixture vs production facts precisely.
- Existing `make lint`, `make test-core`, package-manager E2E, scope/rollback tests, and genuine native Claude/Codex/OpenCode/Cursor/Gemini/Copilot tests on applicable OS platforms. MCP registration/discovery is not equal to `initialize/tools/list/tools/call`; note any vendor-auth constraints as unqualified rather than PASS.
- Inspect current CI on exact HEAD; fix review comments and genuine failures. Request independent human/CodeRabbit review; do not call self-critique independent. Merge only if required gates pass and the exact intended behavior is verified; check `main` after squash merge.

## Review critiques and decisions

**Architecture critic:** A second Directory protocol is not needed when the only new user-facing dimension is installation placement. Keep one signed v1 source authority, existing resolver, installer and project adapters. Block any new service or duplicate policy graph.

**Security critic:** Changing the meaning of `v1.scopes` is a deliberate compatibility-policy relaxation. It is acceptable only for explicit project intent, under independent project-client capability checks and signed source/child revocation validation. Never turn user evidence into project runtime proof. Negative tests mandatory.

**Compatibility/UX critic:** Preserve user defaults and old receipts; unsupported project clients must refuse clearly without quietly installing globally. Site must not advertise unreleased behavior. Source-built fixture success cannot be reported as production native qualification.

**Plan revision after critique:** Use *separate source qualification and local placement admission* behind the existing directory CLI seam, rather than changing the signed schema, mass-rewriting `targetByClientScope`, or introducing a new protocol/engine. Preserve v1 user behavior and original provenance. Keep any eventual dead-v2-code cleanup independently reviewable.

## Explicit non-goals

No v2 launch, paired CAS, new key, key rotation, new closure digest, v2 migrations, scope policy feature flags, new database/index or unrelated adapter refactors. No forced release. Old plans and already published artifacts stay as history with this superseding decision linked.


## Implementation follow-up review — 2026-11-05

This is a **self-audit against three critique lenses**, not an independent LLM/human review. A real independent CodeRabbit review was requested on draft PR #725 but returned a rate-limit response; it remains pending and must not be counted as passed.

- **Architecture:** the first code pass keeps `domain.ResolveDirectory` unchanged (v1 still admits user grants only). A small CLI placement helper invokes that resolver using its existing signed User source authority when the operator explicitly selected Project. Reusing that helper for the alias canonicalizer, dependency candidate selection, locked restore and interactive selection matters; merely changing `add` would miss children and cold locks. Fixed alias canonicalization in the same draft branch after discovering the missed path.
- **Security:** the effective Project installation report must not carry historical User runtime/materialization evidence. The CLI now scopes evidence reporting and requires a valid local project capability entry; actual native admission and all complete-closure revocations must still be proven by CI/E2E. The signed v1 file bytes and trust key remain entirely unchanged.
- **UX/compatibility:** the website currently suppresses the Marketing Kit Project button and uses some direct-Git Project fallbacks. Do not make that site generate a new short-name command for a released CLI that lacks it. A site cutover is subsequent to the qualifying **CLI release**, not a substitute for a release. Preserve existing default user installation, project root selection, user/project locks and offline removal.

**Current branch scope:** small changes in the CLI's preexisting Directory resolution seams, a read-only source-evidence filter, focused regressions and this plan. **Not yet completed:** exact-head required CI, source-built signed Marketing Kit short-name + all MCP dependencies E2E, native client-driven E2E, independent review, release and postmerge. Do not merge before the necessary checks.
