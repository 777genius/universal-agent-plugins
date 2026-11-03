# Management application boundaries

Date: 2026-11-04. Research base: `999c25fcd22ab0cbd4c775aa6acc83e7cf422c3a`.
Branch: `refactor/foundation-boundaries-20261104`.
Status: research and three independent plan reviews complete; baseline E2E first.
PR: https://github.com/777genius/universal-agent-plugins/pull/700.

## Goal and scope decision

Give the existing Management protocol one transport-independent owner for
retained reviews, durable acceptance, execution, cancellation and recovery
observation. HTTP must adapt that owner rather than implement its state machine.
The filesystem adapter must persist observations rather than decide when
uncertainty or private undo data can be forgotten.

The owner authorized a substantial refactor, including 15–25 thousand changed
lines if justified, immediate publication of each commit, a detailed plan and
repeated independent criticism. Line count is not an acceptance target. We will
change the smallest complete ownership boundary established by the audit.

Three independent research lanes examined installer/application ownership,
CLI/Management integration, and frontend/SDK architecture. They agreed that
Management has an existing second workflow (local Skill authoring) sharing its
delicate acceptance protocol. This is a demonstrated need for a common owner.
One reviewer preferred extracting the shared CLI dependency orchestration first;
that alternative overlaps active dependency work in PR #697 and changes a
different authority boundary. This plan deliberately leaves that adapter intact.

## Research findings and priorities

| Priority | Evidence at the research base | Decision |
| --- | --- | --- |
| P1 | `management/server.go` combines listener/token/HTTP lifecycle with retained plans, running operations, concrete process lock and store. `plans.go` interleaves decoding/writing with replay, revalidation, acceptance and worker creation. | Extract application ownership, preserving the existing protocol. |
| P1 | `management/store.go:reserve` applies retention policy and calls backend private-undo cleanup; `operations.go` and `reconciliation.go` make recovery decisions on an HTTP receiver. | Move retention and reconciliation to the application; storage only validates and reads/writes records. |
| P1 | Management SDK tests build `test/fixtures/host.go`, not the production CLI. Backend and real-socket tests exist separately. | Add one actual CLI + SDK process journey covering local files and restart. Keep existing lower-level tests. |
| P2 | `pr-changes.yml` includes `management/` but excludes sibling `management_*.go`; current architecture gates do not enforce a transport-free Management owner. | Cover the entire affected seam in CI selection and executable import rules. |
| P2 | `ManagementBackend` and prepared bindings retain the broad CLI `App` and private dependency orchestration. | Record for a separate refactor after dependency PR #697; do not create an enormous replacement interface here. |
| P2 | Landing `useRegistry.ts` combines feed/cache lifetime, scoped state and deferred DOM interaction; failure memoization and disposal deserve characterization. | Separate frontend work; no landing edits in this plan. |
| Preserve | `environmentapp.Service` already owns readiness, locking, recheck, materialization, publication and guarded bindings. Installer composition and transaction recovery already have explicit boundaries. | No kernel, installer, resolver or environment application rewrite. |
| Preserve | TypeScript SDK already separates HTTP exports from Node process hosting. Runtime Go SDK separates generic execution and platform codecs. | No SDK runtime redesign or generic process framework. |

The baseline Management subtree contains 2,315 non-test lines; the surrounding
CLI package contains 26,299 non-test lines across 169 files. Size alone is not a
defect. The evidence above concerns dependency direction and policy ownership.

## Package design

Keep `cli/internal/agentpluginscli/management` as the existing internal contract
and new application package. Its generated DTO file and generator destination
stay unchanged. Existing `Backend`, `Prepared`, `Outcome` and sanitized
`Refusal` remain the installer adapter contract. These data types represent this
protocol; a second manually maintained domain copy of every DTO is unnecessary.

Add sibling packages:

| Package | Owns | Dependencies |
| --- | --- | --- |
| `management` | `Application`, captured session identity, input bounds, retained review authority, acceptance, execution, observation/retention/reconciliation policy; consumer-owned persistence and lock ports | Standard library only; no HTTP, filesystem I/O, CLI, loopback, concrete store or lock adapters. Generated contract data is deliberately shared. |
| `managementstate` | Existing atomic JSON observation format and process-lock implementation | `management`, existing `atomicfile` and `processlock`; no HTTP/CLI orchestration. |
| `managementhttp` | Listener, private capability guard, routes, strict JSON/body limits, status/error rendering, bootstrap origin/token | `management`, existing `loopbackapi`; no concrete store/lock or installer construction. |
| `agentpluginscli` | Existing installer-facing backend and CLI process composition | Contracts/application plus explicit state and HTTP adapter wiring. |

No new module, external dependency, framework, service locator, event bus,
generic repository, durable job queue, feature flag or public command is needed.
`Application` avoids collision with the generated `Session` response type.

The store port exposes only lookup by request/operation, list, save and delete.
An observation is still a version-1 correlation record, never executable
authority. The lock port exposes nonblocking acquisition and classification of
an active lock error, preserving existing diagnostics without importing the
concrete process-lock implementation into the application. Active ownership must
return an error, not wait: observation inside the second replay check can attempt
the same process lock. One acquisition spans replay, revalidation, admission,
retention and acceptance, then transfers to execution through final persistence
and reconciliation. Every early return releases it.

The application exposes the existing operations as methods returning existing
DTOs and typed refusals: session/catalog/inspection/update reads, ordinary and
local preparation, apply, discard, observe and cancel. HTTP request validation
keeps its decoding/size/media-type responsibilities; bounded semantic input
checks live with the corresponding application operation. Capability checks
must preserve their current error precedence before decoding optional routes.
Existing status/code/message/retryability remain compatible. Numeric refusal
status is retained as existing contract metadata, not used for HTTP writes in
the application.

## Protocol invariants

1. Request replay is checked before retained-plan lookup or expiry, then checked
   again under the cross-process mutation lock. Same request plus same plan
   returns the original operation. Reusing the identity for another plan fails.
2. Prepared authority is rechecked before durable acceptance. No worker or
   effect starts until acceptance saving succeeds. A save error may nevertheless
   leave an acceptance record visible after atomic publication; retain the same
   request identity, preserve that record, and observe it without repeating
   effects. Test failure before writing and publication followed by an error.
3. Accepted execution derives from the application lifetime, not an HTTP
   request. Losing a response or canceling a polling request does not repeat or
   cancel accepted work. Explicit cancellation and shutdown retain their meaning.
4. Shutdown closes admission before waiting, cancels operations, waits for
   admitted calls and workers, closes unaccepted preparations and closes the
   backend. Ownership transfers once from review to running operation. Close is
   idempotent; no WaitGroup Add/Wait race or double Prepared.Close is introduced.
5. Expiry removes only unaccepted reviews and does not race active preparation
   or application. Sixteen retained reviews still permit applying one of them.
6. Restart observes records; it never runs stored jobs. Abandoned accepted work
   becomes Interrupted only after acquiring the lock and rereading the record.
7. Unknown effects block unrelated mutation. Corrupt/unreadable history fails
   closed. Queued/running/interrupted/partial effects, and failed/canceled work
   with mutation or follow-up recovery actions, do not age out.
8. Native recovery settles only records for the same intent root after fresh
   backend proof. It cannot settle local-file effects. Local restoration matches
   the exact operation, authoring root and Skill name and cannot settle native
   uncertainty. Recovery links preserve the original result.
9. Retention remains thirty days with the existing 1,024-record admission limit,
   bounded directory scan and 1 MiB record limit. Private undo data is verified
   and forgotten by its backend before deleting its observation; a refusal
   retains that record without blocking new work below the budget.
10. Panics, invalid/nonterminal outcomes, missing results and unsuccessful final
    persistence remain uncertainty. A successful HTTP call must not manufacture
    installation, activation, authentication or recovery proof.
11. JSON version/fields, SHA-256 filenames, environment partitioning, regular-file
    and symlink checks, atomic writes and 0700/0600 modes remain compatible.
12. `manage serve` keeps one strict startup JSON line (64 KiB), private IPv4
    loopback bootstrap, and stdin-EOF shutdown. `manage discover` remains passive.
    OpenAPI, generated DTO bytes, SDK public API and installation semantics stay
    unchanged.
13. The application owns retained snapshots. Direct callers cannot change a
    retained review or operation through returned slices/pointers. Detach the
    retained review and operation data that crosses this new boundary; do not
    invent a second model or blanket-copy every backend response.

### Shutdown and construction ownership

The application owns backend cleanup after successful construction; the caller
owns cleanup before that handoff. The HTTP server owns the listener and its
successful application handoff. Listener-start or bootstrap-write failure closes
the constructed application and any opened transport. HTTP Close first stops
transport admission and closes connections, including body decoders, then closes
application admission/cancels work and waits for admitted calls and operations.
Application cancellation also closes transport through the same idempotent path.
`Done` signals completed cleanup. No second operation/worker owner is introduced.
Tests must cover a blocked admitted call and failed process startup output.

## Implementation and publication sequence

### 0. Plan and baseline

- Publish this researched plan before production edits, open a draft PR, and
  have independent critics review scope, lock/lifetime ownership and E2E design.
- Record baseline Management tests and environment limitations. The complete
  Management HTTP package passes on Go 1.26.10. Broad CLI Management tests expose
  existing host restrictions around supervised Git and duplicated environment
  names; do not report them as passes or change product policy to accommodate the
  container. Normalize only the test process environment where appropriate and
  use existing hosted gates for native/protected-process qualification.
- Before production extraction, add and run the deterministic actual CLI + SDK
  journey described in phase 3 as a standalone characterization commit. Use
  explicit disposable captured paths and a separate authoring root without a
  target binding. Assert `local_skill` capability, not native availability. Use
  an explicitly bounded child environment. Preparation must not mutate the
  authoring tree or dependency manifest/lock; existing lock-directory creation
  is not prohibited. Rerun this same journey after extraction.

### 1. Separate contracts, observation storage and application ownership

- Retain generated contracts in their current location. Move the custom
  required-nullable local Skill JSON decoder alongside those contract types.
- Introduce consumer-owned observation/lock ports and the concrete state adapter.
- Move replay/admission/acceptance/execution/recovery/retention to `Application`.
  Preserve call ordering, bounds, messages, effect ownership and durable format.
- Remove HTTP response writers and concrete filesystem/process-lock dependencies
  from the application package. Do not leave a second old lifecycle owner.
- Adapt existing focused policy tests. Add failure injection only for critical
  acceptance/lifetime behavior not already covered.

### 2. Thin HTTP adapter and explicit composition

- Move listener/guard/routes/decoding/HTTP rendering to `managementhttp` and
  delegate every operation to the application.
- Wire state adapter, lock, application and HTTP server in the existing CLI
  composition path. Preserve bootstrap/stdin/shutdown behavior and backend
  cleanup on construction failures. Update all in-repository consumers.
- Keep existing HTTP contract tests as real socket tests. Move direct policy
  assertions into application tests instead of exposing private state for tests.
- Publish coherent compiling increments immediately; do not accumulate local
  commits waiting for all remaining work.

### 3. Production E2E and architecture enforcement

- Keep the baseline TypeScript SDK journey launching the actual built CLI
  with isolated roots. Exercise session authentication, local Skill
  create/edit/delete, exact request replay, explicit close/restart, observation
  and exact restore, including one small binary file. Assert preparation has no
  effects and local authoring does not create dependency manifest/lock state.
- Reuse the existing SDK workflow and consumer checks. Provision the real source
  CLI for this journey explicitly; test-fixture hosts remain protocol tests.
- Extend path classification to production CLI/core inputs, loopback support,
  workspace/nested module manifests, management bridge files and both new
  packages, with positive/negative path cases. The actual CLI build makes a
  narrow management-only classifier insufficient. Include affected native-SDK
  triggers.
- Add a source-based import guard for application and adapter boundaries,
  including negative build-tagged fixtures. Keep current architecture guards and
  lint limits; no new suppression or legacy baseline expansion.

### 4. Independent review and completion

- Review the integrated diff with multiple independent critics: concurrency and
  authority, storage/restart/recovery compatibility, and transport/SDK/CI wiring.
- Fix every actionable in-scope finding, then review the fixes and changed
  interactions again. Continue until there are no unresolved blocking findings.
- Run focused uncached/race tests, real process E2E, generated-contract check,
  TypeScript build/typecheck/packed-consumer checks, applicable lint/architecture
  gates and hosted required checks. Distinguish local environmental failures from
  product failures and historical evidence from the current head.
- Update this plan with concrete review dispositions, commands/results and
  remaining limitations. Deliver the reviewed PR and exact published head.

## Explicit exclusions

No resolver work from #697, security/live-install work from #698, SDK docs #696,
README/landing work #699, production registry/SkillSpector changes, new client
support, project-scope qualification, frozen VS Code edits, legacy authoring
removal, installation/transaction kernel rewrite, release or deployment.
The existing `ManagementBackend` remains an installer adapter inside the CLI
package in this change; the plan does not claim the entire repository is now
free of architectural debt.

## Review and validation ledger

- Research: three independent reviewers; scope disagreement and its resolution
  recorded above. No production files changed during research.
- Baseline: `go test -count=1 ./cli/internal/agentpluginscli/management/...` PASS
  (Go 1.26.10, 0.394 s). Broad Management CLI baseline is not qualified locally
  because of the observed host/process restrictions described in phase 0.
- Plan criticism: all three independent reviewers accepted the chosen scope and
  package direction after these corrections: move real-process E2E before code;
  specify save ambiguity, nonblocking/continuous lock lifetime and shutdown
  ownership; cover actual CLI build inputs in CI; detach retained snapshots at
  the new direct-call boundary. No new framework or domain DTO duplication.
- Implementation reviews and final-head evidence: pending.
- Before production changes, the actual CLI + SDK characterization passed on
  Go 1.26.10: `node --experimental-strip-types --test test/production-engine.test.ts`
  from `sdks/typescript` (27.63 s including source build; one pass, zero failures
  or skips). SDK build and typecheck passed. The test also proves that replaying
  the earlier create after deletion/restart does not recreate files, preserves
  a different edit backup, and refuses a lost unaccepted review. A version-1
  observation emitted by that baseline binary was retained for storage
  compatibility testing.
