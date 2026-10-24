# Project group removal admission — R1

The actual composed SDK Cline+Codex project lifecycle now passes install,
read-only replay/unchanged update, successful/failed host callback, changed
update and typed missing native-directory repair. Its final grouped removal is
blocked before effects by installer/group_remove.go's historical beta guard:
zero-selected group removal permits only Claude/Codex. Both single removals
already work and core RemoveGroup supports scoped per-client deactivation.

This slice admits the already supported project clients at that SDK guard only.
Preserve the user-scope historical guard, selected-delivery authority, exactly-two
SDK target cardinality and OpenCode host-snapshot restrictions. No new client
capability, native command, data purge, schema, interface or release claim.

## Existing authority chain

Public Prepare first validates request scope/project root against the configured
engine and ClientScopeSupported for every requested project client; this checks
both the declared policy and concrete project layout/native-root hooks. Group
validation then checks the existing cardinality, duplicate IDs and registry.
planRemoveGroup finds each saved binding and always calls removalPreflight for
a present target. That verifies binding scope/client/profile, selected native
entry, recorded managed digest, independently resolved exact target and owned
PLUGIN_DATA. Apply repeats removalPreflight, confirms exact frozen bindings and
profiles, and calls core RemoveGroup. The core resolves exact scoped bindings,
rechecks directory/receipt ownership and deactivation before transaction commit.
No new admission can bypass those checks.

## Minimal change

Replace only the historical non-Claude/Codex refusal with a positive project
exception: explicit request ScopeProject, a nonempty selected project root equal
to the configured engine root, and ClientScopeSupported for the current target.
Use the existing capability API, no new list of agent IDs. Keep the full original
condition otherwise. Do not infer scope from a saved path or counterfeit HOME.
Absent qualified project targets may retain the existing NoChange path; a present
binding must still pass all existing preflight/frozen-binding checks. Nonzero
selected delivery retains the original behavior and authority checks.

The helper is pure capability/context admission; do not add filesystem reads,
profile discovery, effects or new fallback. The public boundary already performs
canonical project validation. Preserve errors and all user-scope behavior.

## Finite acceptance

- Real SDK Cline+Codex project group install, reopen and remove must remove only
  owned native skills/MCP entries and managed packages, with original user, B and
  unrelated A files preserved and forbidden runner called zero times.
- Repeated group remove and a group with one already absent target preserve
  truthful Unchanged/Mutated facts and do not create extra mutation journals.
- A changed managed package, foreign native object, changed project/request
  scope, missing/unsupported project capability, and stale binding after Prepare
  must refuse before deactivation/publication. Retain real pending-journal and
  PLUGIN_DATA refusal tests.
- A user-scope non-Claude/Codex zero-selected group is still rejected by the same
  historical guard. Selected-delivery tests retain their existing behavior.
- Integrate grouped remove into the full project lifecycle once this independent
  slice is approved; the handoff correction itself can finish its current cleanup
  through the already supported per-client Remove operations.

Use focused installer tests and changed-source/static architecture checks only.
Two independent plan rounds and two frozen code reviews are required. No
production group-removal source has been edited for this plan.
