# Prove task disappearance after a kernel release snapshot

## Observed failure and source evidence

The one provider-parity observation failed on source
`fa59fd25dcac087296a8a95ed0ee2fee5d1ec7b5`, tree
`42fed9b76c7e0a5cd4e43a752c95463546303bfd`, run `40199085334`, job
`121186190348`. The actual GitHub-hosted Ubuntu 24.04 image was
`20261012.334.1`. The unchanged bounded test stopped at its first failure,
fetch iteration 497, after 102.598 seconds. Its controlled original process
error ends with `malformed proc task stable identity`; the fixed class is
`ordinary_supervisor_descendant_cleanup`. The earlier Ubicloud 512-pass
observation remains a separate result.

This identifies the task-stat stable-identity parser's refusal. The raw stat
record was not logged, so its exact numeric fields are inferred from the
producer, not captured evidence. Do not assign this origin to the earlier
unclassified Core normalization failure or claim the earlier parent-change
hypothesis was observed.

The Linux producer admits a valid release snapshot that the parser currently
rejects:

- [Linux v6.8 `do_task_stat`](https://github.com/torvalds/linux/blob/v6.8/fs/proc/array.c)
  (blob `34a47fb0c57f2570a4f7cb1f45373ddaf2afa883`) initializes parent to zero
  and process group/session to minus one. It replaces them only after obtaining
  the task's signal-handler lock. The task state is sampled earlier, so that
  state alone cannot classify the later snapshot as released.
- [Linux v6.8 `__lock_task_sighand`](https://github.com/torvalds/linux/blob/v6.8/kernel/signal.c)
  (blob `c9c57d053ce4f64d9a832f358b4e1ee837959b8b`) can return no lock after
  the task's signal-handler pointer becomes null.
- [Linux v6.8 `__exit_signal`](https://github.com/torvalds/linux/blob/v6.8/kernel/exit.c)
  (blob `dfb963d2f862ada6a2f06259c6df4923272cb218`) unhashes the process
  before clearing that pointer. [Linux v6.17's stat producer](https://github.com/torvalds/linux/blob/v6.17/fs/proc/array.c)
  (blob `d6a0369caa931ebd6baa3fbb01612d04ddf9d3bf`) retains the same stat
  defaults and lock boundary. These are pinned producer examples, not a claim
  that the hosted job exposed either exact kernel version.

## Narrow correction

Change only `osrunner_ordinary_proc_linux.go`, a new inert regression test file
beside it, and this plan. Build on the reviewed fixed-origin diagnostic slice.
Do not change traversal, pidfds, group signaling, quiescence, process limits,
timeouts, profiles, command/environment construction or native admission.

1. Retain the existing bounded read and strict TID, state, field-count and
   numeric parsing. All ordinary accepted identities and malformed failures
   retain their existing behavior. Only after that parsing succeeds, recognize
   the literal canonical parent/group/session tuple `0`, `-1`, `-1`, with
   successfully parsed TID/start time and a complete modern stat record (at
   least the 50 fields beginning with state, through the exit-code field).
   Every numeric field in this special record must be canonical decimal:
   no leading plus, redundant zeroes or negative zero. Other negative groups,
   noncanonical tuple spellings, truncated records and malformed values remain
   ordinary errors and receive no new disappearance authority.
2. Return a private typed release-observation error carrying only the parsed
   stable identity. Its error text remains exactly
   `malformed proc task stable identity`. It is not ENOENT, ESRCH, unsupported
   containment or a successful identity. Do not wrap a gone sentinel. Admit
   this new marker only by its direct private nonnil pointer type; wrappers,
   joins and unrelated errors with the same text do not acquire its authority.
   Detect a private marker nested in a wrapper or join before entering the
   existing gone-error path. Return a private opaque uncertainty error for that
   whole input, including marker-plus-ENOENT/ESRCH combinations. The uncertainty
   carrier retains the exact original error object and delegates its text but
   intentionally exposes no `Unwrap` or `Is` method: the caller's existing
   `linuxProcessGone` must never mistake that uncertainty for disappearance.
3. At the existing identity-error boundary, allow exactly one independent
   `taskExists` observation for this marker, in addition to the unchanged
   existing gone-error path. Only a positively absent task directory permits
   the existing gone result. A still-present directory returns the original
   release error unchanged. A permission or unknown directory-observation
   failure remains failed and retains both the release cause and the new
   observation error. Join the direct release marker with an opaque uncertainty
   carrier holding the observation error; do not expose an observation errno
   through the public unwrap chain. The marker remains discoverable as the
   original parse cause, while the exact observation object remains available
   in its private carrier. This also protects against a broken observer
   returning an error that wraps or joins a gone errno. The production
   `linuxTaskDirectoryExists` already normalizes ordinary missing-directory
   errors to `(false, nil)`; document and test that contract separately.
   Existing unrelated gone-error handling and `linuxProcessGone` itself stay
   unchanged. Do not retry a stat read, sleep, or treat uncertainty as an empty
   children list.
4. If the marker follows a valid initial identity and a missing children
   interface, compare its carried PID/start time with that initial identity
   before the directory observation. Preserve the existing changed-identity
   refusal even if the replacement task subsequently disappears. The first
   identity-read path has no prior identity to compare and still needs the
   same independent absence proof.

The marker alone never proves safe completion. The surrounding supervisor
continues to discover adopted children, retain pidfd ownership, wait for its
existing three clean empty observations and fail on every unrelated unknown
or containment error. A released current locked thread still fails ordinary
preflight; it cannot authorize command launch.

## Regression and validation

Use per-call inert proc operations; run no child command locally. Demonstrate
that the old implementation refuses the exact released-record-plus-absence
case, then require the corrected implementation to pass it. Cover both first
identity and missing-children/second-identity paths, including same identity,
changed start time with an absent directory, present directory, permission and
unknown observation errors. Assert exact call counts and path/size bounds.

Also cover direct-marker admission, non-gone/error-text compatibility, wrapped
or joined marker refusal, same-text unrelated errors, wrong TID, truncated or
nonnumeric fields, noncanonical decimal spellings, other negative groups and
invalid start time. Unknown malformed input must not call `taskExists` at all.
Include marker-plus-ENOENT/ESRCH inputs and wrapped/joined gone observation
errors: assert no directory-based acceptance, exact retained cause objects and
`linuxProcessGone == false` through the real contextual wrappers/joins. Test
the real directory observer's absent/present/error normalization with inert
temporary filesystem entries. Error classification must not read arbitrary
error text, including through the opaque uncertainty carrier.
Preserve the current valid/live, missing-interface, changed-identity, genuinely
gone and prelaunch-refusal contracts. Verify that failed observations retain
their exact cause and do not become unsupported containment or disappearance.

Run the focused new and existing inert process contracts, vet, configured
affected-package lint and the unfiltered size/architecture gate. Keep the
previously reviewed diagnostic, OpenCode and native fixture source intact.
Two independent plan rounds precede coding; two independent frozen code
reviews precede publication for hosted execution.

After publication, advance the focused branch once from its observed failing
head `fa59fd25dcac087296a8a95ed0ee2fee5d1ec7b5` to the reviewed corrected
source using an expected-head lease. Run the same immutable
`f579c6b68c5d625e9e87e4e51b288990ddfcd1a4` test once on the unchanged
`ubuntu-24.04` workflow: 512 serial fetches, first-error stop, unchanged
environment, containment and all deadlines. Record the actual new source,
tree, provider/image, named result and no-skip status. This is validation of
a substantive parser correction, not a retry of the failing source. Preserve
both prior observations and do not claim that one finite pass proves absence
of every process race. The final integrated candidate still requires all
normal Core/Required/Coverage and native-client gates.
