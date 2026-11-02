# Codex TOML codec independent implementation review R1

Reviewer: client_plans, 2026-10-23.
Reviewed committed codec-only diff `fda36d23755318b5cb95c858e1f38430924294ad`
against its parent in the Codex child worktree. Uncommitted adapter changes were
excluded from review. This is not full Codex adapter or native runtime approval.

## Decision: GO for the narrow codec checkpoint

No correctness blocker found in the committed diff. The new codec keeps the
existing ownership/locking/conditional-write kernel and supplies a TOML parser,
retained syntax tree and checked render route. Existing JSON/JSONC codecs keep
their parsers and collection names; Codex-only auth options are rejected by
other codecs instead of silently ignored.

## Reviewed boundaries

- TOML is parsed semantically before editing, so duplicate keys/table conflicts
  and malformed syntax are refused without mutation. The MCP collection must
  be a table. Only MCP values enter the neutral JSON mutation document; unrelated
  TOML date/time and other native types remain in their original AST.
- The native edit scanner gathers matching complete key-prefix entries before
  removal. It removes the owned server's nested table mappings as well as its
  heading, while sibling prefixes remain separate. Inline, dotted, parent-table
  and child-table collection forms have concrete lifecycle coverage.
- Each render re-parses the resulting TOML, compares all non-MCP semantic
  settings, and compares every desired/unchanged MCP member against the
  requested neutral document before committing. An unchanged semantic
  selection returns the original bytes rather than reformatting the document.
- MCP whole-entry digest ownership includes all native entry fields, including
  foreign `enabled=false`. An unreceipted existing member is a collision;
  adding a foreign field makes update/remove fail exact ownership. The generic
  kernel's desired-receipt checks, batch paths/codec checks, no-follow reads,
  writer locks, conditional replacement and verification still apply.
- Native projection maps headers to `http_headers`, retains explicit nonsecret
  Codex authentication references, and rejects unsupported SSE rather than
  degrading its transport. Stdio values remain command/args/env/cwd. Auth and
  OpenCode-specific options cannot silently cross codec boundaries.
- The new TOML editing dependency is limited to native syntax preservation;
  this does not add another installation or settings engine.

## Independent verification

Ran the complete `adapters/nativeconfig` package on the committed codec scope,
including all new `TestProjectCodexTOML*` fixtures and the existing ownership,
CAS, locking, malformed/duplicate parsing, rollback and other-codec regressions.
Result: PASS, `nativeconfig` 3.680s (Go 1.26.10). No
native Codex process, model call, MCP execution, login or trust action was run.

Full adapter scope qualification, project/user/A/B isolation and conservative
readiness remain subject to the separate Codex implementation reviews.
