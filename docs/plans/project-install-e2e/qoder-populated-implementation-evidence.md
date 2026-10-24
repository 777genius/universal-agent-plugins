# Qoder populated collector: current implementation and review receipts

This is research infrastructure only. It implements the previously twice-reviewed
[qoder populated recovery plan](qoder-populated-research-recovery-plan.md).
No production Qoder adapter, native ownership parser, native activation, loaded
skill, trust, model, authentication, session or MCP qualification follows from
collector implementation or inert tests.

## Frozen code and publication

Final independently reviewed local code:
`c3c4b8e680c2d56f849356d32c04a28d0e22a0c0`.
Published equivalent:
`e29237fe546d8aebde98adaae59d346142143d2d`, tree
`15dcc674b81ab5bbd73e7d46de36f99c8e605fe3`.
The publisher compared exact trees before synchronizing the isolated worktree.
The final branch update published the reviewed chain atomically.

Initial code review refused publication for three concrete issues. `/sys/class/net`
was not evidence of the child's actual network namespace; it was replaced by
`socket.if_nameindex()`. Six file pins did not detect new source hooks/files;
complete bounded descriptor-relative source snapshots now freeze exact names,
types, modes, sizes and hashes before the first native call and are compared
before and after every probe. `TemporaryDirectory` could start recursive cleanup
after its one-shot deadline had expired; it was replaced by an explicit inert
cleanup child with a separate five-second timeout and held-root identity.

The upload boundary was also strengthened before publication. The workflow never
uploads raw output. A separate private pending tree receives only complete,
stable, ordinary unaliased files: at most 64 files and 64 MiB in aggregate,
with a five-second copy deadline. Only a complete admitted tree is renamed to
the actual upload directory. Rejected, oversized, aliased or nonregular raw
output produces a small failure summary in that directory instead. Incomplete
experiment cleanup forces failure without replacing the original probe error.
There is no recursive temporary-directory finalizer.

Actual `/usr/bin/git rev-parse HEAD` is bounded to 128 stream bytes and five
seconds and compared with the exact expected 40-character commit before
acquisition. The parent records this observation, preserving expected and
measured source identities. The child checks actual namespace, interfaces,
original nonroot real/effective UID/GID, empty supplementary groups, zero
capabilities and no-new-privileges before any Qoder command. Acquisition is the
only networked phase, and no hosted flags, tokens or proxy authority reach
Qoder's explicit fresh profile environment.

## Independent implementation decisions

Both independent reviewers read the final frozen code and approved only this
finite research collector before publication:

- **R1, root: GO.** Complete fixture-tree pins; actual interface enumeration;
  strict JSON/error fail-stop across the finite 19 native commands; measured
  exact checkout; original/effective identities; independent five-second
  held-root cleanup; bounded stable staging into a separate upload directory.
  Independently ran all 12 inert collector contracts, PASS (0.273 s).
- **R2, skillshare_scope: GO.** Independently rechecked the same corrected
  boundaries and original-failure retention. Independently ran all 12 inert
  collector contracts, PASS (0.280 s).

Author verification: 12 populated collector contracts PASS (0.328 s), six shared
live-capture contracts PASS (2.235 s), seven existing phase1 Qoder research
contracts PASS (3.714 s), Python compilation and `git diff --check` clean.
The genuine unchanged hosted guard was exercised on the task host in a clean
subprocess environment: refusal before output creation. No vendor acquisition
or Qoder process ran locally, and hosted-admission variables were not forged.

## Genuine observation gate

The first genuine populated run is
[40130660701](https://github.com/777genius/universal-agent-plugins/actions/runs/40130660701),
at published source `e29237fe546d8aebde98adaae59d346142143d2d`.
At this documentation checkpoint it is queued; this is not a successful run
or native observation. Pending deployments are empty. The repository has an
active runner backlog; no hosted guard, platform or approval was weakened.

Raw actual observations must be downloaded, verified and independently
interpreted before proposing a production project adapter. Unknown output or
inventory refusal remains a failed observation. Any concrete source-contract
proposal then needs two independent plan review rounds before production code.
