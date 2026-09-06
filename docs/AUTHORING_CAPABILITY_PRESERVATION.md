# Authoring capability preservation

Accepted owner clarification, 2026-09-06. This controls legacy treatment in the
[implementation plan](./STANDARD_FIRST_AUTHORING_ENGINE_IMPLEMENTATION_PLAN.md).

- A narrower portable standard is not a reason to discard useful plugin.yaml code.
- Retiring a command from standard-first help/wiring does not delete its service.
- Preserve useful implementation, required dependencies, tests and design docs.
- Keep it outside the standard authoring dependency graph; no implicit YAML reads,
  temporary YAML conversion or portable identity overridden by legacy metadata.
- Reuse/adapt neutral services when a planned consumer needs them. Avoid speculative
  frameworks, archive-only moves and automatic registration of unsupported commands.
- Before any deletion, inventory implementation, tests, consumers, exact mapping,
  preservation destination and support status. Dispositions are reuse, adapt,
  preserve/defer, or explicitly owner-approved removal. Unresolved means preserve.
- No current standard-first caller is insufficient evidence to delete a capability.
- Historical binaries and Git history alone do not satisfy source preservation.
- Keeping code does not promise a maintained second YAML authoring product. That
  remains a separate explicit decision; this clarification does not expand scope.

Inventory candidates include client generation, launcher/runtime orchestration,
native import, archives/bundles, publication inspection and marketplace
materialization, external Skills lifecycle, and integration sync/enable/disable.
Their current command dispositions do not pre-approve deleting implementations.

Workers and reviewers must read this contract before changing legacy code. Record
the exact inventory item and owner decision in any removal PR. Verify standard
dependency guards and preserved-code tests. If a task only changes current shared
CLI or npm preparation, continue it without touching legacy retirement.
