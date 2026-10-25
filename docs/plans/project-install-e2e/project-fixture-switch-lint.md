# Project identity fixture: preserve branches in a tagged switch

Required job `121162735808` at source
`1a19373e024ba4c2ac0748fda1306ce0ffd95429` reports staticcheck QF1003 at
`installer/project_identity_test.go:72`: the fixture's client ID if-chain can
use a tagged switch.

Only `newProjectFilesFixture` changes: replace `if id == "cline"`,
`else if id == "vscode"`, and `else` with `switch id`, `case "cline"`,
`case "vscode"`, and `default`. Keep the six existing `filepath.Join`
assignments byte exact and in their existing order. The default retains the
Codex paths for every other ID. Preserve registry setup, request/modes,
`f.reopen` placement and every assertion. There is no production change or
additional fixture behavior.

Validate the full affected installer package with the repository's configured
lint and the same `--new-from-merge-base=origin/main` used by CI, then its
unfiltered size/architecture lint gate. A new test mirroring this syntax change
is unnecessary. Existing focused fixture tests may run only to resolve a
concrete remaining concern. Two independent plan rounds and frozen source
review precede the implementation commit and prompt publication.
