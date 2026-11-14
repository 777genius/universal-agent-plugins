# Project component helper amendment independent review R1

Reviewer: sources_lead, 2026-10-23. Read-only review of
`cursor-component-helper-amendment.md` before helper implementation. Scope:
Cursor and Codex project component consumers, existing nativeconfig Kernel,
no existing Gemini/Cline/OpenCode behavior refactor.

## Initial decision

The minimal helper boundary is accepted. Final R1 GO follows two narrow
contract clarifications below; no additional engine or client generalization is
requested.

1. **P2, missing skill repair versus no-op.** The optimization for equal prior
   and desired digests must require an actually present, exactly verified skill
   directory. `Verify(allowMissing=true)` succeeds for an absent old directory,
   so equal receipts alone cannot authorize skipping its reconstruction. Add
   an explicit missing-same-pin repair contract using the production helper.
2. **No-follow clarification.** The declared no-follow roots and containment
   must reject ancestor aliases even when they resolve inside the selected
   project. Include that contained-alias regression alongside external escape;
   a same-project alias must not become native ownership authority merely
   because it passes a containment check.

## Accepted contracts

- Explicit project boundary, distinct skills root, exact config paths/codec and
  declared ownership kinds; adapter retains validated scope, original user root,
  selected registry and native dialect/projection responsibility.
- No helper payload loader or descriptor, portable transport mapper, discovery,
  trust/settings engine, source acquisition or native executable dispatch.
- Caller-prepared native servers and exact staged desired receipts are checked
  before mutation. Existing Kernel handles exact entries, locks, conditional
  replacement, postconditions and rollback; native servers are never executed.
- Skills stage/backup/swap under a private same-filesystem transaction, exact
  backup verification, post-install verification, and cancellation before MCP
  commit. Foreign drift prevents deletion/overwriting, and failed restoration
  retains the only recovery copy.
- Successful config commit or CommittedCleanupError keeps installed skills.
  Unknown MCP commit state stays uncertain; observed safe failures may remain
  unchanged. Private cleanup failure becomes committed cleanup degradation.
- New Codex and Cursor consumers share this boundary without altering existing
  client implementations. Each retains separate prepared readiness and exact
  native layout, including Codex's separately reviewed narrow TOML codec.

The proposed API is accepted: ProjectComponents layout with ValidateObjects,
SkillObject, MCPObject, Verify and Inspect; ProjectComponentMutation with active
package, previous/desired ownership, prepared server map, injected Kernel and
Apply(ctx). Object IDs are kind+":"+name; the supplied MCP receipt path is
limited to the layout's exact selected JSON/JSONC candidate. No extra helper
surface is required for Codex.


## Fix verification

Re-read the amended contract's R1 clarification and frozen public signatures.
Both observations are closed: missing same-pin skills are recreated and only
observed exact present directories qualify for no-op; contained as well as
external ancestor aliases are explicitly refused and covered by contracts.

**R1 GO.** No plan blocker remains. Independent R2 and parent coding
authorization remain separate; this reviewer has not implemented the shared
helper. Codex will consume the frozen API and retain its independent codec,
projection and scope/readiness responsibilities.
