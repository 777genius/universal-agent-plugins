# Transport-independent environment application

Status: implemented locally; focused validation passed; draft PR #590 remains
subject to hosted checks and independent review. The plan was published in
`efd979ed98da178922eca4045ce9a2720674eb15` before implementation began.
Base: `ae068426706d5f2c9d624ae94c6504ac8b7f553e`.
Branch: `refactor/environment-application-20261030`.

## Evidence and problem

The domain-level `environment.Coordinator` already owns graph resolution,
selected binding planning, ownership checks and single-binding application.
Those mechanisms should remain intact. The missing boundary is the application
protocol around a prepared environment change.

`cli/internal/agentpluginscli/management_apply.go:applyPreparedDependencies`
currently owns readiness checks, environment-lock acquisition, retained-review
rechecks, package materialization, intent publication and binding application.
Its callers are CLI synchronization, the Skills picker, management HTTP and
update-all. All four therefore depend on a protocol coupled to `App`, CLI
options and the concrete `dependencyInstaller` adapter.

`deps_apply.go` also embeds transport-independent responsibilities in
`dependencySyncRequest`: manifest/lock preimage validation, ordered intent
publication, partial-publication handling and selected-package filtering. This
is the same state whose publication drives management's durable-change result
and update-all's partial-failure report.

The existing source acquisition, strict standard loader, Directory provenance,
security evaluation, native installer and transaction state remain useful
infrastructure. Extracting their application ordering must not replace or bypass
their checks.

## Bounded implementation

1. Add `agentplugins/environmentapp` in the existing installer module. It will
   own applying an already-reviewed environment change, with typed prepared
   plan/result data and narrow ports for mutation readiness, environment locking,
   package materialization, file access and the existing binding mutator.
2. Move the intent preimage/publication policy and selected-lock filtering into
   this package. Keep the `uap.toml` before `uap.lock` ordering, exact current error
   guidance, successful-write preimage advancement and actual durable-change
   observation after publication failures.
3. Replace `applyPreparedDependencies` with CLI-side adapters and mapping. Route
   its existing CLI, picker, management and update-all consumers through the new
   service without changing their rendering, confirmation, options, scoped paths,
   per-binding guards or partial-result mapping.
4. Reuse the extracted intent policy in the remaining request helpers so import
   installation and retained preparations share the same preimage semantics.
   Keep agreement import's distinct rollback/identity policy in its existing
   adapter rather than flattening two different publication contracts.

The application protocol remains: check readiness; acquire the existing
environment lock; honor cancellation; check readiness again; recheck the retained
review; materialize the selected lock unless repairing; honor cancellation;
recheck intent/custom guards; publish intent; apply bindings through the existing
coordinator. Release the lock on every exit. Every binding preparation and apply
retains its current stale-input guard. Existing callers continue to own user
confirmation and presentation.

## Scope and preservation

- Do not change solver behavior, source preference, lock schema, receipts,
  transaction recovery, persistence layout, native client policy or security
  rules. Fresh candidate preference belongs to #584.
- Do not change frozen VS Code source, adapters, protocols, configuration or
  dedicated fixtures/tests.
- Do not add a DI framework, generic workflow engine, alternate installer engine
  or new supported public API/command.
- Preserve useful legacy capabilities under `AGENTS.md` and
  `docs/AUTHORING_CAPABILITY_PRESERVATION.md`; this slice removes no legacy
  capability.
- Existing project/native/E2E expansion remains owned by #520, #566 and #575.
  Existing relevant tests remain regression gates here.
- Coordinate with the root before touching npm/process or client-boundary lanes.
  No release or merge is authorized by this plan.

## Behavioral gates

Add transport-free tests for the real protocol risks: readiness failures before
and after lock acquisition; lock failure and release; stale retained reviews;
materialization failure; cancellation before publication; repair-only behavior;
manifest success followed by lock failure (including durable-change reporting);
write-success/error ambiguity; unchanged intent; selected-package isolation;
per-binding stale-input refusal; and preservation of completed installer results
when a later binding fails.

Run the existing environment/coordinator suite and the CLI suites covering saved
add, reconciliation, exact dry run, agreement import/install, Skills batches,
management application/recovery, install selection and update-all. Run formatting,
module-appropriate tests and the repository's relevant architecture/dependency
guards. Verify that `environmentapp` cannot import CLI, Cobra, HTTP, concrete
filesystem/process adapters or client-specific packages. Any additional gate
must resolve a concrete remaining risk, rather than duplicating existing E2E
ownership.

## Delivery checkpoints

1. Publish this plan immediately and open a draft PR before implementation.
2. Implement the application boundary and migrate existing consumers as one
   cohesive reviewable change, with risk-focused tests and evidence.
3. Root coordinates commits and immediately pushes each commit. Update the draft
   description and this plan with actual validation and remaining limitations.

## Implemented slice and validation

`environmentapp.Service` now owns the application ordering through narrow
readiness, environment-lock, selected-package materialization, intent-file and
existing binding-mutator ports. `environmentapp.Intent` owns preimage validation,
pending-change detection and the existing manifest-first publication policy.
`dependencySyncRequest` retains this one mutable intent object; successful writes
advance the same preimages that every later binding guard observes.

The CLI mapping moves an existing custom `beforeApply` guard to the service for
the duration of application, restoring it afterward. This preserves the Skills
context guard and executes one guard before each native preparation/application.
`environment.Coordinator.ApplySync` remains the sole binding loop. No resolver,
security, provenance, transaction, import rollback or native-client policy was
changed. The four callers already shared the protocol; this change makes that
protocol independently usable and testable outside the CLI package.

Passed locally with Go 1.26.10:

- `go test ./environment ./environmentapp -count=1` in the installer module.
  New tests cover readiness/lock/review refusals, cancellation, materialization
  failure, selected-package isolation, repair-only behavior, unchanged intent,
  partial writes and error-after-write durable effects, preimage advancement,
  custom guards, per-Prepare and per-Apply input checks, partial installer results
  and the application import boundary.
- CLI package compilation: `go test ./internal/agentpluginscli -run '^$'`.
- Existing CLI integration selection passed: `TestUpdateAll*`,
  `TestManagementMutableArchiveUpdate*`, the replacement/noop, invalid-source,
  confirmation-recheck and publication-rollback `TestDependencyTransfer*` tests,
  and `TestInstallScopeSelection`, `TestInstallPreviewShowsSelection` and
  `TestInstallAutoScopeRejectsNonregularManifest`. These use real archive
  acquisition/SDK persistence and controlled native runners. Loopback tests ran
  outside the network sandbox. The child environment removed duplicate lowercase
  proxy-variable names exported by this host; Codex's ambiguity check and all
  source/fixture assertions remained unchanged.
- Full `golangci-lint run ./environmentapp`: zero issues. CLI changed-line lint
  against `ae068426706d5f2c9d624ae94c6504ac8b7f553e`: zero issues. Unfiltered CLI
  package lint reports 112 existing findings outside this slice; no unrelated
  source cleanup or suppression was added.
- Formatting and `git diff --check` passed.

Host limitation: baseline and refactored real Git-backed tests stop before the
application path with `ErrOrdinaryContainmentUnsupported` because this Linux
host lacks the required `/proc` task-children interface. This includes real-Git
management/Skills preparation, exact-registry dry run and the dependency CLI
journey. Their assertions and production fail-closed process policy remain
intact. They require the existing qualified hosted Linux CI; these local results
do not constitute native-client or all-transport E2E qualification.

