# Windsurf: preserve the separately selectable legacy startup capture

## Observed CI problem and narrow correction

Published source `ae452ad7526f09f26efaff1b2a7a14647de8562c` failed root lint
in run `40553344674`, job `122362700881`. The six unused findings belong to
the retained shared startup helper and its reachable fields/methods. The new
selected component test had become the leaf's only native entry, leaving
the old startup implementation without a test entry in this isolated branch.

Root independently approved plan R1; the desktop reviewer approved R2 after
reading the selected test, original helper/guards and exact frontend selector.
The correction adds a real separately selectable diagnostic test,
`TestAgentpluginsDevinNativeProjectStartupAttempt`, delegating to the original
`projectNativeCLIRuntimeAttempt(t, "windsurf")`. The selected
`TestAgentpluginsDevinNativeProjectRuntimeAttempt` remains byte-for-byte
unchanged and continues to run component first capture only.

No frontend case, selector, opt-in, workflow, environment, production adapter,
native command, deadline or qualification rule changes. The old helper retains
its original opt-in and genuine-host guards, complete protected authority,
intentional failure and all-false qualifications. Component CI does not select
or qualify the separate startup diagnostic.

## Exact review and validation

Code commit `9c8c7475cde7dfc06f6f89a822752f68283454e0` has reviewed tree
`8caa7ea43c20c82d50e326c2dd230d7389a18f44`. Its sole changed file has SHA256
`4354a723212275900eb3afb449919e1017d0f75b9c5723c069eb34bdade829d2`.
The independent desktop reviewer verified the exact tree, hash and complete
five-line delta and gave CODE GO without findings. This documentation changes
no reviewed source bytes.

Focused inert `^(TestWindsurfComponent|TestProjectNativeCLI)` contracts passed
in 1.479 seconds. Runtime/component Python contracts passed all 24 tests in
0.144 seconds. The all-enabled root lint gate against current PR base
`443fa8dae7d5ffbd352b54abe1dc9c92b3e57fa2` reported zero issues after
recovering the checkout's shallow Git ancestry. The unchanged unfiltered
whole-root revive/funlen/gocyclo/gocognit/dupl/depguard gate also reported zero
issues. Gofmt and whitespace checks are clean. No vendor or positive hosted
guard ran locally. Root owns publication and fresh current-source CI review;
no native qualification, account action, release or VS Code work is included.
