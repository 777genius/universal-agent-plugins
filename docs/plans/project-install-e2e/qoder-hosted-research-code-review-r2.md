# Qoder hosted pre-code research implementation independent review R2

Reviewer: cursor_clients, 2026-10-23. Frozen source:
`dfb262a80b37952a7156e37f5e53b787a90a3fd1`, work/uap-project-qoder.
Reviewed the complete runner, inert tests, dedicated workflow and evidence
description against the pre-code plan and both preceding GO reports.

## Decision: GO for bounded hosted research

No implementation blocker found. This approves publishing the research leaf
and interpreting its eventual genuine version/help/empty-list artifacts. No
Qoder mutation adapter, installation or project runtime is qualified.

- The actual unchanged imported require_hosted guard runs before output
  creation, acquisition or launch. Refusal returns a bounded diagnostic and
  creates no output parent. Workflow source is exact, checkout credentials
  are not persisted, contents permission is read-only, runner is ubuntu-24.04
  and the outer timeout is ten minutes. Existing matrix guard/helper bytes
  are unchanged in this leaf's implementation delta.
- Exact versioned HTTPS acquisition has a 90-second request timeout and a
  96MiB cap measured during reads. Vendor archive hash is checked before
  decompression. Expanded gzip bytes, tar member count, regular binary size,
  normalized names and duplicate entries are bounded; links, special files,
  traversal/absolute names, extra regular files and missing/duplicate binaries
  refuse. Only the fixed verified binary path is written mode0700, after the
  captured binary digest matches. No installer script is executed.
- Fresh profile_environment output plus literal PATH/TMPDIR/LANG is the entire
  native environment. Ambient tokens, credentials, proxy/config variables and
  workflow metadata do not reach Qoder. Every probe uses the fresh project cwd,
  DEVNULL stdin, argv execution without shell, and a new process session.
- The six exact probes match the reviewed allowlist. Each child inherits a
  hard RLIMIT_FSIZE output cap and disabled core dumps; live polling checks
  both log sizes and the 30-second deadline. Cleanup sends SIGKILL to the
  process group even when the direct child has exited. Thus the output cap is
  enforced during execution and ordinary surviving descendants are closed;
  it is not merely a post-exit truncation. Captures are read only after cleanup
  and retain byte counts, hashes, exit/signal, elapsed time and error status.
- Exact version-token and strict empty JSON-array checks fail on wrong version,
  nonzero exit, output/deadline error, malformed/unrecognized/nonempty list or
  list stderr. Main records every completed allowlisted probe, does not turn
  failed probes into empty evidence, writes a post-admission failure summary,
  and requires all six accepted results for success. Artifact upload always
  runs. Native help text remains evidence to interpret, rather than a claim
  that an ancestor's help proves a particular mutation verb.
- Evidence wording leaves populated record schema, project ownership,
  discovery, skill execution, MCP initialization, trust/account/model and
  restart unobserved. Runtime network isolation is expressly unclaimed.

## Independent verification

- Inert research contracts: **7 tests PASS** (0.923s), including actual ordinary
  Python child output limits, DEVNULL stdin and descendant timeout cleanup.
- Existing `test_native_client*.py` safety/evidence suite: **54 tests PASS**.
- The direct refusal test invokes the real imported guard in an ordinary empty
  environment and proves no requested output directory appears. It neither
  patches the guard nor forges hosted variables.

No native Qoder binary was executed by this review. Genuine hosted outputs are
still pending and must be evaluated before any later registry mutation code.
