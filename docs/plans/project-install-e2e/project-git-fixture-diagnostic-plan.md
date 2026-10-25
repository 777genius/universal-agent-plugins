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
