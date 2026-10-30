# Transport-independent environment application

Status: planned; publish this plan and open the draft PR before implementation.
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

