# Qoder pinned hosted research before project adapter code

Owner request, 2026-10-23: obtain genuine pinned version/help/list evidence on
an approved disposable GitHub-hosted runner before implementing the Qoder
project registry mutation adapter. The original two independently approved
client plans remain in force. This narrow plan describes research plumbing,
not a replacement adapter design or a native project qualification.

## Reason and evidence boundary

The official release manifest pins Qoder CLI **1.1.78**, Linux x64 archive:
`https://qoder-ide.oss-accelerate.aliyuncs.com/qodercli/releases/1.1.78/qodercli-linux-x64.tar.gz`
with vendor SHA256
`5e3213d2d4256c86c5ad2c8bf73fd392f92ba80d0d38f822806542805a48a8ea`.
Manual bounded acquisition yielded one regular `./qodercli` binary, 182,982,112
bytes, SHA256
`0da0976e13fe00bc28f725d6fe7c2f0fe9fa9a8c37eeb2ee4e80ad4c0fc58953`.
The task-host read-only probes encountered private-proc mount refusal, then
Bun1.4.2 abort134 in the more restrictive networkless namespace with empty
/proc. No successful task-host help is claimed and host /proc was never used.
Captured provenance/crash logs remain separate evidence.

Official references:
- https://docs.qoder.com/cli/plugins
- https://docs.qoder.com/cli/plugins-reference

The reference states `plugins list --json` includes all recorded scopes;
project/local rows have projectPath, while enabled reflects the command cwd.
The empty hosted list can confirm only the top-level empty-list shape. It
cannot establish a nonempty record schema, native project discovery,
installation, ownership, MCP initialization, skill execution or restart.
Project mutation implementation remains gated on sufficient genuine schema
evidence; neither static binary strings nor an empty list will be relabeled.

## Planned files and approved actions

Add `scripts/research-qoder-native.py` and a separate research-only workflow
`.github/workflows/qoder-native-research.yml` after this plan receives two
independent GO reviews. Keep the existing native matrix runner/guards unchanged.
The script dynamically imports the adjacent existing
`run-native-client-matrix.py` and calls its unchanged
`require_hosted('linux-amd64')` **before** network access, artifact/probe directory
creation or native execution. The workflow uses only GitHub-hosted ubuntu24.04,
a fixed source checkout, contents:read, persist-credentials:false and explicit
AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED=1. Trigger it for this leaf's pull-request
changes or explicit workflow_dispatch. It has a short timeout and uploads
research outputs with always(), even when a probe fails. No repository secrets
or workflow tokens are passed to the native process.

Acquire only the exact HTTPS archive above with a 90s request timeout, a
96MiB compressed byte cap checked against actual reads, and the exact vendor
hash before parsing. Inspect the tar without extract/extractall: at most16
members, only directories plus exactly one regular basename qodercli, no links,
special files, traversal/absolute member names or another regular file. Bound
the binary to256MiB before reading, read exactly its declared bytes, require
its captured exact binary SHA256, then write one fixed binary path mode0700.
Never invoke either vendor installation script or `qoder install`.

Create a fresh temporary parent with separate binary, home, project, tmp and
output directories. Use the existing profile_environment helper for fresh
HOME/XDG/npm/Git config files, plus a literal allowlist for PATH (/usr/bin:/bin),
TMPDIR and LANG. No ambient environment, user settings, credentials, proxies,
GH_TOKEN, GITHUB_TOKEN, model keys or permission flags reach the binary.
Set cwd to the fresh project for every command. Run exactly:

1. `qoder --version`
2. `qoder plugins --help`
3. `qoder plugins list --help`
4. `qoder plugins install --help`
5. `qoder plugins uninstall --help`
6. `qoder plugins list --json`

Each process has a30s deadline, bounded stdout/stderr capture (1MiB each) using
files rather than unbounded PIPE accumulation, stdin explicitly DEVNULL,
no shell, and process-group
termination on timeout/output-limit breach. Keep an independent total workflow
timeout. Record arguments, elapsed time, exit/signal, stdout/stderr byte counts,
SHA256 and separate log files. Run the allowlisted probes even if one fails;
the final script exit is nonzero on any failure, unexpected version or
nonempty/unrecognized empty-list output. The version must contain exact1.1.78
with version-token boundaries. Parse the final list as JSON; require exactly
an empty array. Do not treat error, truncation, unexpected shape or skipped
probe as an empty result. After hosted admission, preserve a machine-readable summary even on failure.
A hosted-guard rejection instead prints only a bounded diagnostic and exits
nonzero without creating any output/probe directory, acquiring bytes or
launching a native process; summary creation must never precede admission.

This research does not install/uninstall plugins, create hooks, edit settings,
run trust/auth/login commands, contact a model, initialize MCP or execute a
plugin payload. The read-only list may create native scratch state only inside
the disposable empty profile/project. Runtime network isolation is not claimed;
the native environment is credential-free and no connected account is used.

## Verification before publication

Use Python syntax/static checks and meaningful unit tests for bounded archive
admission, pins, version/JSON validation, subprocess output/deadline handling,
explicit environment and guard ordering. Tests must use injected inert
functions/processes and must not spoof hosted environment variables, monkeypatch
or weaken require_hosted, or execute Qoder on the task host. A direct invocation
without hosted opt-in must fail through the real imported guard before outputs
or acquisition. Existing matrix safety tests remain unchanged and pass.

Commit plan plus both independent review/fix GO reports before adding runner
code; then complete bounded implementation checks, commit source and send SHA
to root. Root publishes the early Qoder leaf/PR to run hosted evidence. Review
and interpret genuine uploaded evidence before implementing registry mutation.
The research artifact is explicitly named as pre-code research and cannot be
substituted for the separate project E2E native proof gate.
