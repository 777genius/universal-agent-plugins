# Copilot: exact message-role membership

## Concrete finding and isolated scope

The published Copilot source is `9b8ddc5332d70ca0c8d39c41c5ce60f3bba1c6e7`,
tree `8386a313607a2d1acfd5583bd9335d9ce7af1340`. Its private scripted-provider
validator uses a delimiter-string substring search to recognize message roles.
An otherwise valid full request whose message is
`{"role":"user|assistant","content":"fixture"}` passes that search and every
remaining message/envelope check. `system|developer` and
`user|assistant|tool` provide equivalent counterexamples. Root and an
independent critic confirmed the complete source path statically; this is not
a claim of a new vendor observation.

The separate nine-lint proposal deliberately preserves the existing accepted
input set. This amendment addresses the semantic enum defect independently,
rather than attributing a grammar correction to formatting or lint cleanup.

Use an isolated new worktree and branch from the exact published source.
Do not edit another continuation's checkout, integration branch, lint plan,
checkpoint or pending native capture. Scope is two existing Copilot test files
plus this plan/validation record. No production adapter or shared validator
changes.

## Correction after two independent plan approvals

In `copilotProjectMessage`, retain the required string type, then use exact
membership over precisely the existing five intended labels: `system`,
`developer`, `user`, `assistant`, and `tool`. Reject every other string with
the existing finite message-role error. Preserve all content/null handling,
tool-call identities, optional fields, declaration/envelope constraints and
provider state transitions. Add no normalization, splitting, case folding,
alias or accepted role.

Extend the actual full-request contract seam with the demonstrated composite
roles, using the existing valid request recipe and changing only its role.
These cases must fail on the old substring validator and pass after the exact
enum correction. Include positive full requests for each canonical role with
its existing required fields, so the intended five-role grammar is preserved.
Retain all existing unknown-field, malformed-content, tool, provider-return,
privacy and original-config controls.

## Validation, integration and publication boundary

Obtain two independent plan GOs before code, followed by two independent reviews
of the exact changed bytes. Formatting and diff checks must remain clean.
Run focused inert Copilot/shared contracts in a free coordinated Go slot; no
vendor executable or positive hosted guard may run locally. Verify the changed
code with the applicable lint gates. Existing unrelated nine-lint findings
remain visible and must be reconciled by the separate reviewed amendment.

Commit the isolated plan and correction as separate local increments. Do not
push or alter the authoritative leaf while the current `9b8ddc53` native
capture is still needed. Publication remains root-owned after writer
coordination, a fresh exact remote-head check, applicable checks, and semantic
integration with any separately reviewed lint changes. No forced update, new
PR, release or VS Code work. Historical failed captures and all existing
qualification/authentication/model flags remain unchanged; the static fix
does not supply native session or lifecycle evidence.
