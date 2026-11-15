# Copilot project installation and remembered folder trust

## Goal

A confirmed Copilot project installation should prepare the selected project for
the next ordinary `copilot` session without a second folder-trust dialog. The
existing install review discloses that Copilot will remember this project;
`--yes` accepts that reviewed plan. No launcher wrapper or extra required step.

GitHub documents `trustedFolders` in the selected Copilot home's `config.json`:
https://docs.github.com/en/copilot/how-tos/copilot-cli/set-up-copilot-cli/configure-copilot-cli
This is separate from tool, path and URL permissions. The existing plan remains
honest about native discovery, authentication and session policy.

## Implementation

- Keep the behavior inside the Copilot adapter and the existing native-config
  parser/ExactFile transaction. Bind the write to the captured user profile and
  canonical selected project, including `COPILOT_HOME`.
- Add only the selected project to `trustedFolders`. Preserve other folders,
  unrelated configuration and permission settings. Reject ambiguous or unsafe
  roots, malformed documents and symlink paths.
- Prepare trust before the existing project-component mutation; compensate the
  trust write if component application fails, preserving concurrent edits.
  Respect committed-cleanup errors. Dry runs and verification never grant trust.
- Remembered trust is a user decision and survives package removal. Removal still
  deletes only exact owned skills and MCP entries.
- Reuse native Copilot lifecycle evidence. Add an account-free real CLI test
  starting from an untrusted project, checking fresh-process skill/MCP discovery,
  sibling isolation, repeat/update/repair/remove and unchanged tool permissions.

## Scope and review

No changes to other clients, generic installation orchestration, public JSON
schemas, authentication, blanket tool approvals, or production release.

Estimate: 150–220 production lines and 300–500 focused test lines. Complexity
4/10; expected reliability 9/10; confidence 8/10 before execution. Two review
rounds: transaction/consent design, then independent implementation and evidence.

The design review identified two main risks: compensating a trust write after
components have already committed, and changing unrelated native user settings.
Use existing effect classification, exact byte comparison/rollback, and the
native document parser to address these without another state engine.

## Verification

Run focused parser/adapter and public CLI lifecycle tests, `make lint`,
`make test-core`, applicable cross-platform checks and genuine native Copilot E2E.
Keep native executable pins and hosted-runner evidence checks intact. A skipped
or queued case is not a pass. Push each coherent commit and maintain a Draft PR.

## Evidence (in progress)

Copilot 1.0.103 was checked in an isolated profile without credentials: an
untrusted A exposes no project MCP entries, adding only A to `trustedFolders`
enables A in two fresh processes, and sibling B remains undiscovered. This
confirms the native mechanism; product implementation and final validation are
still pending.
