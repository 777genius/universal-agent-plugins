# Explicit selected-profile Local adapter

`NewLocal(LocalConfig)` is independent of historical `New()`. Hosts explicitly
register one constructor in their registry. The default registry, CLI/shared
Copilot semantics, availability labels and public-beta metadata are unchanged.
This implementation uses the existing loader, selected-delivery record, stager,
native file lock/CAS kernel, `vscodeprofile` and `vscodelocalhooks`; it adds no engine.

The constructor freezes the explicit physical `settings.json`, source-qualified
TEST tuple, target shell, native Stop selection, typed fixed specs, declared hook
file digest, and sorted MCP/skill selections. Parent aliases normalize once;
final file symlinks and ambiguous/invalid JSONC fail closed. No ambient HOME,
default profile or Copilot CLI supplies Local authority. The sole TEST source
pin is VS Code 1.140.0 / built-in Copilot source
`07f806f999227108933c2e30515b26eecc1fda74`; it is not native release admission.

Before construction the host verifies its ledger-recorded primary runtime
artifact and supplies that fixed absolute executable in the Stop spec. Supply
exactly one `Stop` spec with timeout 5 and fixed args; only `${PLUGIN_ROOT}` and
`${PLUGIN_DATA}` may appear in args. Projection resolves them once from
`Plan.ActivePath` and `PluginDataPath`, then renders quoted native argv. The
expected `sha256:` digest binds the declared native hook bytes; their schema and
fixed command/spec identity must also match. UAP never chooses an AN runtime.

Native hooks live at `com.github.copilot/hooks/hooks.json`. Real staging removes
unsupported root hooks, retains unknown namespaces, omits unselected MCP/skills,
and records separate canonical and installed projection digests. Manual-only
projection omits Stop and may omit hook-qualified shell. Preparation has no
native effect. Selecting neither projects no desired profile object. The existing
lifecycle retains previous selector ownership across unchanged native effects;
matching native booleans alone grant no authority. Optional MCP uses the genuine existing managed-stdio helper;
constructor/preparation alone does not advertise helper readiness.

Registration owns only the selected `chat.pluginLocations[activePath]` boolean.
Public pure Plan/VerifyOwned preserve foreign JSONC, comments and native false.
Identical unowned booleans and nonboolean drift conflict. Exact-file native
writes use the existing writer lock, CAS, readback and rollback. Linux attributed
profiles refuse changed writes because this kernel cannot preserve ACL/xattrs.
Other OS writes refuse until metadata preservation and actual native CI qualify
that path; a Windows build would not constitute native proof. Inspection creates
no locks and reports owned active/disabled/missing/conflict without CLI listing.
Registration is `prepared` / package-valid; live activation remains unknown.

Recovery consumes the recorded mode/profile/shell/components/digests/selector
and attempt, ignoring current constructor/environment. The facade must validate
binding, complete pending scope and artifacts before calling the structurally
matched reconciler. Removal uses a confirmed recorded reverse decision and only
patches the owned selector; late foreign edits survive and retries are idempotent.
A direct adapter reconciliation test is not public Engine recovery proof.

## Exact 040 foundation limitations

An empty unowned route repeats without changing state or settings. An already
owned selector survives deselection and stays disabled, but 040 supplies no prior
receipt to either projector. Omitting the undesired entry then differs from
retained ownership and produces maintenance state churn. The strict owned-to-empty
consumer control records this unresolved carrier gap. Full corrective acceptance
requires the brokered confirmed-ownership input; current profile values cannot
substitute for it. No core carrier or selected authority is changed here.

Public Apply requires a helper even with no selected MCP. Public Remove omits
selected reverse authority. Inspect omits native pending attempts and Recover
has no selected reconciler input. These remain separately owned lifecycle work;
no unapproved repair source was copied. The real exit-91 child fixture is ready
for composition with the independently approved repair and full public recovery.

040 also cannot persist an observed native-false receipt separately from frozen
desired true. Until an approved receipt contract exists, missing formerly owned
true refuses additive repair rather than risking re-enabling a disappeared false.
A present false remains unchanged across repeat/update/refresh/repair.

Consumer tests live in the test-only `integrationtests/vscodelocal` package,
explicitly composing actual NewLocal, public Engine and real filesystem/process
boundaries. Adapter tests need no upward planner/provider or self imports; the
existing architecture/lint configuration remains unchanged. All original
filesystem, installed managed-stdio and exit-91 crash assertions are retained.
Independent root review is required before integration. Native VS Code/agent
execution, Windows CI and downstream AN qualification have not run. This
candidate does not qualify executable release or installed AN.
