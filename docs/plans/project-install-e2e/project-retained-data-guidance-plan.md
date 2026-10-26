# Preserve selected project context in retained-data follow-up commands

Pre-code correction plan, 2026-10-23. This supplements the approved core scope
contract and its removal acceptance case. Root requested a concrete correction
after a genuine Codex macOS project-removal artifact exposed the wrong hint.
No source or test implementation is changed at this checkpoint. Both independent
plan/fix GO reports must be committed before implementation.

## Evidence and exact render paths

The actual hosted artifact
`research/hosted-evidence/codex-darwin-40011489662/project-remove-installer.json`
records a project data receipt and successful retained-data removal, but its
`retained_data_action` says only:

`agentplugins remove <installation-id> --purge-data`

The default CLI scope is user, so copying that instruction loses the selected
project and cannot address its retained receipt. It is also dependent on the
future invocation cwd if corrected with project scope alone. No new native run
is needed to establish this rendering defect.

The bounded source audit found two producers:

1. `removeManySession.recordRetainedData` in
   `cli/internal/agentpluginscli/remove_multi.go` builds RetainedDataAction,
   which is emitted in JSON and human `Next:` output.
2. `savedLifecycleSelection.renderNextAction` in
   `cli/internal/agentpluginscli/saved_lifecycle.go` emits the human saved-remove
   `PLUGIN_DATA preserved; after removal completes, run ...` hint.

The selected App already owns canonical scope/root after `forOptions` and
production ForScope composition. The first producer has that App; the saved
lifecycle caller has it before rendering. The render method currently receives
only the environment ID rather than the selected root. Existing user tests in
`cli_test.go` assert the current words and path-free user result. The existing
`quoteResumeArgument` helper supplies the repository's shell-escaped command
presentation contract for spaces, apostrophes and metacharacters.

## Narrow implementation

Add one small CLI rendering helper for the retained-data purge command. Its
user result remains exactly `agentplugins remove <installation-id> --purge-data`.
For a selected project App it appends both explicit `--scope project` and
`--project-root <escaped-selected-canonical-root>`. Use the original validated
App.ProjectRoot, not raw option text, the current cwd, a managed package path,
a receipt locator or an inferred parent. Quote the argument with the existing
`quoteResumeArgument` convention; do not interpolate an unescaped root into a
shell fragment or invent a new platform shell grammar.

Use this helper in both producers. Thread the selected App or its already
validated scope/root into saved renderNextAction, retaining the selected
environment ID used for alias/target selection. Keep the surrounding user words,
JSON keys, ordering, installation selector and post-removal purge policy.
Do not add a target or external-uninstalled attestation to the hint: the
follow-up acts only after bindings are removed under existing ownership rules.

The root appears only in an operational project follow-up command, where it is
needed to select the original namespace. It must not enter manifests, locks,
source identity, persisted receipts, path-free retained-data receipt objects or
default user output. No automatic command execution, data deletion, native
cleanup, trust/auth flow, new flag, new dependency or lifecycle behavior is added.
Actual explicit purge remains guarded by the existing selected-scope receipt
ownership and confirmation rules.

## Meaningful verification

Extend the real public CLI composition fixtures with a selected project A whose
directory name contains spaces, alongside a populated user environment and
project B. Use the existing inert host/security boundary and production loader,
store, planner, installer and native file transactions; no genuine vendor
session is required for this output correction.

Exercise the two paths separately:

- Imperative local/no-save add and removal produce JSON retained_data_action
  and the matching human command. Assert the selected project scope/root is
  retained as one escaped argument, with the installation ID unchanged.
- Saved add and human saved removal produce the context-preserving saved hint;
  do not silently skip that producer because JSON saved output has no hint.
- Reuse the emitted project command from an unrelated cwd through the public
  CLI selection path, with normal explicit confirmation where required, and
  prove only A's retained data/receipt is purged. Preserve user and B native,
  state, owned data markers and the shared immutable store. Snapshot other
  namespaces before the follow-up to detect a wrong default-user selection.
- Verify the actual shell word boundary for a space-containing root rather than
  merely searching for a quoted substring. On POSIX hosts, an inert shell
  argument-capture step can parse only the generated fixed command and return
  its arguments for the public in-process CLI; it must never launch the purge
  command through a shell. Cover apostrophe/metacharacter escaping under the
  existing POSIX presentation convention without claiming universal cmd.exe or
  PowerShell quoting qualification.
- Retain the established user tests and assert both user hints remain byte-for-
  byte unchanged, including no project flags/root leakage. Preserve existing
  failure/incomplete-removal wording and the no-purge-before-removal guard.

Run only the new public regressions and affected existing user removal/saved
lifecycle tests, plus exact changed/whole-file lint and diff checks. No broad
native matrix or new vendor process is needed. Freeze the implementation for
two independent code reviews and fixes, commit their reports, then send root a
clean SHA for coordinated feature-stack publication. This correction makes no
new native, all-client E2E or release qualification claim.
