# Preserve fixed diagnostics at the two observed acquisition fixture seams

At source `1a19373e024ba4c2ac0748fda1306ce0ffd95429`, Required job
`121162735459` fails `TestSkillsBatchRejectsStaleAndUnresolvedBeforeEffects`
at `skills_batch_test.go:134` with the fixed selection preparation error.
Coverage job `121162735210` fails
`TestNativeHTTPSGitPublicAndPrivateCredentialHelper/false` at
`git_transport_test.go:131`: native ref resolution succeeds, then immutable
acquisition fails with its intentionally sanitized message. The nearby TLS
server errors do not establish this failure's cause. These are different
fixtures from the earlier instrumented dependency paths.

## Exact change

- In `newSavedJourney`, attach the existing `dependencyGitDiagnosticRunner`
  around the existing `sourceacquisition.OSRunner` before `skillJourney`
  copies its Acquirer. Keep its existing fixed command phase/class/status,
  four-failure cap and exact context/command/result/error delegation. On a
  failing test only, emit one fixed-label summary of delegated failure count.
  Zero observed delegated failures remains distinct from a transport error;
  neither case authorizes guessing the selection refusal's cause.
- Give each existing native HTTPS subtest a test-only Runner decorator. It
  delegates its original context and complete Command to OSRunner exactly
  once, returns the original bytes and error, and logs at most four failures.
  The outer operation label is only `resolve` or `acquire`, assigned at the
  existing call boundaries; this new decorator never reads command arguments.
  Error classification reuses the established closed policy: context,
  unsupported ordinary containment, stdout limit, forced cleanup, recognized
  ordinary-supervisor stage, distinctive positive Git exit/status, or unknown.
  No error text, streams, credentials, paths, environment or arbitrary types
  are formatted or read for classification. Opaque native TLS policy errors
  remain unknown. No production changes are included.
- Add inert contracts for exact delegation/context/body/error identity, the
  four-failure cap, silent success, fixed operation/class/status, generic
  process exit refusal and an Error-panic positive Git exit fixture. Preserve
  every existing fixture assertion and transport/credential configuration.
- Revise the existing finite diagnostic workflow on its exact temporary
  diagnostic branch. Keep the Required Linux selector, Go/tool pins, read-only
  permissions and 15-minute job budget. Run exactly the newly failed Skills
  test without coverage and exactly the native HTTPS test with the observed
  atomic coverage mode/profile, each once with `-count=1 -timeout=5m`.
  Do not repeat the two earlier passed diagnostic tests, add retries or use
  a repeated full lane to obtain one label. Full Required/Coverage keep the
  decorators for subsequent actual failures.

## Validation and qualification

Run only the inert privacy/delegation/classification contracts and compile
the affected real fixture entry points. Use configured affected-package lint
with CI's `origin/main` merge base and the unfiltered size/architecture gates.
Review the exact focused workflow command selection. Both independent plan
rounds and frozen source review precede code publication. One genuine hosted
diagnostic run may expose a fixed origin; a passing run does not establish
the historical cause. No timing, command, TLS, environment, containment,
credential, retry or success/failure semantics change.
