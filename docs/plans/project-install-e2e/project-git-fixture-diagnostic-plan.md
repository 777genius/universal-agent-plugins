# Bounded test-only immutable Git failure diagnostics — R1

## Observed failure and narrow objective

Required run40181365678/job121132946340 failed only
TestCLIModule -> TestProjectComposedAgreementColdRestoreUsesPortablePins/git,
at its initial public add (project_lifecycle_transfer_test.go:15, 0.51s).
AcquireGit intentionally replaces errors from acquireGitTransport with one
credential-safe public message. Its command sequence covers init, remote,
config, fetch, rev-parse, ls-tree, optional sparse checkout and checkout, followed
by package-path inspection/snapshot. Existing logs cannot establish which failed.
A different earlier immutable acquisition failure is not proof of the same cause.

Add diagnostics only to the disposable project's existing fixture. Do not change
product errors, acquirer/runner/process code, retries, timeouts, cleanup,
containment preconditions, public command semantics or test assertions.

## Exact proposed implementation

- Add cli/cmd/agentplugins/project_git_diagnostic_test.go with a private Runner
  decorator. It delegates once to sourceacquisition.OSRunner with the original
  context/Command unchanged and returns the exact original bytes/error object.
  A private delegate seam is used only by inert decorator tests.
- The fixture transport owns one shared mutex-protected recorder and passes it
  into both configureProjectJourney Acquirer and SourceAcquirerForDryRun. Every
  production ForScope reconstruction reapplies that same decorator. No env knob,
  executable replacement or fake Git success is added.
- Never log argv, environment, stdout, stderr or source bodies. Operation labels
  are a closed mapping after optional -C <repository>: init, remote-add, config,
  ls-remote, check-ref-format, fetch, rev-parse, ls-tree, sparse-init, sparse-set,
  checkout and show. Any other grammar is labelled unknown; its cause is omitted.
- Successful operations only increment this fixed label map; no per-success log.
  On failure emit at most four diagnostic records per transport and one bounded
  snapshot of the operation counts. Drop later diagnostics without affecting
  execution/results. This is an observational limit, not a command-admission gate.
- Error class is a fixed enum, selected through errors.Is: context_cancelled,
  context_deadline, ordinary_unsupported, stdout_limit, other_git_or_process.
  The opaque final class cannot establish authentication or containment success.
- A known operation's nested error may be logged only for this exact credential-
  free fixture authority: HTTPS127.0.0.1 endpoint with no userinfo/query/fragment,
  known disposable workspace and repo operands. Replace those locators with
  fixed placeholders, redact every remaining URI (including userinfo/query),
  and omit the entire cause if credential-style assignments/headers occur.
  Unknown/unrecognized authority prints only cause_omitted. Quote controls and
  truncate the final quoted cause to at most2048 ASCII bytes. No real repository
  URL or credential is admitted to the diagnostic output.
- The existing public CLI error remains unchanged. If all Git commands succeed
  before that generic error, report only that the remaining failure is in the
  post-command package validation/snapshot phase; do not invent an exact cause.
  A later narrower observation would require a separate proposal.

## Finite verification and next observation

Inert delegate tests prove exact one-call context/Command/bytes/error identity,
success silence, exact operation mapping/unknown omission, typed error classes,
known locator/URI/credential redaction, escaped/control and final2048-byte cap,
and four-record saturation without affecting returned errors. No subprocess is
needed for those tests. Existing real TLS transfer and shared source lifecycle
tests may run once at the normal configured limits; no retry-until-green loop.

Publish only after two plan rounds and two fresh code reviews. The next genuine
Required run will either preserve a bounded nested failure class/cause or pass
the unchanged test; one passing run alone does not explain the prior failures.
No failure is labelled flaky and no native qualification follows.

## R2 — concrete review corrections, superseding conflicting R1 wording

Root's source trace and Claude's independent R1 both confirmed that
OSRunner.Run intentionally includes argv and merged streams in nonnative errors.
The cause paragraph above is narrowed as follows; it does not authorize logging
an arbitrary wrapped error.

1. **Nonnative causes are always omitted.** If NativeCredentials is false, emit
   only the fixed operation/class and literal cause_omitted; do not call Error(),
   scan, quote or retain that wrapper text. This applies even to a known init,
   config, checkout or tree operation. Unknown grammar/authority also omits cause.
   Preserve the original error object and bytes to the caller without wrapping.

2. **Positive cause admission is exact and bounded.** Only NativeCredentials=true
   plus an explicitly known HTTPS127.0.0.1 fixture endpoint with no userinfo,
   query or fragment and known disposable workspace/repository operands can
   admit cause text. That OSRunner branch already avoids Git argv/streams in its
   native-error wrapper. After obtaining its existing error string, require
   valid UTF-8 and at most8192 input bytes; oversized/invalid input is omitted
   before any URI/credential scan or quoting. This does not claim a bound on an
   arbitrary externally implemented Error() method; the production delegate is
   the unchanged known OSRunner, and the test seam has only inert finite errors.
   Redact known locator operands and all remaining URI forms, omit credential-
   style assignments/headers, then cap the final quoted ASCII cause at2048 bytes.

3. **Counts are observational, not acquisition-phase proof.** The transport
   recorder is shared across Resolve/Acquire and scope reconstruction. With no
   observed Runner error, emit exactly “no observed Runner failure; remaining
   source phase undetermined.” Workspace creation can fail before the first
   command, and later validation/snapshot can fail outside invocations.
   Neither aggregate counts nor a final checkout proves a complete acquisition.
   Remove the R1 suggestion that this uniquely identifies a snapshot failure.

4. **All accounting is finite.** Store only the closed fixed label/class map,
   saturate each operation count at65535, and emit at most four failure records
   per transport, each with a fixed-size count snapshot. Unknown labels use one
   fixed unknown slot. Saturation only suppresses diagnostics; delegate calls
   and returned results are untouched. No URL, argv, output stream or environment
   is retained in successful-operation accounting.

5. **Pin negative observations in inert tests.** A known nonnative init error
   deliberately contains argv, stdout/stderr and a credential marker; assert
   Error() is not inspected and none reaches diagnostic output. Include
   unknown/native external-URI authority, oversized native error text, control/
   Unicode escaping, URI userinfo/query, credential-style headers/assignments,
   pre-command empty counts, prior Resolve counts, four-record saturation and
   exact original context/Command/bytes/error identity. Every omitted-cause case
   retains fixed diagnostic facts and the real unchanged failure to its caller.

These refinements preserve the original test-only scope and no-retry/no-budget/
no-guard/no-product-output invariants. R1 reviews authorize this revised plan
for final R2 review, not implementation. No diagnostic source has been written.

## Implemented candidate and author checks

The implementation is based on published d94d6c1b09e89634220e471cb1f2c03d758b0511,
not the earlier plan branch's product tree. Only the two approved plan commits
were cherry-picked. Root and Claude independently returned final PLAN GO for
772422310444bf235222e7ef430d49db7f376d0e before source edits.

Production acquirer/runner/process/CLI behavior is unchanged. Four test files
add the private wrapper, its finite contracts and existing fixture wiring.
Known native origin fetch receives cause text only after an observed successful
exact fixture remote-add in the same owned workspace. This uses one bounded
provenance slot; unknown grammar or another remote-add invalidates it. Failure
to qualify only omits diagnostics and cannot alter the delegated result.

- Initial five private inert contracts: PASS0.176s.
- Final endpoint query/fragment precision plus the same contracts: PASS0.033s.
  No vendor, Git transport, model or MCP process was used by these contracts.
- One predeclared unchanged TLS source-lifecycle plus cold-transfer selection
  ran once: FAIL9.913s. Both Git branches refused their first init because this
  local host lacks the ordinary proc prerequisite. The new fixed diagnostics
  recorded phase=init, class=ordinary_unsupported, cause=cause_omitted and
  counts init:1/all others:0. ZIP branches produced no failure. This is the
  known local prerequisite refusal, not a reproduction or explanation of the
  genuine hosted later immutable-fetch failures. No skip, retry or guard change
  was made. Full log is preserved in the task runtime as
  apm-recovery/git-diagnostic-source-lifecycle.log.
- Fresh source/code reviews and configured changed-source lint remain gates.
  No native-client or hosted failure diagnosis is promoted from local checks.
