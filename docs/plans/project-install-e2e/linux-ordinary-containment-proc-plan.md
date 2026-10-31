# Linux ordinary containment proc prerequisite plan — R2

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

## R2 precision and affected surface inventory

R1 exact ec1c179b received independent architecture GOs from Root and apm_vercel_scope. Both required this explicit impact inventory and precise unsupported classification before code. Root additionally required bounded proc reads and strict task identity parsing.

All proc task/stat/children reads introduced or reused by the new prerequisite/absence classifier must have finite byte limits and strict parsing of PID/TID/starttime/state/parent fields. Use a 4KiB stat cap and 64KiB children cap; read at most that cap plus one byte, refuse overflow/trailing malformed content and unknown identity errors, and keep the existing traversal process/deadline bounds. Use per-call small read/identity operations for inert injection, never mutable package globals. The unsupported sentinel is reserved for ENOENT of the required children interface while BEFORE and AFTER observations prove the same live task; permission errors, malformed identity, unknown read errors and a changed task are failures or explicit race outcomes, not unsupported skip authority.

The ordinary wrapper fronts OS.Run, RunWithTreeExitGrace and RunWithDescendantExitGrace. On this host all of those production commands must now refuse before requested-child launch, including ordinary Git acquisition, scanners, client version/list/probe calls, legacy integrations and dependency bootstrap. This is a deliberate fail-closed unsupported-host contract. It is not a claim that only one bootstrap test is affected. The strict duplex/cgroup path remains unchanged.

The first implementation will gate only these named real-process tests on the exact production prerequisite before any child launch (helper-only child branches must retain their existing behavior):

| File | Exact tests needing ordinary prerequisite |
| --- | --- |
| bootstrap/process_linux_test.go | TestDefaultRunnerKeepsCapturedStagingAndContainsSetsidDescendant; TestDefaultRunnerExecutesOnlyLockedCommandWithSanitizedEnvironment; TestDefaultRunnerCancellationStopsDescendants |
| process/osrunner_test.go | TestOSRunnerPreservesCommandOutputAndExitCode; TestOSRunnerCleanExitWithoutOtherGroupMembersRemainsSuccess; TestOSRunnerBoundsStdoutAndReturnsLimitError; TestOSRunnerStdoutLimitStopsProducerBeforeRemainingOutput; TestOSRunnerStdoutLimitPreservesNonzeroExit; TestOSRunnerStdoutLimitPreservesConventionalPipeExitCodeForGenericCommands; TestOSRunnerBoundsStderrWhileRetainingDiagnosticEdges |
| process/osrunner_tree_linux_test.go | TestOSRunnerSurvivesRemovedOwnExecutable; TestOrdinaryCommandContainmentDoesNotRequireDelegatedCgroup; TestOrdinaryRunWorksWhenDelegatedCgroupIsUnavailable; TestOrdinarySupervisorDescriptorsAreClosedBeforeCommandExec; TestOSRunnerRapidSuccessfulCommandsPreserveNaturalExit; TestOSRunnerRapidShellCommandsWithShortLivedChildrenPreserveNaturalExit; TestOSRunnerCleansSameProcessGroupMemberAfterLeaderNaturalExit; TestOSRunnerDescendantGraceAcceptsOnlyNaturalHelperExit; TestOSRunnerDescendantGraceStillRejectsForcedCleanup; TestOSRunnerPreservesDiagnosticAndStatusWhileCleaningNaturalExitGroup; TestOSRunnerTreeGraceCleansImmediateDescendantThatCreatesNewSession; TestOSRunnerCancellationContainsImmediateSetsidDescendant |

Existing requireDuplexCapability gates are not weakened or repurposed; a dual prerequisite test keeps both. Inert tests of injected traversal, quiescent-boundary counting, stable identities, output buffers and sole-reaper behavior remain unconditional. A named runtime test that reports unsupported is not a runtime containment pass.

Production composition/caller inventory traced read-only: cli/cmd/agentplugins/compose.go injects OS into client discovery, lifecycle and scanner boundaries; clients/shared/native_registry.go uses tree grace; clients/claude/probe.go uses descendant grace; agentplugins/adapters/sourceacquisition/{acquirer,git_transport}.go directly use ordinary/tree-grace OS for Git; integrationctl_service.go and adapters/{source,claude,gemini} retain ordinary legacy defaults; authoring/bootstrap/bootstrap.go invokes OS.Run. Existing external source-acquisition, scanner and client probe integration tests may therefore refuse on this host too. No broad package skip or fallback runner will be added to make those tests appear qualified; supported hosted CI must exercise them. Any additional per-test prerequisite edits require their exact test names and review before expanding this bounded list.

A successful self prerequisite query proves availability of the bounded proc interface only. Actual supported-host containment qualification still requires the unchanged real cancellation/setsid/cleanup assertions. This environment's unsupported result will remain explicitly distinct from those hosted qualifications.

R2 exact 09e24c58 independently received PLAN GO from Root and apm_vercel_scope before production edits. Root approved explicit numeric cap supplement (stat 4KiB+1; children 64KiB+1; existing 1000-process traversal/deadline). These are prerequisite bounds, not runtime qualification. No bootstrap/process production source has yet been edited.

## Implemented candidate and bounded validation

The candidate now performs the same process-inert prerequisite in the ordinary parent constructor and again in its isolated supervisor before starting the requested command. The public OS.OrdinaryPreflight query reports that prerequisite only. Runtime task scans in both the supervisor traversal and passive parent tracker reuse the same per-call observation helper. A missing interface for a still-live task has a distinct unsupported sentinel and deliberately does not wrap ENOENT: existing ancestor traversal must not misread it as a disappeared process. Missing stat inside a still-present task and changed/malformed/permission/unknown observations fail closed.

Concrete bounds are stat 4KiB plus one, children 64KiB plus one, and task-directory enumeration 1000 entries plus one, reusing the existing traversal count. A process exceeding that task enumeration budget produces an observation error; it cannot contribute an empty-tree proof. Task names and PID/starttime/state fields are parsed strictly. Operations are injected only per call in private inert test seams; there is no environment override or mutable fault global. Strict duplex cgroup handling, existing cleanup/reap deadlines and supported-host containment assertions remain unchanged.

Focused local Go1.26.10 validation with -p=1:

- Process inert observation/cap/identity/prelaunch tests plus existing traversal/quiescent-boundary/sole-reaper checks: PASS0.021s. Covered 10 disappearance/error subcases, exact/over byte and directory bounds, invalid identities, actual unsupported OS.Run refusal with command marker absent, and injected available empty interface retaining ordinary wrapping without starting a child.
- The exact22 reviewed real-process tests reported explicit pre-launch unsupported capability and SKIPPED on this host (process0.004s; bootstrap0.011s). Their raw log is retained. This result is not runtime containment qualification and no manager/setsid child was launched.
- Existing custom inert Runner captured-staging regression PASS0.020s. Custom bootstrap runners keep their existing contract.

Raw selected local test logs and member hashes are saved under docs/evidence/process-containment/linux-proc-children-unavailable/. The original real failure trace/proc proof remains preserved. No vendor, native client, MCP server, model, authentication, trust change or broad suite ran for this candidate. Supported hosted Linux runtime qualification is still required. The source must receive two independent frozen-source code reviews before publication.

### Final review correction and validation

Root's review of candidate4a1b536e required the new ordinary prerequisite to run after helper-only exit/return branches. The final delta moves only that added query in14 helper-bearing tests; original helper behavior and existing duplex-query order stay intact. Other8 ordinary-query guards remain at test start. As a result, final local runtime accounting is16 ordinary-prerequisite skips plus6 existing duplex-prerequisite skips,22 unqualified runtime tests in total. No test assertion, timeout, skip predicate or cgroup rule was weakened.

The new helper's three deliberate non-errno wrapping boundaries render the original errno as message text; the unsupported sentinel remains the sole wrapped cause. This retains the required distinction from disappeared ancestors and satisfies errorlint. State membership, integer range and constructor-only CommandContext corrections preserve behavior.

Final combined focused validation after those corrections: process PASS0.037s (inert assertions passed; runtime skips excluded from qualification), bootstrap PASS0.025s (custom Runner assertion passed;3runtime tests unsupported). The final raw combined log is retained separately from earlier candidate logs. Pinned golangci-lint2.13.2 changed-source check over only process+bootstrap, concurrency1/GOMEMLIMIT512MiB/90s: PASS0issues. No broad module lint or vendor run was performed.

## Final independent code review receipts

Both reviewers approved exact final source932ed0f9fc86d5bde6b2c8c08f91354be4afce63 (treef7f9c53093ffd88f873ace86be7405ba0aab7913) before publication.

- **Root (`/root`) — final CODE GO.** Reviewed the full4a1b implementation and final4-file correction. All14 added guards follow helper-only exits/returns and precede parent child launches; existing duplex order is preserved. Explicit errno message rendering keeps missing-live-task failures distinct from ENOENT ancestry disappearance. IndexByte, integer range and constructor-only CommandContext changes preserve behavior. Accepted the inert local passes and22 explicit unqualified runtime skips; supported hosted Linux runtime qualification remains mandatory.
- **APM (`/root/apm_vercel_scope`) — final independent CODE GO.** Reviewed the full4a1b production/test delta and final932 correction. Confirmed helper branches and existing duplex order are intact, bounded task observations fail closed, missing-live children produce the exact unsupported sentinel and other observation errors cannot provide empty-tree proof. Accepted the equivalent lint corrections and truthful final SKIP provenance. Performed source review only during this final review; no new build, vendor or native execution was claimed.

The separate Claude hosted success run40139625617 is preserved under docs/evidence/project-scope-native/claude-2.1.263-prepared-success-40139625617/. It qualifies the committed earlier Claude/grouped Prepared fix, not these later Linux containment changes.

## Subsequent genuine hosted bootstrap runtime subset

Existing Authoring native offline run40143431447, Linux amd64 job121019719395, ran exact published source1bc5b898d8085e707fb405ed0c8f84ca2454157f and succeeded. The original archive and raw JSONL prove all three reviewed real bootstrap runtime tests ran and passed without bootstrap skips: captured staging plus detached setsid containment0.08s; locked command plus sanitized environment0.07s; descendant cancellation0.04s. Full bootstrap package passed0.318s. Artifact12579492503 (109006bytes, SHA256078da3d0930e82647a0ed78139f423932621edc6eff5eaad2a5ba0036af6795e) was independently checked for exact length/hash/CRC. All104 original bootstrap-package JSONL lines and original metadata are preserved at docs/evidence/process-containment/linux-proc-children-unavailable/hosted-bootstrap-40143431447/ with full-source/member hashes and an explicit byte-line extraction predicate.

This closes the supported-host qualification of those three bootstrap runtime tests only. The workflow package inventory does not include the separate nineteen ordinary process adapter tests. Their current-source full qualification remains assigned to the unchanged Required/test main-base lane invoking make test-required, whose unfiltered ./cli/... and ./install/integrationctl/... commands include both packages. Local unsupported prerequisite skips remain unqualified. No workflow, selector, assertion or retry was added; no vendor ran locally while preserving evidence.
