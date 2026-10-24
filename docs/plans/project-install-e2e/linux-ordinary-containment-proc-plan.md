# Linux ordinary containment proc prerequisite plan — R1

## Observed failure and preserved evidence

The unchanged Linux bootstrap test TestDefaultRunnerKeepsCapturedStagingAndContainsSetsidDescendant failed persistently in the root serial run, after the integrated-core-f9 suite. One authorized exact reproduction used the ordinary production source at 4a1634f4906453eac0b87eddbcc54246226dcfdd, Go1.26.10 via run_go.py, -p=1, and syscall-only strace. It failed in1.440s with context cancellation plus the existing one-second sole-Wait cleanup deadline. No deadline or test assertion was modified, no vendor executable was invoked, and no second test reproduction was run.

The raw full trace remains at /dev/shm/uap-project-runtime-5e2d9d9f7f5b/bootstrap-cancel-once.strace. A bounded literal syscall excerpt and the separately captured live-thread proc proof are retained with hashes under docs/evidence/process-containment/linux-proc-children-unavailable/. The excerpt is not represented as a full trace.

The actual test supervisor killed the npm process group, observed/reaped that manager and exited. The setsid descendant1428797 was not killed by production supervision and survived until the test's stable-pidfd fallback killed it about1.02s later. This descendant held inherited output pipes; the parent process was already reaped, but exec.Cmd.Wait could not finish its I/O drain inside the existing bound. The diagnostic is truthful; increasing its timeout would mask the escaped descendant.

An independent read-only probe found /proc/<current-live-pid>/task/<current-live-tid>/children and /proc/thread-self/children both absent with ENOENT while the same process's task directory and stat were valid. No child was launched by that proc proof. This establishes a missing proc interface on this environment; it does not identify a particular kernel configuration or claim a generic namespace defect.

## Source cause and product boundary

linuxDirectChildIdentities in install/integrationctl/adapters/process/osrunner_tree_linux.go reads every task's children file. It currently treats ENOENT/ESRCH as a disappearing-thread race without proving that the thread disappeared. When every live thread lacks that interface, an ordinary supervisor can infer an empty descendant tree. Three time-separated empty scans repeat the same blind observation and cannot repair this capability gap. This is an unsupported environment plus a product fail-open observation classification, rather than evidence that the existing cleanup deadline is too short.

Do not fall back to numeric PID polling, /proc-wide ancestry sampling, process groups alone, a relaxed empty-tree rule, or a longer cleanup interval. Keep the current stable pidfd signaling and all existing cleanup assertions.

## Bounded correction proposed for two independent plan rounds

1. Add a narrow ordinary-containment launch preflight in the process adapter. On Linux it checks the required proc children interface for a positively live current thread, before wrapping/starting a supervisor, and the supervisor repeats this check before starting the requested command. Lock the current OS thread for this short self check so its own identity cannot disappear through Go thread migration. Missing children for that known-live thread produces a distinguishable unsupported-prerequisite error. Other read/identity errors remain failures; they are not silently skipped. The capability query certifies this prerequisite, not successful future descendant shutdown.
2. Expose the same process-inert prerequisite query through a small OS method so Linux bootstrap runtime tests can check the actual production preflight before launching a descendant. Non-Linux ordinary behavior remains unchanged; no other platform capability is newly asserted. Prefer an explicit unsupported error sentinel/type over matching diagnostic text. Do not change public command/result schemas or lifecycle policy.
3. During real descendant scans retain legitimate disappearing-thread handling only after a positive identity check: capture/revalidate the relevant task identity and accept missing children only when that exact task is proven gone/exiting, or belongs to a different reused identity. If the same live task still exists, missing children is an observation error and cannot contribute an empty snapshot. Unknown/permission/read errors also fail closed. Use a tiny private dependency seam for inert read/identity tests; no production environment switch or fault knob.
4. The bootstrap containment test may skip only on the exact distinguishable unsupported preflight before any manager/descendant launch. Other errors fail. Keep every existing cancellation, stable-identity, setsid, captured staging, deadline and sole-reaper assertion unchanged on supported hosts. Record any unsupported local run as not qualified, not a containment pass. Existing genuine hosted Linux containment must still qualify the runtime behavior.
5. Keep bootstrap manager source/staging behavior and custom inert Runner tests unchanged. If inspection reveals additional real-process tests in the affected packages need the same prerequisite, enumerate them for reviewers first; do not add blanket package skips or reinterpret unrelated execution errors.

## Mandatory regressions and finite validation

- Inert injected missing children file for a positively live same-identity task refuses; no successful empty-tree result and no command launch callback is reached.
- A known disappearing task can still be omitted without an error; a reused task identity cannot authorize signaling of its replacement. Unknown/read/permission errors are retained.
- Unsupported self-preflight is observed before requested command execution; the fixture's command marker remains absent. A supported self interface with an empty child list permits the ordinary launch path.
- Existing descendant traversal identity, quiescent-empty, stable pidfd, bounded reaper and cleanup causality tests remain unchanged and pass where their declared primitive is available. Do not run a broad suite for this correction.
- Exact bootstrap runtime test on this environment must report explicit pre-start unsupported capability (if still missing), with no manager/setsid artifacts. That result is not a substitute for supported-host cancellation containment evidence.
- Two independent R1 and R2 plan reviews must precede production edits, then two frozen-source code reviews must precede publication. Root decides the bounded implementation after review. No production source has been edited for this plan.
