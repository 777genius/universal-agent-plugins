# Portable package-manager E2E qualification

## Owner request and boundaries

Create one reusable, deep package-manager acceptance suite for Linux, Windows and
macOS. Execute it locally on Linux and in Windows CI; provide the same command
for the owner's macOS run. Investigate first, review the plan independently,
then implement and correct reproduced defects. Keep this pull request a draft
while qualification is in progress. Publish each commit immediately.

Use GPT-6.1 Sol with xhigh reasoning for researchers, critics and implementers.
Do not change disputed product semantics or introduce a large new subsystem
without the owner's decision.

## Research baseline

- Repository main at `77ed6c0412f67907609a43db9b5eeff111485b19`.
- Existing composed CLI dependency tests live in
  `cli/internal/agentpluginscli/deps_*_test.go`; the optional real-binary harness
  lives in `repotests/uap_dependencies_e2e_test.go`.
- Existing native-client qualification and VS Code-deferred work are separate.
- Open PR #563 owns the saved dependency update lifecycle. Recheck its status
  before asserting or changing update-selection behavior.
- The GitHub CLI is installed but has no authenticated account in this workspace;
  use the authorized GitHub connector for branch, commit and PR publication.

## Work stages

1. Inspect current resolver, lockfile, acquisition, store, transactions and E2E
   helpers. Compare suitable package-manager contracts using official sources.
2. Have independent critics review the scenario matrix, portability, expected
   failure behavior and scope. Record product decisions needing owner input.
3. Extend the existing test infrastructure with deterministic, disposable fixture
   repositories/registries and public CLI journeys. Prefer shared helpers to a
   parallel installer or resolver.
4. Run on local Linux, fix demonstrable defects with narrow regressions, and
   review the actual changes. Add a scoped Windows CI execution and one
   platform-independent local command, including macOS.
5. Publish exact execution evidence and instructions. Distinguish a true CLI
   process run from composed in-process CLI tests, and a passed run from an
   unavailable environment or skipped test.

## Proposed scenario families (subject to code research)

- Direct, transitive, diamond, wide and deep graphs; shared-node deduplication.
- Compatible constraints, incompatible constraints, candidate backtracking,
  deterministic selection, exact pins and prerelease policy.
- Self-cycle and longer dependency cycles according to the existing contract.
- Locked/frozen replay, stale or corrupt lock rejection, cold restore, immutable
  source identity, integrity errors, and intentional source unavailability.
- Idempotent repeated sync, explicit update, selected-root preservation, removal
  and shared-dependency retention.
- No durable writes on preflight rejection; honest partial outcomes and recovery
  when effects have already started; preservation of unrelated/user-edited files.
- Paths containing spaces and Unicode, platform environment isolation, portable
  digests, Windows filesystem behavior, bounded execution and actionable logs.

## Acceptance criteria

- One documented entry point runs the identical semantic scenarios on all three
  operating systems using disposable roots and no model/account credentials.
- Expected graph/package identities and resulting managed bytes are asserted;
  exit code alone is insufficient.
- No production safety guard is disabled for this environment. Unsupported host
  capabilities are reported explicitly and do not count as a pass.
- CI is path-scoped, bounded and reusable without changing repository protection
  or unrelated workflows.
- No new dependency solver, persistence format, daemon, UI, native-client feature,
  release or VS Code change.

## Status

Research is in progress. No implementation or qualification claim yet.
