# Isolated per-agent installation, retry and install log

Status: plan plus implementation in this PR (Draft).

## Question

Can plugin and Skill installation run in parallel across agents, and how should
one agent's failure be isolated, shown and retried?

## Evidence (checked on main c7303478e)

| Finding | Where |
|---|---|
| Skills, saved plugins, `install`, `repair`, `update --all` share one serial, fail-fast loop. The first failing binding stops every later agent. | `environment/coordinator.go` `ApplySync` |
| Three call sites map the loop's results back to bindings by a positional cursor. A prepare error yields no result, so the next never-attempted row is mislabelled `failed`. | `skills_batch.go`, `management_result.go`, `update_all_saved.go` |
| Local/`--no-save` plugin installs use `AddGroup`: atomic managed commit, then per-client activation that already continues past ordinary failures. | `usecase/group_activate.go` |
| A safe concurrent activation seam exists (`IndependentClientActivator`: serial prepare/publish, concurrent client effect). It is all-or-nothing per group and not wired in the CLI composition. | `usecase/group_activate_independent.go`, `providers/activator.go` |
| Hard limits on binding-level parallelism: one global `mutation.lock`, one read-modify-write `state-v2.json`, installation identity created by the first binding of an alias. | `usecase/service_mutation.go`, `statev2/store.go` |
| No install log, no `--verbose`. Technical detail is lost or printed raw. | repo-wide |
| Skills apply is cheap per agent. A real 4-agent `skills add` spent most of its ~45 s in shared source acquisition (`git cat-file -s` per blob); that is PR #932's latency work, not apply. | local spike, isolated HOME |

## Decision

1. Do not parallelize state-writing bindings. The gain is small for Skills and
   the risk (global lock, shared state file, installation identity) is large.
2. Isolate failures at the single shared loop so every flow benefits at once.
3. One shared presentation + retry + log layer for Skills and plugins.
4. Parallelism only at the already designed seam: client effects after serial
   preparation (plugin groups). Per-target eligibility, not all-or-nothing.

## Options (reliability / confidence, each /10)

| Option | Reliability | Confidence |
|---|---:|---:|
| A. Isolate failures in `ApplySync`, typed per-binding outcomes, shared report/retry/log (chosen) | 9 | 9 |
| B. Parallel binding apply with per-binding state locking | 4 | 5 |
| C. Fix only presentation, keep fail-fast | 7 | 9 |

## Contract

- `environment.Coordinator.ApplySync` returns one `BindingOutcome` per planned
  binding: `applied`, `unchanged`, `failed`, `not_attempted`, with its own error.
- Binding-scoped failures (client unavailable, foreign destination, client
  command failed, incomplete/conflict outcome) continue with the next binding.
- Stop everything on: context cancellation, recovery required, plan or source
  change, corrupt/migrating state, and any error marked `environment.Stop`
  (input guards that protect reviewed authority). Each Prepare still re-inspects
  state and refuses while recovery is required, so continuing is fail-safe.
- The returned error is a `*SyncError` that unwraps to every cause, so existing
  `errors.Is` classification keeps working.

## Retry

Retry is a new plan restricted to the failed agents, never a replay of old
actions (a committed receipt turns an install into repair/update). A retry whose
actions are a subset of the already reviewed ones proceeds without a second
confirmation; anything else is shown and confirmed.

## Install log

Every apply that has a failed or not attempted binding writes a JSON Lines file
under `<data root>/logs/` with 0600 permissions, bounded retention, full error
chains and context (operation id, agents, versions). The terminal shows a plain
cause and the log path; the log is meant to be handed to another agent.

## Overlap with open PRs

#932 and #936 rewrite `skills_batch.go`, `add_multi.go`, `deps_*.go` and the
progress board. This change keeps new logic in new files and touches those files
only at call sites; expect a mechanical rebase on whichever lands second.

## Delivered in this PR

- `ApplySync` per-binding outcomes and isolation; the three positional cursors are gone.
- One failure classification (`failureKindOf`) shared by review refusals and apply results.
- Per-agent report, `<data root>/logs/install-*.jsonl` (0600, newest 20 kept, credentials removed), and an interactive
  retry step for Skills and plugin add. Non-interactive runs print the `--target` command instead.
- Concurrent client activation for the plugin group path: independent clients run together with the serial
  remainder; the CLI opts in with `providers.Activator.Concurrent`.

## Follow-up delivered after the first PR

- Retry for a single-target `add` (plain yes/no) and for `install` (converges every outstanding binding).
- A retried `add` installs exactly the package of its first attempt (tree digest pinned) or asks for a new review.
- The SDK `Config.IndependentActivations` keeps working with the new `providers.Activator.Concurrent` gate.

## Not done, on purpose

- Binding-level parallel apply for Skills and saved installs (shared `state-v2.json`, `mutation.lock`).
- A retry step for `repair`, `update` and `remove`; they converge when run again.
- Native verification latency and source scan latency belong to #932.
