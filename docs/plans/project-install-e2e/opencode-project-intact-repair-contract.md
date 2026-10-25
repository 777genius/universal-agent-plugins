# OpenCode project: report actual intact group repair mutation

At exact source `e81e9daf2fbce3acf36aee33eb954a68fcf18acc`, tree
`587193cb6cf17ea277630ecd8a03ef9f5fc7753e`, native run `40194870389`
passes repeat add and unchanged update but fails the newly reached intact
repair assertion on both platforms: `mutated = true, want false`.

The macOS artifact `12600345253` is 647,797 bytes, SHA-256
`3a948f788587492286ffb853302fe6a26959feed51d84bb6bd61fa05883267f0`;
Windows artifact `12599574306` is 635,744 bytes, SHA-256
`024a89ef76c5f9ac9022628b1aad217484fcdf43455562a3362c639a0ac46d4b`.
Their closed runner digests/source identity were verified. Both raw repair
outputs are real completed single-target batches, Prepared and installation
verified, with `mutated:true` and no `no_change`. Recorder counts remain zero.
The remaining four required native tests pass without skips. Later project
stages were not reached and remain unqualified; the failed envelope does not
contain the working authority's full state or journals.

## Existing contract and exact correction

The public group repair path calls `RepairGroup`. In `group_plan.go`, repair is
explicitly excluded from the unchanged-package shortcut, so it restages and
commits the managed package. The existing
`TestProjectPreparedGroupPublicLifecycleAndHandoff` explicitly requires actual
mutation and changed state for intact group repair while native bytes remain
equal. The separate single-target usecase repair can be unchanged; that is not
the public batch path under test. Preserve production behavior and both tests.

Change only this native project's assertion flow and narrow pure contracts:

- Keep repeat add and unchanged update in the current strict no-mutation loop,
  including exact state/config/skill bytes and the existing assertion reader.
- Run the same already-budgeted intact repair separately. Require its real
  completed batch, actual boolean `mutated:true`, and absent or boolean-false
  `no_change`; refuse malformed or contradictory values without defaults.
- Require state bytes actually to change. Snapshot the exact managed package
  directory, native config and native skills before/after and require identical
  byte/mode inventories. Keep the following fresh `stable-A` config/skill pair.
- Compare the complete before/after binding, excluding only `UpdatedAt` and the
  appended transaction receipt history. Require the old receipt prefix exactly
  preserved and exactly one new committed `directory_swap` receipt with matching
  binding/path and identical nonempty before/after package digests. All other
  binding fields, including identity, selected delivery, package revision,
  native ownership, profile authority, lifecycle, authentication, policy and
  pending-attempt fields remain exact; explicitly refuse unresolved attempts.
- Keep bounded before/after state/native snapshots and the binding comparison
  facts in the existing summary, without a new ZIP member or collector change.
  Describe the stage as unchanged replay plus mutating, byte-preserving repair.

No command, timeout, cap, environment, native parser, original user test,
production lifecycle or later stage changes. Expected implementation is one
small native helper and one pure receipt/binding assertion with focused positive
and refusal contracts, not another lifecycle harness. Root and a second peer
review two frozen plan rounds and the final code. Run the pure contracts, compile,
existing project group lifecycle contract and configured affected-source lint.
No local vendor execution; exact final-head hosted replay is still required.
