# PR #878 — Cline selected-process implementation

This is the first production slice of the six-client plan, not completion of
all six. VS Code and ChatGPT Web remain excluded. No release is authorized.

`agentplugins run --target cline --scope project --dry-run` reviews the entire
owned Project MCP source. Explicit `--confirm-client-start` starts one Cline
process with that source; `--inspect` selects Cline's read-only inventory instead
of its interactive UI. No arbitrary child arguments, global shim, profile copy,
parent-shell mutation, state schema, automatic workspace trust or activation
attestation is introduced. Ordinary `cline` still uses its original User source.

A single optional client capability supplies and verifies the process plan.
Only Cline implements it. Existing package/native ownership verification and
mutation locking are reused. Unknown extra MCP members, disabled or modified
entries, missing packages, pending operations and changed plans are refused.
Native inventory must identify the exact selected file and every owned server;
a successful process exit alone is insufficient. Support is based on this
observed contract, not an exact-version allowlist.

The existing inherited-stdio Unix replacement / Windows Job Object boundary
is reused, with explicit child environment. Old managed-stdio callers retain
their existing environment behavior. The installer lock is not held throughout
an interactive session. Files are checked again immediately before startup;
this is ordinary cooperating-user process execution, not a sandbox against
concurrent arbitrary same-user code or the native client's own installed plugins.

The native workflow pins Cline 3.0.73 for reproducibility. It exercises actual
UAP add, repeat, update, missing-package repair and remove, native selected-file
inventory through the new public command, direct installed-config MCP calls,
retained user data, consent negatives and independent User/Project B authority.
The separate MCP caller is not represented as a model-initiated Cline tool call.
Linux native qualification does not by itself qualify the other native hosts.

Remaining work in the same PR: Grok's safe enable route, Antigravity registration,
Devin/Kiro qualification, Hermes Project MCP, full final-head review/checks and
postmerge acceptance. The temporary offline-worktree exporter is removed once
its verified source/compiler inputs are available; it is not a product workflow.
