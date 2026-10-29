# Qoder anonymous metadata: exact auth-directory membership

Status: proposed correction; no code in this slice has been changed. This addendum
responds to two independent reviews performed after the original metadata recipe
was implemented. It does not backdate those reviews into preimplementation GOs.

## Frozen starting point and concrete defect

Start from candidate 39f1267d5096a0f8c2e3cd0388e047978c2c7de6, tree
9e452cff096e0ab53b3cdf235258a4a2ddc9dd30, in a separate worktree. Preserve the
existing author's complete commits and contracts. The helper currently has SHA256
be18f86af7d290b079d5b360c85b501e8451969a47005761fe8ad6455c9ea7f7;
contracts b2799da43d86c5ee961ba8eba0aec3f584155ec30914cae47e0cf6c6eb74736e;
shared fixture 557b178c29b065ba5e17a55b42ae3e1c3c77dce3402a5199e9ed4eb00110359e.

The current fixed-file checks do not enumerate .auth. setup runs native version
and public add before the first complete-profile catalog snapshot. Consequently
an unknown .auth sibling can pass recipe.check and enter that later baseline.
This is a source counterexample, not observed native account creation.

## Narrow implementation

Add a private bounded verifier for the exact direct .auth names machine_id and
.credential-transaction, and no others. Do not enumerate the whole profile or
read unknown child bodies. Open a readable directory descriptor relative to the
already-held .auth descriptor with openat(".", O_RDONLY|O_DIRECTORY|O_NOFOLLOW|
O_CLOEXEC); never reopen an unchecked filesystem path or use O_PATH for readdir.
Check held ownership, directory type, exact0700 and identity against the captured
auth directory before and after enumeration. Use a fresh descriptor each call,
read at most three entry names, and require exactly the two expected unique names
and EOF, with all read and close errors refusing. Unknown names are not printed.
Keep existing fixed-file regular-file, owner, mode, size, link, identity and digest
checks; expected names still pass those independent checks, so no special file,
alias or changed expected child gains authority.

Enforce membership from initial capture onward and around every existing recipe
check, including the final linked identity phase. Preserve held profile/auth
checks and linked Fstatat verification before/after the membership operation.
Do not pin directory access timestamps across commands; ordinary reads and
same-mode chmod retain their existing semantics. Inside a single enumeration,
verify stable directory identity and membership-relevant metadata to refuse a
concurrent mutation; any observed change is a refusal, never a new baseline.
Retain the full original-profile equality guard and all command boundaries.

No changes to production adapters, vendor pins/argv, original HOME, credential or
trust handling, file contents, deadlines, cleanup, parser allowances, qualification
flags or VS Code. No native/vendor executable will run locally. The recipe still
does not establish equivalence between the public npm source and the pinned
standalone executable, or account authentication or successful native behavior.

## Regression and validation

Add actual recipe.check controls before any complete-profile snapshot: create
.auth/user, .auth/user.production or an unrelated sibling as an ordinary file;
also exercise a directory, symlink and FIFO without opening their bodies. Each
must refuse promptly, leave the unknown object and the authored bytes intact,
and keep the fixed-file protections. Add direct initial-capture rejection with
an unknown entry and repeated positive checks, including after ordinary reads and
same-mode chmod. Preserve every prior test. No mock positive native result.

Run the new direct regression against the old helper using an inert Go overlay
to show the concrete gap (expected failing test), then the corrected focused
Qoder/shared inert Go contracts and actual affected all-enabled plus unfiltered
whole-file architecture/size lint gates. Serialize Go/lint with any active job;
do not delete another task's files. Use two independent plan GOs before coding and
two independent exact-code reviews after the correction. Record source hashes,
actual commands/results and their scope without attributing other workers' test
execution to this worker.

## Publication and concurrency

Do not edit /tmp/uap-qoder-publish or its publication manifest/checkpoint. Commit
this addendum, correction and actual validation separately in the isolated branch.
Publish only after fresh inspection of the existing PR523 head, full source review
and checks. A normal fast-forward may include the original author's39f126 ancestry;
if concurrent source differs, stop publication, preserve it and reconcile/review
the exact combined bytes. No force-push, replacement PR, merge of unqualified leaf,
release, cancellation of unrelated CI or change to the original negative captures.

All existing five qualification flags remain false until genuine current-source
hosted evidence independently establishes applicable claims.
