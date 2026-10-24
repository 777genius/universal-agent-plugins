# Project process death and fresh SDK reopen

Status: current-source fixture evidence; not vendor runtime or release proof.
Source base: `75cece1488cdab0674b4817ea7a30ea95f7aad06` plus the two process-death
test files introduced with this record. Platform: Linux/amd64, Go 1.26.10.

## Independent boundary review

The root integrator (`/root`) and independent APM reviewer
(`/root/apm_vercel_scope`) each reviewed and approved both concrete boundaries
before test implementation. Both traced the production ordering and required
parent-side checks before the actual kill, bounded child handling with guaranteed
kill/reap cleanup, ordinary adapters for setup and fresh reopen, retained native
uncertainty, unchanged ownership receipts, and user/project B isolation. No new
production injection option or synthesized journal/state was authorized.

The APM review identified that a managed-package-only interruption would not
cover the completion plan's literal native skill swap/MCP boundary. The second
case was therefore separately traced and independently approved before coding.

## Tested boundaries

| Label | Production boundary | Independently verified before parent kill |
| --- | --- | --- |
| `managed_package_committed_before_native_activation` | Existing public `Config.OnCommittedBinding` after the managed directory transaction and durable native-attempt marker, before the client adapter's activation | New managed package skill; old native project skill and MCP config; unchanged last native ownership; nonempty durable native attempt; stable selected project, binding and data receipt |
| `native_skill_swapped_before_mcp_lock_and_commit` | Real Codex activation and `ProjectComponentMutation.commit`, after native skill swap/verification and before `nativeconfig.Kernel.ApplyBatch` acquires the MCP writer lock | New managed package and native project skill; old MCP config; unchanged last native ownership; durable native attempt; exactly one real recovery directory retaining the old skill backup; stable project, binding and data receipt |

The second child alone registers a test adapter embedding the real Codex adapter.
It delegates the actual scope, ownership, projection and mutation behavior, and
supplies the existing exported `nativeconfig.NewWithLockAcquirer` primitive.
The barrier accepts only the Codex codec and project A's exact
`.codex/config.toml` path. `WithCodexDocumentParser` preserves that acquirer.
This is disclosed primitive instrumentation, not an ordinary vendor session.
All initial installations and reopened engines use the ordinary Codex adapter.

The ordering is established by the existing production path:

- `usecase/apply_commit.go` commits the managed package directory.
- `usecase/native_attempt.go` persists the native attempt before dispatching the
  activator; `installer/seams.go` then invokes the public committed-binding hook.
- Codex project activation delegates to `ProjectComponentMutation`, whose commit
  swaps and verifies native skills before calling the MCP kernel batch.
- `nativeconfig/kernel.go` validates the batch/parser, then calls its lock
  acquirer before resolving, rendering or writing the MCP document.

## Actual process death and recovery behavior

The parent installs the same inert package through the public SDK in a disposable
original Codex user profile and projects A/B, with real environment receipts and
owned data markers. It advances the source skill and MCP arguments, then starts
a separate SDK child process. The child announces the selected barrier only
after reading and checking the genuine persisted state and filesystem. The
parent independently checks those facts again and calls `Process.Kill` and
`Wait`; the child does not return an injected error or call `os.Exit`.

A completely new project A engine then performs read-only `Inspect`. The ordinary
project Codex delivery has no typed selected native-intent reconciler, so the
persisted attempt remains unresolved. The public inspection reports
`Recovery.Required` with `native attempt has no persisted intent; retain uncertainty`.
This is an honest refusal to infer ownership or acknowledgment from matching
files. No state or journal is rewritten by the test to manufacture recovery.

Valid update, repair, removal and a distinct fresh package installation remain
fenced. Where preparation yields a handle, confirmed application must return
`ErrRecoveryRequired` without mutation. Removal may refuse earlier through its
specific retained-native-attempt validation. Owner-scoped `RecoverEnvironment`
also refuses and retains the attempt; a project B engine rejects project A's
observation. Repeated inspection returns the same pending facts and launches no
native process or host callback.

The test compares complete project A filesystem facts (entries, modes and file
hashes) after death with the facts after inspection, retry and recovery refusal.
It also compares the complete original user profile, user UAP state/data tree and
project B tree. New managed bytes, the native swap's old backup where applicable,
old MCP config, foreign skill/config files, ownership state and persistent data
all remain intact.

## Verification

The focused command is:

```sh
go test ./install/integrationctl/agentplugins/installer \
  -run 'TestProjectProcessDeath' -count=1 -v
```

Both parent table cases passed in the recorded Linux run (focused package
17.363 s; parent table 17.32 s). A final rerun of the complete test source,
including the reused Windows hold, also passed (48.299 s under shared build load). The child-only helper
skips when invoked outside its explicit parent-controlled subprocess arguments.
The parent bounds the barrier and child lifetime, limits protocol output, caps
stderr at 64 KiB with synchronized diagnostic access, and kills/reaps a running
child on every parent failure path. The test reuses the existing Windows
managedstdio execute-bit hold; this Linux result does not remove that platform
limitation or establish a Windows process-death pass.

This evidence establishes actual UAP process death, partial filesystem effects,
fresh composition, mutation fencing and retained recovery uncertainty. The
native-skill case covers the completion plan's skill-swap-before-MCP-commit
interruption under the disclosed existing primitive instrumentation. It does
not establish successful automatic reconciliation, vendor discovery, trust,
authentication, MCP initialization, tool execution or release qualification.
