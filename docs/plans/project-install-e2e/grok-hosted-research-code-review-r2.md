# Grok hosted research implementation independent review R2

Reviewer: cursor_clients, 2026-10-23. Frozen source
`3107bced573c4f665d2ce7c440cfb176e4b0a0cf` in work/uap-project-grok.
Reviewed complete research runner, shared utility move, inert tests, both
workflow changes and evidence against final pre-code plan `9314feef` and both
plan GO records. No production Grok adapter changes are included.

## Decision: GO for bounded hosted research

No implementation blocker found. This approves publishing the research leaf
for genuine read-only populated-source observations. Actual source/version/
marker attribution and precedence interpretation remain a separate review;
the code always keeps project_source_qualified:false.

- The actual unchanged imported require_hosted guard precedes output creation,
  acquisition and launch. Real refusal is nonzero, bounded, and creates no
  requested output parent. Hosted workflow is ubuntu-24.04, contents:read,
  exact source checkout without persisted credentials, explicit opt-in,
  ten-minute outer timeout and always-upload evidence. No guard is spoofed.
- Exact official versioned archive bytes are bounded during reads and checked
  against the measured pin before gzip parsing. Expanded bytes and exact binary
  hash are checked before one fixed mode0700 executable is written. Provenance
  honestly states these are measured official HTTPS bytes, not an independent
  vendor-published digest. No installation script is executed.
- The fresh profile helper plus literal PATH/TMPDIR/LANG is the complete native
  environment. Two independent empty local Git repositories prevent discovery
  from crossing their project roots; fixed /usr/bin/git init has an empty
  template, DEVNULL stdin, the same environment and bounded process helper.
  No network/source/remote/auth/hook Git operation is requested.
- Only plain manifest and unique skill text candidates are created under the
  exact existing projector's user/A/B directory layout. They declare no hooks,
  commands, agents, MCP servers, executables, trust or account settings. Existing
  fixture names and aliased parents refuse. Native argv is confined to version,
  help, plugin list --json and inspect --json at the selected fresh cwd.
- Nineteen expected observations are ordered and explicitly accounted for.
  Nonzero exit, deadline, overflow, malformed/duplicate/nonstandard JSON,
  stderr on JSON observations, wrong version or missing probe cannot become
  complete collection. Every completed record retains arguments, cwd, byte
  counts, hashes, elapsed time, exit/signal and errors. Budget exhaustion yields
  incomplete failed collection rather than skipped-success evidence.
- The independent eight-minute main-thread interval timer covers acquisition,
  fixture preparation, observations and inventories, with monotonic loop/read
  checks and per-process min(30s, remaining budget). Process cleanup has a
  bounded wait; the budget context restores timer/signal state before the
  post-admission summary is written, leaving margin before the outer job limit.
- Inventory is descriptor-relative and no-follow. Entry/depth/name/path limits,
  regular-file size and aggregate actual read limits are enforced while walking
  and reading. Regular opens recheck type, identity/link count and before/after
  metadata; multiply-linked files refuse. Directory descriptors are validated
  before/after recursion. Symlink targets and special-file metadata are bounded
  and recorded without following links or opening special files. Baseline
  fixture/profile text must remain unchanged; partial/volatile inventory fails.
- Qoder's bounded_read, child_limits and run_probe bodies are AST-identical to
  the previously reviewed functions. Qoder pins, exact argv, real guard order,
  environment and output acceptance remain intact; its workflow only adds the
  shared helper to path routing. The original matrix runner is unchanged.
  Child output caps remain hard RLIMIT_FSIZE plus live checks, with process-group
  cleanup even when the direct child exits. No post-exit truncation substitutes
  for the live output boundary.

## Independent bounded verification

- Grok inert contracts: **8 tests PASS** (0.103s), including actual timer
  interruption/restoration, no-follow symlink/FIFO and hard-link handling,
  entry/depth/file/aggregate/metadata caps, fixed observation accounting and
  real guard refusal before effects.
- Existing Qoder inert research contracts: **7 tests PASS**.
- Existing native-client safety/evidence suites: **54 tests PASS**.
- Independently compared moved helper ASTs and verified no matrix-runner delta.

No native Grok or Qoder binary was executed by this review. Runtime egress
isolation, real project installation/ownership, disabled/ancestor/compatibility
conflicts, skill execution, MCP tool results, trust/account/model and native
restart remain unobserved. A green collection workflow cannot qualify them.
