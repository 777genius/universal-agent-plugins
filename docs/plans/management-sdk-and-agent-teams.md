# UAP Management API, language clients, and Agent Teams integration

Status: implementation plan v2 — independent lifecycle, transport and UX critiques incorporated.
Owner authorization: 2026-10-24. Implement end to end, open draft PRs early, publish
every commit immediately, review plan revisions and implementation independently,
verify actual Electron interaction, merge only after required gates and resolved
review findings. This is not authorization to bypass repository protections or
publish a new public release.

## Outcome and scope

A host application can use the existing Go installer through a typed, language-neutral
management contract: discover plugins or standalone Skills, inspect desired and
observed state, prepare a change, accept exactly that retained plan, observe progress,
update selected/all moving dependencies, restore locked dependencies, repair,
remove, and handle interruption without repeating an uncertain mutation.

TypeScript is the first managed client, not a second installer. Other languages
use the same OpenAPI contract and supported generator. Go installation policy,
resolution, native-client adapters, profile authority, ownership, receipts,
journals, and recovery remain authoritative.

Agent Teams is the first desktop/server consumer. Reuse its Extensions shell,
Plugins/MCP/Skills/API Keys tabs and shared Radix controls. Preserve existing native
Claude plugin identity and local Skills authoring. Add a separate UAP-managed
contract, not a cast into the native marketplace type.

Project installation adapters are a separate owner-directed workstream. This plan
does not recreate or merge unfinished adapter work. Capability negotiation must
accurately reflect the engine used. Unsupported project/local scopes are refused,
never silently downgraded. Complete project/worktree qualification depends on that
workstream being merged and qualified.

## Research baseline and selected tools

Inspected UAP main 4555aa53844ee0078e9036cd82638afd67f77c16 and Agent Teams main
23617c3e825f775ea27d354d03e17a28bf612586. Recheck moving prerequisites before final
integration. UAP project PRs #472/#475/#480 and Agent Teams Host migration #982
were open drafts at this baseline.

| Concern | Selection | Why |
| --- | --- | --- |
| Contract | OpenAPI 3.0.x | Existing editable schema + generated DTO convention |
| Go transport | standard net/http + oapi-codegen v2.8.0 models | Reuse current loopback service and pinned generator |
| TS wire types | openapi-typescript 7.13.0 | Already adopted; types derive from one schema |
| TS HTTP | openapi-fetch 0.17.0 | Typed fetch, injectable transport, no framework coupling |
| TS runtime/tooling | repository-compatible Node, TypeScript 7.0.2 or newer | New scripts are .ts; do not copy legacy .mts conventions |
| Progress | poll authoritative operation snapshots | Existing service pattern; no extra stream protocol required |
| Electron automation | existing raw CDP harness, pnpm dev:mcp | Real desktop renderer, same maintained harness |
| Future languages | documented OpenAPI Generator recipe | Do not maintain unused language SDKs |

Research sources and inspected examples:

- https://github.com/oapi-codegen/oapi-codegen/tree/v2.8.0/examples/only-models
- https://github.com/oapi-codegen/oapi-codegen/tree/v2.8.0/examples/minimal-server/stdhttp
- https://openapi-ts.dev/openapi-fetch/
- https://openapi-ts.dev/openapi-fetch/api
- https://openapi-ts.dev/openapi-fetch/middleware-auth
- https://openapi-generator.tech/docs/generators/
- https://typespec.io/docs/emitters/clients/introduction/
- https://connectrpc.com/docs/introduction/
- https://google.aip.dev/151
- https://google.aip.dev/155

Alternatives: Connect is a reasonable protobuf-first RPC platform but would add
another authored contract and migrate a working HTTP/OpenAPI foundation. TypeSpec
adds another DSL and its language-client emitters are documented as preview.
Neither improves this first consumer enough to justify the extra toolchain.
Custom JSON-RPC, FFI, WASM, and CLI-output scraping are rejected for this work.

Code generation gives transport consistency, not installation safety. Request
validation, stale checks, idempotency, cancellation, and recovery must be explicit
application behavior with executable acceptance tests.

## Architecture and ownership

1. Composition roots inject the supported client registry and concrete host
   dependencies. Generic code must not import clients/all.
2. Share typed application preparation with CLI and Skills picker. Start from
   PreparedSkillsBatch, environmentSession, dependencyInstaller, and Coordinator.
   Extract only the conversations needed by management; no wholesale CLI rewrite.
3. Management transport owns bounded JSON validation, auth, Host/Origin checks,
   routing, HTTP status, and generated DTO mapping.
4. Management session owns retained prepared handles, expiry, accepted-operation
   observation, and process lifetime. Installer owns native effects and recovery.
5. Pure TS entrypoint owns typed requests, error conversion, wait/onProgress, and
   optional injected fetch. No filesystem, Electron, DOM, React, or Zustand.
6. Separate Node entrypoint starts an explicitly supplied pinned executable without
   shell, validates bounded bootstrap/version/capabilities, retains private bearer
   credentials, and owns only its spawned process.
7. Agent Teams owns current project/profile/runtime selection, actual execution
   cwd, active-run policy, and UI. Both IPC and HTTP call the same feature facade.

Initial public surface:

- session/capabilities and engine/API version;
- verified catalog search/details and standalone-Skill candidate discovery;
- environment desired/lock/observed state and supported actions;
- plans.prepare for install, skill install, update selected/all, locked sync,
  repair, remove, and recovery;
- plans.apply with opaque plan ID and request ID;
- operations.get, operations.wait in SDK, and explicit operations.cancel;
- fresh inspection and retained recovery preparation.

Finalize exact field names in a reviewed OpenAPI schema before integration.
There is no arbitrary execute-command or arbitrary-filesystem endpoint.

## Lifecycle invariants

### Prepared authority

Installer Plan has operational fields excluded from JSON. Inspection/recovery also
contains typed authority. Retain the actual Go prepared object/observation and expose
an opaque identifier. Diagnostic DTOs are never replayed as executable authority.
Check again before effects. Unaccepted plans expire after a bounded TTL and are
invalid after process restart. A new plan requires fresh preparation.

### Accepted operation identity and restart

Accepting an operation is a durable boundary. Persist a small acceptance record
with request identity, input fingerprint, environment identity, and correlated
installer operation identities before starting effects. Existing installer
journals/receipts remain the authority for native effects; do not create another
transaction engine.

Same request ID + same accepted input returns the known operation/result or an
explicit interrupted/recovery-required state. Same ID + different input conflicts.
Lost HTTP responses never justify blind resubmission with a new request identity.
After restart, inspect/reconcile prior receipts and require fresh recovery/sync
preparation. Never deserialize and automatically execute a saved plan.

The manifest write is itself a durable effect. TOML publication can succeed before
lock publication fails. Error and cancellation outcomes must cover this case even
when no native binding has committed.

### Cancellation and progress

An HTTP AbortSignal stops waiting, not an already accepted install. Explicit cancel
requests cooperative cancellation. Completion may win a race. Before durable
effects a clean cancellation is possible; after effects report partial/incomplete
or recovery-required with truthful next actions. Do not promise global rollback.

One active mutation per environment is sufficient for v1. Use existing locks and
return busy/conflict rather than inventing a scheduler. Progress reports actual
phase text and binding outcomes, not fabricated percentages. Operation snapshots
are authoritative even if a progress callback or UI poll is lost.

### Sources, updates and ownership

Use the signed directory publication through UAP. Do not parse the frozen registry
index in Agent Teams. Preserve distribution identity, release policy, suspension,
revocation, immutable pins and provenance.

Update selected/all constructs one candidate graph and lock, preserving unselected
pins. Extend the existing single-alias resolver selection; do not implement bulk
update as a TS loop. Exact pins are explicit. Distinguish available/current/pinned/
blocked/check-unavailable states; unavailable is not up to date.

Standalone Skills use the same uap.toml/uap.lock lifecycle. Local authored Skills
remain local. UAP-managed native projections must be protected from legacy local
edit/import/delete on the backend. Offer explicit editable copies with separate
ownership and destination.

### Host context and portability

Hosts provide explicit profiles/roots/executables. Ambient HOME is not an authority
for choosing an app profile. The renderer submits identifiers and intent; trusted
host composition captures and validates context. Navigation does not retarget
accepted work. Context revisions invalidate stale preparation.

A Node caller sends the precise Origin required by the loopback bootstrap contract.
Do not remove existing Host/Origin checks. Tokens do not reach the Electron renderer,
browser storage, command-line arguments, diagnostics, or progress logs.

Browser consumers use their trusted application backend to reach UAP. They do not
launch a binary or obtain arbitrary local filesystem access. HTTP and IPC adapters
must preserve equivalent validation and outcomes.

## Scenario and acceptance matrix

All rows must have an implementation/qualification status and evidence before
claiming support. Dependency-blocked rows remain visibly blocked.

| ID | Scenario | Required outcome / evidence |
| --- | --- | --- |
| S01 | Catalog/search/detail | Real verified catalog; distribution identity and policy reasons preserved |
| S02 | Registry plugin install | TS -> real Go -> saved manifest/lock -> actual native files/config -> authoritative inspect |
| S03 | Standalone Skill install | Candidate selection, immutable pin, managed ownership and native discovery |
| S04 | Local Skill authoring | Existing local create/edit/import/remove continues for unowned destinations |
| S05 | Managed Skill protection | Backend rejects direct edit/import overwrite/delete; explicit local copy works |
| S06 | Add client target / separate profile context | Source pin reused where supported; same-client profiles use separate captured environments; no sibling overwrite |
| S07 | Update one/selected | One candidate graph; unselected pins unchanged; per-binding results |
| S08 | Update all | Saved moving dependencies included; exact pins/blocked/unavailable reported |
| S09 | Locked restore | Exact content and normalization restored into a fresh environment |
| S10 | Remove | Only selected owned bindings removed; retained data and sibling state reported |
| S11 | Repair/drift | Recorded pin used; modified/unowned content never silently overwritten |
| S12 | Source/alias conflict | Actionable refusal before effects |
| S13 | Lost response/repeated apply | Same accepted request observed; no duplicate mutation |
| S14 | Conflicting request ID | Same ID with different input rejected |
| S15 | Process restart after acceptance/effect | Interrupted state correlated to receipts; no blind replay; recover/sync verified |
| S16 | Cancel before durable effect | Cancelled with no manifest/lock/native changes |
| S17 | Cancel after durable effect | Partial/incomplete or complete result wins truthfully; recovery next action |
| S18 | Changed root/profile/manifest | Stale plan rejected; fresh prepare needed |
| S19 | Expired plan | Old handle rejected without effects |
| S20 | Missing runtime/auth/manual activation | Materialized distinct from ready; specific action and inspection |
| S21 | Offline/unavailable catalog | Installed state usable; freshness explicitly unknown |
| S22 | Revoked/suspended/unsupported source | Policy refusal and safe remaining actions preserved |
| S23 | Multiple projects/tabs | A's late response cannot overwrite B; operation remains captured in A |
| S24 | HTTP/IPC | Same feature facade, parity validation/outcomes; no renderer token leakage |
| S25 | Project scope | Qualified project adapter + explicit root; no fallback to user scope |
| S26 | Worktree launch | Exact locked environment materialized at actual execution cwd before new run |
| S27 | Active affected profile | Mutation refused/deferred before effects while a run uses shared roots; unaffected profiles remain usable |
| S28 | Bad executable/version/process exit | Bounded startup, useful error, cleanup, no latest download |
| S29 | Electron install/update/remove/reopen | Real clicks, screenshots, actual UAP process and native artifact assertions |
| S30 | Packaged app | Pinned UAP executable packaged/resolved for supported OS/architecture |

## Agent Teams implementation and UX

Use src/features/extension-management with only layers that own real behavior.
Keep native EnrichedPlugin and external installations intact. Preserve existing
tab order and pinned project context. Reuse search, Radix primitives, Markdown,
status and diff presentation. Add a small managed-entry view model/hook rather than
rewriting the store or extending frozen oversized extensionsSlice/SkillsPanel.

Show current scope and actual agent runtime near the action. Capability-based
disabled actions include their reason. Prepare as part of showing details; ordinary
install uses one clear action. Review meaningful dependency/permission/target
changes, without a mandatory wizard for every install. Recovery/partial outcomes
must remain accessible after navigation and restart. Put receipts/digests in
expandable diagnostics.

Record a narrow new ADR extending remote managed Skills while preserving the
accepted separate Skills domain and local authoring. Existing draft
docs/extensions/plugin-kit-ai-integration-plan.md supplies product context, but its
obsolete engine/CLI assumptions are superseded.

Host migration #982 remains a dependency to recheck. If merged, wire UAP below its
captured context and mutation owner; do not bypass it with a second writer. Do not
cherry-pick the whole draft migration for this task.

## Contract decisions after independent review

### Immutable context and profile isolation

A management process/session binds one immutable trusted-host context: manifest/
lock root, state/managed roots, scope and explicit client profile/executable mapping.
A second profile for the same client is a separate captured environment unless the
engine explicitly supports multiple profile bindings. Do not add a speculative
profile dimension to the client-ID-keyed dependency graph. Existing physical-root
ownership checks remain authoritative; conflicting roots are refused.

### Acceptance ordering, storage and retention

Serialize acceptance lookup/create with an existing environment/process lock.
Session mutexes alone are insufficient across two UAP processes. Lookup a known
accepted request before resolving its old plan handle or enforcing plan expiry.

Namespace request IDs by captured environment identity. Keep request ID, immutable
context/input fingerprint, accepted plan ID, correlated installer operation IDs,
disposition and final public result/next actions. Store no credentials, operational
prepared objects, filesystem transaction copies or progress ticks. Persist before
effects; a persistence failure refuses acceptance. It is an observation record,
not an automatically replayed job queue.

Retention policy: keep terminal observations for at least 30 days; bound individual
record/result size and history. Never prune unresolved/recovery-required records by
display-history limits. Expired/unknown observations return a structured not-found/
expired result and require environment inspection; they never imply no effects.
A request ID is never silently recycled within retained acceptance history.
Document exact limits in the contract and test them.

### Bulk resolution policy

Manifest exact pins remain unchanged with a pinned explanation. Eligible moving
selections resolve together into one candidate graph; failed acquisition, dependency
conflict or validation refuses the candidate before acceptance. Excluded pins and
unsupported/policy-blocked entries appear with stable reasons in review. An unknown
update check is not current. No largest-installable-subset solver and no silent
best-effort graph loop. Partial results refer to effects of an accepted plan.
An exact lock revision does not mean manifest intent itself is pinned.

### Reproducible artifacts without public release

Build the SDK tarball from an exact reviewed UAP source commit. Vendor that tarball
in Agent Teams with package identity, source SHA and SHA-256, following the existing
vendor/terminal-platform/sdk precedent. Pin the same source commit for native UAP.
Agent Teams build/CI builds that source with its declared Go toolchain and
read-only module resolution, then stages the executable into packaged resources.
End users receive a built executable; no runtime Go build or latest download.

Use the same pins for PR E2E and clean-checkout/package-consumer qualification.
Include license and third-party notices. Do not depend on an expiring Actions URL
or an unpublished registry version. A later move to public release artifacts uses
the separately authorized release process. Refresh source and package pins together
before final qualification.

### SDK lifetime, compatibility and qualification

wait() abort/deadline never kills the child or cancels accepted work. UI navigation
does not close the engine. Node close() is idempotent, asks its own process to shut
down gracefully, waits a bounded interval, and only then terminates that child if
needed. Interrupted accepted work stays discoverable for recovery. No unrelated
native process is killed. Test startup exit, bootstrap timeout, repeat close and
close during accepted work.

Protocol major is independent of package/engine version. Reject major mismatch.
Optional response fields are additive; unknown outcomes/actions are conservative,
never success. New request fields need supported capability/version. Map
materialization, activation, authentication, policy and verification without
flattening them into an installed boolean; include a manual-action response fixture.

CI runs pinned generation with --check. Explicit incoming request validation remains
required. Test the packed TS package from a clean consumer against real Go. Check
pure exports for accidental Node/Electron/DOM-storage dependencies. Add one small
Python standard-library protocol smoke against the real server (handshake, read,
fixture lifecycle and structured error), not a speculative maintained Python SDK.

### Application HTTP admission and live profiles

S24 covers only the new Agent Teams feature, as required by its feature standard;
it does not rebuild every legacy web extension API. Both adapters invoke the same
facade. Existing application CORS settings are not mutation authority. Add a narrow
management-route admission check: exact trusted Host/Origin and loopback client;
refuse unauthenticated non-loopback deployment unless an existing authenticated
trusted adapter supplies authority. Require bounded JSON mutation bodies. Test
hostile/unknown Origin/Host and broad CORS configuration before facade invocation.
Do not weaken UAP's own capability checks or introduce a global auth framework.

Current shared native profile files can be read by active agents. Therefore refuse/
defer affected-context mutations while a run uses those roots, with an actionable
stop/defer message; unaffected contexts are allowed. No false immutable-run promise.

### Completion and dependency gates

API/SDK can be independently mergeable when their advertised capabilities pass.
Full Agent Teams project/worktree completion still requires S25/S26 against a
merged, qualified project engine. Capability refusal tests are fallback evidence,
not a passing project installation. Never merge unfinished prerequisite adapters
or mark dependent scenarios passed to satisfy a checklist.

## Delivery sequence and review checkpoints

M0 Research and plan:
- Commit/push v1 and open draft PRs in UAP and Agent Teams.
- Independent architecture/lifecycle and frontend/UX critics.
- Address each finding explicitly, commit/push v2 before production implementation.

M1 Contract and smallest vertical lifecycle:
- Shared preparation/application seam and versioned headless HTTP startup.
- Generated Go/TS DTOs; pure SDK + Node helper.
- Before the first mutation: durable acceptance/result correlation, request fingerprint,
  installer operation IDs, cross-process acceptance serialization, interrupted state.
- Real fixture install, operation wait, combined lost-response + kill + restart retry.
- Acceptance lookup precedes expired/missing plan lookup; no automatic replay.
- Independent contract/code review before broadening surface.

M2 Complete management lifecycle:
- Catalog/Skills/environment, selected/all update, locked sync, repair/remove.
- Extend M1's acceptance correlation, cancellation and recovery to every lifecycle.
- Shared CLI/picker behavior, generated drift checks and second-language recipe.

M3 Agent Teams host integration:
- Captured context, pinned binary preparation, SDK adapter, IPC/preload/HTTP.
- Ownership guard in legacy Skills mutation path.
- Actual execution-directory seam and explicit capability gating.

M4 Frontend:
- Reused shell/components, managed entries in Plugins and Skills.
- Small normal install flow, update selected/all, partial/recovery UX.
- Independent UX critique and actual Electron clicking/screenshots.

M5 Qualification and delivery:
- UAP focused lifecycle/contract/CLI parity + required core/lint/generation gates.
- TS package typecheck/build/package-consumer test.
- Agent Teams typecheck/lint/build, architectural guards and focused behavior tests.
- Dedicated real Electron management E2E on test-only homes/projects and native
  discovery proof; packaged binary qualification where required.
- Independent final code review; resolve findings and rerun affected checks.
- Current-commit required CI, including Agent Teams Full qualification.
- Refresh prerequisite PR status, exact diff/reviews/checks; merge only qualified
  changes. No main bypass, forced push, fabricated pass, or unsupported feature claim.

Every commit is immediately pushed. Plans and evidence live in repository docs,
not only chat. New review findings are recorded in the next plan revision with
disposition and verification.

## Test and evidence strategy

Use deterministic local source/release fixtures for content versions and failure
barriers. Fixtures may provide input packages and isolated agent profiles, never
fake a successful SDK result or prewrite the installation being asserted. Test
public boundaries and filesystem/native observations; avoid mirrored unit tests.

Real Electron uses pnpm dev:mcp, its own discovered CDP port, existing CdpClient,
test-only userData/HOME/XDG/projects, screenshots, console errors, and actual
native configuration checks. No native folder picker, no user projects, no web-dev
substitute. A small native-agent handshake complements materialization tests.

Existing CI does not automatically click the new UI. Add a focused management E2E
entrypoint and workflow path triggers. GitHub Full qualification remains mandatory.

Initial local environment has a full disk. Isolated temporary in-memory workspace
was authorized and cloned successfully; production/test execution still needs
toolchain/dependency preparation. Do not claim any test passed until executed.

## Open questions to settle by experiment

| Question | Resolution method / owner |
| --- | --- |
| Minimum shared extraction without Cobra leakage | Implement S02 vertical slice; review exact dependency ports |
| Acceptance/result persistence and crash boundary | Correlate current installer OperationID/receipts, deterministic S15 barriers |
| Manifest succeeds but lock publication fails | Existing publication path fault injection; truthful partial/recovery result |
| General managed Skill ownership inventory | Inspect authoritative binding materialization roots; server-side bypass tests |
| Cross-repository artifacts | Decided: exact-source SDK tarball vendored with digest; same source builds native binary during app build/CI |
| Project/worktree readiness | Recheck separate project PR qualification; keep capability truthful |
| Host migration timing | Recheck #982 before final integration; bind beneath captured ownership if landed |
| Cross-platform binary launch/package lifetime | Node helper contract and existing packaged matrix; no implicit latest |
| External examples and generator behavior | Run pinned generation with --check and compile small actual SDK consumer |

## Anti-overengineering budget

Keep one wire contract, one transport, one Go engine, one mutation per environment,
polling, existing journal/receipt authority, and one maintained language SDK.
Do not add a daemon, scheduler, dual transport, generic remote callbacks, global
atomic rollback, universal OAuth/configuration framework, five unused SDKs, a new
installer, or a complete frontend rewrite.

Original planning budget: roughly 3,500–6,500 new handwritten production LOC across
both repos, separate generated schema/types and focused tests. This is a planning
range, not a target; prioritize shared behavior and delete unnecessary wrappers.
Any material expansion requires a recorded reason and critic review.

## Review log

- v1: source/primary-tool research incorporated; three read-only research reports
  identified HTTP parity, native identity preservation, backend Skill ownership,
  retained recovery authority, manifest publication effects, and durable request
  correlation. Independent review of this written plan is the next checkpoint.

### v2 critic dispositions

- Lifecycle B1/M1: moved durable correlation and cross-process acceptance into M1;
  specified retry lookup before missing/expired plan checks.
- Lifecycle M2: immutable context, separate same-client profile environments.
- Lifecycle M3: explicit batch pin/refusal policy; no best-effort subset solver.
- Transport R1 / lifecycle M4: reproducible vendored SDK + same-source native build.
- Transport R2/R3/R4/R5: combined restart retry, shutdown semantics, actual Python
  smoke, packed-consumer/generated drift qualification.
- Transport R6: new-feature IPC/HTTP parity only, mandated by app standard.
- Lifecycle M5/M6: dependency completion gate and limited observation persistence.
- UX: enforce active-profile mutation refusal and strict management HTTP admission.
- Compatibility/result terminology: conservative unknown handling and readiness axes.

No critic requested another transport, installer, scheduler or frontend rewrite.
All mandatory written-plan findings known at this revision have been incorporated;
implementation review remains a separate gate.
