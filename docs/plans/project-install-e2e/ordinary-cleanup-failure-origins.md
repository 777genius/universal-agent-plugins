# Ordinary supervisor cleanup: fixed origin diagnostics

## Observed boundary

At source `e81e9daf2fbce3acf36aee33eb954a68fcf18acc`, terminal Linux
job `121173439610` fails
`TestReconciliationCLIValidatesPinnedDeclarations/missing_edge` with the
existing fixed record `phase=fetch`,
`class=ordinary_supervisor_descendant_cleanup`, `exit_status=unavailable`.
This proves a supervisor cleanup failure for that acquisition. It does not
identify the precise cleanup origin or prove that every other historical
acquisition refusal shares the same cause. Core job `121173439891` separately
fails `TestNormalizationPreservesRawTreeAndStableManifestAcrossMove` at
`skillsource_test.go:138`; that fixture has no runner diagnostic. Coverage
passes this source. Do not add another per-fixture logger or rerun passed
focused selections.

## Concrete files and immutable semantics

Production changes are restricted to a new Linux origin helper,
`osrunner_ordinary_failure.go`, `osrunner_tree_linux.go`, and
`osrunner_ordinary_proc_linux.go` in `install/integrationctl/adapters/process`.
Add inert contracts beside those files. No fixture logger, workflow, command,
environment, transport, credential, timing, cleanup or native gate changes.

Wrap only the already-created error at an existing failure return/join. A
private Linux carrier delegates `Error()` to the exact original cause and
`Unwrap()` returns that cause itself. Nil and unknown reason inputs are no-ops.
An already-owned recognized origin survives broader wrapping unchanged; joined
causes retain all original errors, with the first recognized origin used only
as a bounded diagnostic. Origin lookup never reads error text, arguments,
streams, paths, arbitrary type names or environment. Existing gone/unsupported
decisions retain their exact outcomes; no disappearance becomes an error and
no uncertainty becomes success.

## Frozen origin map

| Fixed reason | Existing function and failure return/join |
|---|---|
| `exit_observation` | `superviseLinuxOrdinaryChildren`: failed leader `Waitid` observation |
| `task_directory_read` | `linuxDirectChildIdentities`: failed bounded task-directory read, preserving gone handling |
| `task_identity_unavailable` | `linuxTaskGoneAfterIdentityError`: identity missing while the task directory independently remains present |
| `task_children_observation` | `linuxDirectChildIdentities`: invalid task ID or failed task-children observation, preserving a more specific existing tag |
| `child_identity_read` | `linuxDirectChildIdentities`: observed child's identity read fails after the existing gone acceptance |
| `child_identity_changed` | `linuxDirectChildIdentities`: the same child ID appears with another start time during task inspection |
| `pidfd_open` | `linuxDescendantIdentitiesWith`: stable handle acquisition fails after the existing gone/reused acceptance |
| `pidfd_identity_read` | `linuxDescendantIdentitiesWith`: identity revalidation after handle acquisition fails after the existing gone acceptance |
| `pidfd_identity_changed` | `linuxDescendantIdentitiesWith`: handle-bound identity start time differs |
| `parent_identity_read` | `linuxDescendantIdentitiesWith`: changed-parent revalidation has a non-gone identity-read error |
| `parent_changed` | `linuxDescendantIdentitiesWith`: child attribution differs while the exact parent remains proved live/owned |
| `tracked_identity_read` | `liveLinuxTrackedProcess`: non-gone identity-read error after the existing stable-handle check |
| `tracked_identity_changed` | `liveLinuxTrackedProcess`: observed identity start time differs from the handle-bound identity |
| `traversal_deadline` | `linuxDescendantIdentitiesWith` and `linuxDirectChildIdentities`: existing traversal deadline returns |
| `traversal_limit` | the same two functions: existing process-visit/child-count bound returns |
| `descendant_signal` | `superviseLinuxOrdinaryChildren` and `killLinuxSupervisorDescendants`: existing non-ESRCH group/pidfd signal failures |
| `cleanup_deadline` | `superviseLinuxOrdinaryChildren`: existing forced-cleanup deadline return |

Where a bounded task-directory error is currently passed to the caller's
existing gone decision, keep a gone/unsupported error object unchanged rather
than assigning an origin; only annotate its non-gone failure. This metadata
decision changes neither the returned error cause nor the caller's control.
Broader contextual errors and existing joins retain specific leaf origins.

## Private wire and existing classifier

Add optional `FailureReason` to the private supervisor result. The existing
cleanup failure producer keeps its exact error string and stage and copies
only the first recognized fixed origin into this field. A nil cleanup error
does nothing. Introduce no additional error formatting or observation.

The receiver admits a reason only after the existing strict one-record/EOF/
close admission, with a nonempty Error and `descendant_cleanup` stage. A
nonempty reason with an empty Error or another stage is contradictory: it
fails strict admission and establishes neither a stage nor a reason class.
Malformed, trailing and unknown-field records likewise remain unclassified
failures. Unknown reason plus valid cleanup/error remains the existing generic
cleanup failure. Missing reason retains legacy behavior.

Keep the existing portable `OrdinarySupervisorFailureClass` signature. Only a
recognized, strictly admitted cleanup reason adds its fixed suffix to
`ordinary_supervisor_descendant_cleanup`. Other stages and unrelated errors
retain their existing classes. This lets current bounded fixture loggers expose
the origin without changing their precedence, caps, delegation or messages.
No arbitrary wire string becomes diagnostic output.

## Contracts and qualification

Use existing inert task/traversal seams to verify applicable origin returns,
and private carrier contracts for all fixed labels, nil/unknown no-ops, first
recognized joined cause, exact original pointer in Unwrap, errors.Is, unchanged
messages and panic-Error privacy. Exercise the actual cleanup producer helper
through inert JSON/status pipes and the existing receiver for every known
reason, legacy/unknown reason, wrong stage, empty error, unknown fields and
malformed/trailing records. No subprocess is needed for these contracts.

Run focused process and existing fixture-classifier contracts, vet and the
configured affected-package lint plus unfiltered size/architecture gate. Two
independent plan rounds and frozen source review precede implementation
publication. The next required full gates provide the actual observation;
the metadata is diagnostic only and does not claim to fix runtime behavior.
