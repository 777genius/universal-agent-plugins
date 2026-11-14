# Project component helper amendment independent review R2

Reviewer: project_core, 2026-10-23. Read-only amendment review before helper
implementation. Reviewed the complete amended contract, R1 findings/fix ledger,
existing nativeconfig committed-cleanup and shared native-effect semantics.

## Decision

**R2 GO after two narrow contract clarifications, independently verified in the
final amendment.** The explicit shared boundary is justified by the separately
reviewed Cursor and Codex project consumers. No new source loader, settings or
discovery engine, client dispatch table or durable recovery framework is needed.
Parent coding authorization remains separate from this plan review.

## R2-1 — source inputs and prepared server maps must be closed before effects

The frozen public mutation includes ActivePath, projected source-relative
directories, stagingRoot and a prepared server map. Their relationship must be
explicit rather than rely only on destination containment or receipt comparison.
Requested clean absolute no-follow project-child source roots, portable strict
child source-relative paths, and exact equality between the prepared server-name
set and desired MCP ownership names. Extra map entries must not authorize an
unowned write; missing entries must not become an accidental removal.

**Closed.** The final R2 clarification explicitly states these preconditions,
rejects aliases/traversal/absolute/drive/backslash/control inputs and adds
prewrite source/server-map tamper fixtures with unchanged user/A/B inventories.
Adapter native dialect/projection remains outside the helper, and desired
receipt equality is still checked before any mutation.

## R2-2 — cleanup may not discard changed recovery material

A private directory name and an earlier backup digest are insufficient deletion
authority after another writer changes a recovery copy. Requested physical
transaction identity and exact known child/digest revalidation before private
cleanup; unknown, replaced, aliased or modified copies must remain available.
After successful MCP commit, such refusal must be committed cleanup degradation,
not trigger rollback of already committed native configuration.

**Closed.** The final contract records physical transaction identity and exact
staged/backup digests; it rejects unknown entries, replaced roots, symlinks and
changed copies before deletion. Postcommit refusal retains an actionable
recovery path under CommittedCleanupError. The planned failure fixture changes
a backup at the successful MCP boundary and verifies committed objects remain
installed while the changed recovery copy and warning remain available.

## Accepted lifecycle and ownership boundaries

- Layout explicitly names selected project, skills root, exact config paths,
  codec and distinct ownership kinds. Original user ConfigRoot and planned
  native registry are validated by each adapter; shared helper containment and
  core engine/state scope/namespace checks are independent layers.
- All previous/desired objects have exact kind+name identity, paths and digests.
  Unknown kinds, duplicates, foreign same-name objects and disabled/conflicting
  entries fail closed. Missing same-pin skills are reconstructed; equal receipt
  digests alone never authorize a no-op.
- Skill moves/copies remain on one filesystem, staged and isolated backups are
  verified, publication is exclusive and installed skills are rechecked before
  a single existing kernel ApplyBatch. An MCP precondition race rolls skills
  back only through exact ownership checks.
- Observed complete rollback can report unchanged. Unknown/partial config
  commit remains uncertain even if skills restore; failed restoration preserves
  recovery copies. Successful commit and CommittedCleanupError both retain the
  desired skills; private cleanup failures cannot undo committed MCP state.
- Cancellation keeps its existing before-effect/after-effect distinction.
  No persistent plugin data, foreign files, unrelated native settings or user
  and project-B state is consumed by helper cleanup.
- Existing Gemini/Cline/OpenCode and user Cursor/Codex behavior is outside the
  new files. Future consumers need their own reviewed qualification. Project
  prepared/installed state does not claim native trust/discovery or execute MCP.

No code was implemented or edited during this review. The required contract
tests remain implementation gates, including rollback, postcommit cleanup
degradation, exact missing repair, aliases and source/server-map tampering.
