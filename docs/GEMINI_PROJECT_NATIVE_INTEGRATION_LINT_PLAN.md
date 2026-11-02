# Copilot and Gemini: nine integration lint corrections

## Evidence and exact source boundary

This is a review proposal, not an implementation or a qualification result.
Root's combined integration is commit
`d1b1f51c98ffd0142993456b51762f7331aed6b7`, tree
`7906245dd8cb1287ebd78dd13b300aec99091fe3`, based on current main
`bd205dda` and the active project program. Root's actual full-delta all-enabled
lint against main reports exactly nine findings in
`/tmp/uap-active-nine-integration-lint.log`. Integration Go contracts passed
in 5.273s and 216 Python contracts passed in 31.782s before this proposal.

The authoritative leaves are Copilot PR527
`9b8ddc5332d70ca0c8d39c41c5ce60f3bba1c6e7`, tree
`8386a313607a2d1acfd5583bd9335d9ce7af1340`, and Gemini PR526
`fa7b9db9a94e39392934be5b3820f6266900c13b`, tree
`0310327c0dc9ec64036b99bb99387bdecf750cbc`. Their relevant source files are
byte-identical in the combined integration. Their new genuine native captures
are still pending; preserve those source identities and wait for their raw
evidence before applying or publishing a follow-up to either leaf.

## Exact changes

| Client and existing file | Actual finding | Proposed correction |
| --- | --- | --- |
| Copilot provider, message validator | gocritic argument-order heuristic at the role membership expression | Name the exact existing delimiter string as a local constant and keep the same `strings.Contains(collection, token)` call and refusal. |
| Copilot provider, message validator | staticcheck De Morgan expression | Replace the negated assistant-and-calls term with its exact equivalent `role != "assistant" || !hasCalls`, preserving the surrounding missing/null-content condition. |
| Copilot provider, `start` | noctx `net.Listen` | Change only this private method to `start(ctx context.Context)` and use `(&net.ListenConfig{}).Listen(ctx, "tcp4", "127.0.0.1:0")`. |
| Copilot contract, existing JSON controls | gofmt at the hostile JSON table | Apply ordinary gofmt; retain all cases, order, bytes and assertions. |
| Copilot contract, hard-link refusal diagnostic | misspell `hardlinked` | Use the plain diagnostic `immutable file with multiple links accepted`; keep the actual link/refusal test unchanged. |
| Copilot contract, provider request authoring | noctx `httptest.NewRequest` | Use Go 1.22-compatible `http.NewRequestWithContext(context.Background(), ...)`, check its error with `t.Fatal`, and retain the exact request target as `RequestURI`. |
| Gemini contract, native stream role branch | staticcheck tagged switch | Convert only the existing user/assistant/default if-chain to `switch row["role"]`, with identical stage checks, assignments, returns and default refusal. |
| Gemini contract, provider request authoring | noctx `httptest.NewRequest` | Use the same Go 1.22-compatible context-aware request construction, checked error and exact `RequestURI` preservation. |
| Gemini provider, `start` | noctx `net.Listen` | Apply the same private context parameter and `ListenConfig.Listen` change as Copilot. |

Each provider has exactly one actual `start` caller, in its existing
`agentplugins_<client>_project_native_attempt_test.go`. Pass that fixture's
already-held `f.ctx` from the caller. This adds the two caller files to the
four finding files; do not add a provider context field, alter constructors,
or add a fresh deadline. Gemini's complete-cycle provider already uses its
held context and is not part of this change.

The authoritative leaves still declare Go 1.22 even though current main uses
a later Go version. Do not use `httptest.NewRequestWithContext`, a newer API.
The replacement retains method, absolute URL/route/query, body, header and
host mutations, response-recorder path and direct handler invocation. Retain
`RequestURI` as the original absolute target to preserve the incoming-test
request shape. No request is sent to a real service by these controls.

The role warning is a false-positive argument-order heuristic, not evidence
that the arguments should be reversed. Naming the existing collection makes
the intent explicit without changing the accepted input set. The primary
go-critic rule in `checkers/rules/rules.go`, Git blob
`f7bdf35aed2f539428de7359d477f43de856ab68`, expressly exempts a named first
argument. Preserve the original membership behavior; do not fold a distinct
role grammar change into this lint correction.

## Preserved authority and behavior

Keep listener address/family, provider routes, body bounds, exact credentials,
HTTP timeouts, ordinary server close, request/response counts and one-call
ordering unchanged. Using the fixture's existing context only binds listener
creation to its original work budget; it does not replace provider shutdown
or process/cgroup supervision.

Keep all source and executable pins, independently authored installed files,
the held firstLaunchAt/original-config digest, Gemini trust/settings/policy,
native argv/environment, package/data/receipt authority, permission and
approval checks, privacy/redaction and artifact admission unchanged. The
original work/total deadlines and fatal cleanup checks remain mandatory.

Copilot remains its deliberately failed native-grammar review capture, with
all five qualification/authentication/real-model flags false. Gemini remains
its deliberately failed complete-cycle first capture, with the same false
flags and separate future completion requirement. Do not promote historical
or newly pending artifacts as a consequence of these lint corrections.

Scope is the two existing provider files, two existing contract files, two
existing legacy-attempt caller files, and a truthful plan/validation record
per authoritative leaf. No shared Python/Go helpers, workflows, selectors,
production adapters or VS Code-specific files change.

## Review, validation and publication

Obtain root R1 and independent Kimi-owner R2 against this exact proposal
before implementation. Review actual new native artifacts first and reconcile
the exact current leaf heads. Preserve any separately required raw-evidence
fix as its own reviewed change; do not silently expand this plan.

After the two plan approvals, apply the small changes in isolated worktrees
from the authoritative current leaves. Obtain independent exact-code review.
Use one coordinated exclusive Go/lint slot for the existing focused Copilot
and Gemini contracts, gofmt/diff checks, all-enabled affected lint and the
unchanged unfiltered whole-file architecture/size lint. Root should also
rerun the actual integration full-delta lint against main to close all nine
reported findings. Do not add mechanical mirror tests or rerun vendor clients
locally. A lint or compiler failure remains a failure to resolve, not a reason
to add suppressions or change toolchain declarations.

Root alone publishes each frozen reviewed leaf after a fresh non-forced
expected-head check, then reintegrates it. Preserve the current pending run's
source/evidence identity. Native observations from an older head are not
retroactively relabeled as results of these later source changes.

## Accepted implementation record (2026-10-27)

Root R1 and independent Kimi-owner R2 approved the exact proposal SHA256
`33cf3a7e9400e451b0ced924c71029fdaf2c21efb2110618489bf3aaafddd6a4`
before edits. The genuine source-bound runtime 40553310025 has now completed and
its current native/frontend artifacts were independently reviewed before this
follow-up. Historical observations retain their original source identities.

This commit applies only the three existing Gemini Go files named in
the proposal. Evidence-driven changes are separate commits with their own
accepted plans. Validation results are recorded after the required gates.

### Validation

After the approved mechanical edits and the separate temporary-parent fixture
correction, all focused `^TestGemini` inert contracts passed (1.090s).
The unchanged all-enabled root lint against base443fa8d and the unfiltered
whole-root revive/funlen/gocyclo/gocognit/dupl/depguard gate each report
zero issues. No vendor or public installer executable was run locally.
The complete-cycle runtime still requires a fresh genuine capture.
