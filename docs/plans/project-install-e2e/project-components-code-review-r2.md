# Project component helper implementation independent review R2

Reviewer: project_core, 2026-10-23. Starting committed helper `91738de1`, final
frozen helper `f02663a56cf7b1cfd88f1b8fa70fb7aff64d1822`, including corrective
`5e987e979661cde95d178f7d2eba99cd6a7cafe1`. Reviewed only the committed new
clients/shared helper and contracts; uncommitted Cursor implementation is
excluded. The original amendment and both independent plan reviews remain
separate evidence.

## Decision

**R2 GO at f02663a5.** One portable-spelling finding is independently verified
closed. The planned MCP-only/no-skill path gained a focused positive contract
at the final checkpoint. No confirmed implementation blocker remains in the
shared helper. Consumer adapters, their native codecs, project qualification
and hosted discovery still need their own review and evidence.

## R2-1 — strict portable source and logical-name spelling

At 91738de1, componentRelative rejected empty/dot/traversal/drive/backslash and
control paths, but did not apply the existing portable segment rules. Trailing
dot/space, Windows reserved device names, wildcard characters, invalid UTF-8
and non-NFC names could pass this helper despite its frozen portable-child
contract. ValidateObjects and inspection called ValidateLeafID, whose legacy
internal trimming accepts padded logical names without changing the original
name later used in a physical path.

**Closed in 5e987e97.** Each source-relative segment now passes the existing
pathpolicy.ValidatePortablePathSegment; original logical names must exactly
equal their unpadded spelling before leaf validation in ownership/inspection.
The negative contract covers the listed portable forms plus dot/traversal and
duplicate separators, and verifies padded ownership is rejected. No new path
grammar or compatibility exception was introduced.

## Mutation and ownership assessment

- Explicit selected project, source/staging roots, skills root and exact
  config candidates are clean absolute no-follow project children. Desired
  and previous ownership has exact kind/name/path/digest; unknown kinds and
  duplicate IDs are refused. Source directories are checked inside the active
  immutable package and digested before staging and after copying.
- Missing same-pin native skills are reconstructed. A no-op requires an
  observed exact present directory, rather than only equal receipts or
  allowMissing success. New skill/server names collide with unowned objects.
- Prepared MCP map cardinality and per-desired-name lookup jointly establish
  exact name-set equality. DesiredReceipt is recomputed from the selected
  codec/server/path and must match staged ownership before skill/config writes.
  The helper does not read descriptors, expand placeholders, discover clients,
  execute MCP, grant trust or substitute a user profile.
- Skills are staged/backed up/published on one filesystem with exclusive
  rename and digest checks. The helper rechecks installed skills/cancellation
  before one existing Kernel.ApplyBatch. No alternate native config writer or
  solver is introduced.
- Prior exact whole-entry MCP receipts are checked before staging and again
  by the kernel's locked mutation preconditions. Current kernel ErrCollision
  and ErrNotOwned occur before config publication; complete observed skill
  rollback can report unchanged in those cases. Other kernel failures remain
  conservatively uncertain, including an unknown config commit state.
- Failed rollback does not overwrite/remove changed external installs and
  retains exact old backups with a recovery location. Private cleanup checks
  recorded physical transaction identity, exact known entry set and digests;
  changed/unknown recovery material is retained.
- Successful config commit, committed lock-cleanup degradation and subsequent
  private-cleanup refusal retain installed desired skills. Cleanup degradation
  is CommittedCleanupError and cannot roll back committed native state.
- MCP-only input calls the existing kernel without creating a skill root or
  private transaction. Removal tolerates missing exact owned objects without
  consuming foreign disabled entries. Existing clients and user lifecycle
  code are untouched by the committed shared-only diff.

## Independent execution

Ran exact frozen shared contracts with Go1.26.10, `GOMAXPROCS=3`:

```sh
go test -p=2 ./install/integrationctl/agentplugins/clients/shared \
  -run '^TestProjectComponents' -count=1 -timeout=2m -v
```

**PASS0.058s.** Execution covers add/repeat/missing same-pin repair/update/repeat
remove; MCP-only add/update/verify/remove with no skill root; source/map/receipt
tampering before effects; contained skill/config aliases; strict portable
spelling; MCP failure restoring old skill; changed external install preserving
backup and uncertain effect; committed cleanup preserving modified/unknown
recovery material; and cancellation before mutation. Foreign disabled MCP data
is retained.

No production or test file was edited by this reviewer. No real native client,
model, MCP process or hosted certification was executed. Architecture/whole-file
lint and consumer-specific end-to-end qualification remain applicable gates.

## Whole-file gate follow-up

Independently inspected `cc40930d866cf050c2036da3ceb0c70b8efb2b35`. The new
validateObjectPath function is an unchanged extraction of the existing
kind-specific exact path/source checks followed by clean-path and no-follow
containment checks. ValidateObjects still performs layout, unpadded/leaf name,
exact/unique ID and nonempty digest checks before that call, in the same order.
No policy, public API, error classification or mutation code changed.
**R2 GO remains valid through cc40930d.** Author's final exact changed-code and
whole-file gates report zero issues; no baseline was expanded.
