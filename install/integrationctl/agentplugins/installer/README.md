# UAP installer SDK

Public process-local installer API for a standard local Agent Plugins package
(`plugin.json` + MCP/skills). Constructor, inspect, Recover, and prepare/apply
for **install**, **update**, **repair**, **refresh_projection**, and **remove** are published. One
client uses `Request.ClientID`. Two explicitly registered clients use
`Request.Targets` for install/update/repair with the same operation verb.
OpenCode install/update/repair groups return `ErrUnsupported` before preparation:
per-target native host snapshots are not part of the group handle yet. Single-target
OpenCode keeps its explicit Prepare/Apply host authority contract.
Historical group removal retains its Claude/Codex boundary. Selected removal
requires the explicitly registered adapter's typed native-intent reconciler. `install-group` as an operation name is invalid.
Two PackageRoot values in one install or update group stay unpublished.
Repair of mixed live revisions uses per-target PackageRoot so each binding
keeps its exact recorded digest. Group Repair with one PackageRoot across
mixed live revisions is `ErrUpdateRequired` before effects.

Update and repair of a missing owned binding return `ErrNotInstalled` before
mutation. Repair rematerializes a missing target of a live binding.
Repair of one live binding uses that binding's recorded package, not the
installation's latest Source.TreeDigest, so an older sibling can be repaired
after a subset update. Repair of that same binding with a different digest
stays `ErrUpdateRequired`. `InspectedBinding.TreeDigest` is that binding's
recorded package digest; mixed live revisions are not collapsed to
`InspectedInstallation.TreeDigest`. Apply `ClientResult.TreeDigest` is that
same per-binding digest, including unchanged mixed group Repair.
`OnCommittedBinding` receives the same per-binding digest, not the first
group envelope. `ProjectArgs` BindingFacts use the envelope digest of the
client being staged.

Single-target `refresh_projection` uses the same Prepare/Apply confirmation
flow as repair, but requires the managed package to be intact. It restages the
exact recorded package revision with current host `ProjectArgs` and the owned
PLUGIN_DATA locator. Identical output returns `OutcomeUnchanged`; changed
output commits a new managed projection digest. A damaged package needs repair
first. `OnCommittedBinding` observes the committed projection before client
activation. If that handoff fails, a repeat with the same package and args
resumes the pending lifecycle without another directory receipt. Group
projection refresh is not supported.

`Engine.SupportsClient` and `Discover` preserve explicit SDK registry membership,
including existing VS Code APIs. Saved dependency admission uses the separate
`ClientSupported` policy: eleven qualified local adapters, with hostless ChatGPT
preparation and owner-deferred VS Code excluded from that workflow. Existing SDK
and imperative capabilities remain available. Registration alone never claims
automatic activation or support for every component. `ClientMetadata.Capabilities` reports
the existing domain capability contract. `Plan.Delivery.Components` carries actual
package support decisions, including unsupported components in a mixed package.
A package with no supported components is refused before installed effects.

CLI-registry adapters require `ClientExecutable` on install; OpenCode also needs
it for bounded host preparation. Native config clients Gemini, Cline, Windsurf
and Kimi need only an explicit profile. Any supplied executable, including known
sibling target facts, must be an absolute clean path. Existing group and historical
removal boundaries above are unchanged; broader single-target qualification does
not expand the separately qualified local-file CLI route.

## Contract

```go
engine, err := uapinstaller.New(uapinstaller.Config{StateRoot: stateRoot, HelperExecutable: helper})
prepared, err := engine.Prepare(ctx, uapinstaller.Request{Operation: uapinstaller.OpInstall, ...})
defer prepared.Close()
result, err := engine.Apply(ctx, prepared, uapinstaller.Decision{Confirmed: true})
```

`New` validates paths and does not create directories, open a journal, or run a
helper. ClientConfigRoot is an explicit request path; CODEX_HOME,
CLAUDE_CONFIG_DIR, and HOME are not read as profile defaults. Directories are
created on Recover or a confirmed Apply. Host seams may
replace args of one declared MCP server and observe a committed binding before
client activation. Optional `Config.Assess` and `Request.Assessment` are
digest-bound: block and unavailable never become allow.
`Config.TrustedLocalPackages` is the explicit policy for pre-authorized
bundled/local bytes; no evaluator or decision is an error when that policy is
false. Mixed group Repair assesses each distinct
snapshot digest once; blocking any snapshot refuses Prepare without mutation.
Same-root group Repair assesses that snapshot once. `New`, Inspect, Discover,
and Recover do not invoke Assess. Coarse `Config.Progress` phases are
observational. `Config.IndependentActivations` explicitly enables concurrent
external activation for distinct Claude/Codex user targets without selected
native delivery. Other targets and the default configuration retain sequential
activation. Preparation, host callbacks, state writes and result publication
remain coordinated. Per-client `ProgressEvent.Client` checkpoints report
`external_started`, `external_completed` or `external_failed`; failures expose
their finite stage rather than raw provider diagnostics.

Claude discovery and plugin listing have a 30-second budget, capped by the
caller deadline. A local listing timeout may leave another activation running
only after the runner proves that no child started or that the process and its
descendants closed. Parent cancellation and unconfirmed cleanup cancel and join
all dispatched work before returning. Qualification version pins do not change
these time budgets or become installed-client version restrictions.

The engine does not import Notifications types and does not
query a live Claude/Codex identity by default.

### Explicit state upgrades

An embedding installer should review the authoritative state after choosing its
state paths and before planning client registrations or starting install workers:

```go
migration, err := engine.PrepareStateMigration(ctx)
// Handle err before proceeding. For migration.Required, present SourceSchema,
// TargetSchema, Installations, and NeedsRebind and obtain host confirmation.
// Keep the complete plan unchanged across that confirmation.
result, err := engine.ApplyStateMigration(ctx, migration)
// Handle err before any client registration. result.BackupPath is also
// populated when a backup succeeded but committing the upgrade failed.
```

`PrepareStateMigration` validates supported schema 2, 3, and 4 state without
writing it or recovering old operations. Applying a required upgrade reuses the
explicit CLI migrator: it takes the same mutation lock as installation, checks
pending work and the exact reviewed digest again under the lock, writes a
byte-exact backup, then atomically publishes the validated current schema.
Old pending journals, receipts, and native intents require recovery using the
corresponding previous SDK or binary before migration. Current-schema pending
work remains the normal recovery flow's responsibility.

Missing and current state return `Required=false`; applying that plan rechecks
the observed state and returns `NoChange=true` without creating a backup or a
lock. Corrupt and future schemas are rejected. A changed review returns
`ErrPlanChanged`, including when the reviewed state was replaced by corrupt or
unsupported bytes. Plans bind the state root/file, mutation lock and operations
directory as well as the digest and may be serialized
between a host's confirmation process and its parent apply process. A new
review after successful migration is a no-op; replaying an old required review
is refused. Passing a canceled context never starts migration.

Read-only inspection remains compatible with old schemas. Mutation guards
report `ErrMigrationRequired`, with `MigrationRequiredError.SourceSchema` and
`TargetSchema` available through `errors.As`; hosts do not need to parse an
error message or edit state JSON.

For actionable refusal diagnostics, `errors.Is(err, ErrStateCorrupt)` identifies
invalid state records or JSON, `ErrRecoveryRequired` identifies old pending work,
and `ErrUnsupportedStateSchema` identifies an incompatible on-disk schema.
`errors.As` into `*UnsupportedStateSchemaError` exposes `SourceSchema` and
`TargetSchema`; a source greater than the target requires a newer SDK/binary.
I/O and namespace errors retain their original classification.

Prepare freezes the selected adapter's complete `SelectedDelivery` through the
public Plan/DeliveryPlan and each group PlanTarget. Apply regenerates that
adapter plan before effects, repeats all-target and recorded-binding checks after
preflight/stage observers before mutation locks or native discovery, and guards
the use case's subsequent replan too. Changed mode, physical/profile identity, tuple, components, digests or owned
selector authority returns `ErrPlanChanged`. The prepared installation identity
is reused for the commit. These typed fields use `json:"-"`: diagnostic JSON
cannot be replayed as confirmation authority. The existing digest-bound content
assessment remains a separate decision.

`BindingFacts` in `OnCommittedBinding`, and Result binding/client selections,
come from committed state; activation checks the matching committed selection,
including its staged projection digest. Empty selections retain historical
behavior. This is a public contract checkpoint with injected TEST adapters;
the default registry and actual NewLocal availability are unchanged. It grants
no native VS Code, Windows or macOS qualification.

Selected single/group removal freezes the complete persisted binding and data
receipt through Plan, BindingFacts, result and RemoveInput. It validates every
profile, target, managed digest and native scope before observers and repeats
validation after them, under the one service lock, and at native dispatch and
return. Constructor facts never replace removal authority. A historical adapter
without the typed reconciler and read-only native registry capability refuses a
persisted Local selection before commands. CLI registry inspection is ineligible
for selected removal; the registered adapter reads its exact persisted profile.

Inspect includes exact `Recovery.NativeIntents`: installation/binding/profile,
pending direction/attempt, complete selected tuple/components/selector and a
digest of the persisted binding. It acquires no mutation or profile lock. Recover
requires that exact typed observation and StateRoot, revalidates under one retained
process lock, and invokes the selected Service reconciler with persisted facts.
`Recovery.StateDigest` binds all state scopes; each complete journal is hashed.
Checks before and after every reconciler and before kernel reads/acknowledgement
reject new journals, changed receipts and foreign bindings. Only successful
existing kernel/service transitions advance the retained expected scope.
Journal recovery remains supported; stale/missing/foreign scopes and uncertain
attempts stay unresolved. Native intent diagnostic JSON exposes existing domain
facts, but a JSON round trip cannot restore the operational typed observation.

A complete selected Local plan without MCP needs no absent managed helper and
skips host MCP argument projection, including a package with no MCP server.
Selected MCP, historical plans and explicitly configured helper paths retain
helper validation. No Local adapter is automatically registered by this seam.

Prepare copies `Request` and reports canonical `Plan.TreeDigest` with algorithm
`agentplugins-tree-sha256-v1`. That value is the packagedigest source identity,
not the installed packagesnapshot ArtifactDigest. Scratch `TempRoot` must not
overlap the package source, including case, symlink, and Unicode NFC/NFD aliases
when the filesystem presents them as the same directory. `PackageRoot`
identifies the sealed bytes to read; optional `SourceRoot` identifies the stable
local source when a host creates a fresh snapshot for every invocation. After a terminal
remove, a later install of the missing client uses the explicit profile in the
new request; sibling bindings and PLUGIN_DATA stay. An existing record whose
`TreeDigest` does not match the snapshot, including an old-bridge artifact
digest in that field, returns `ErrUpdateRequired` without rewriting state.
Remove Prepare verifies the managed artifact and persisted target before any
client deactivation. Last-client remove retains PLUGIN_DATA and reports
`data_retained`. A later remove of that retained empty installation returns
`already_absent` without creating a journal, including when a sibling client is
still installed. `LocalPackageTreeDigest` reports that same canonical digest
without writing state. Retained metadata Update of a different digest is
`SwitchRetained`; a later Add is a separate Install. Plan of that Update is
metadata-only. SwitchRetained Progress is prepare/preflight/commit/complete
and does not report stage, activate, or verify. Cancelled SwitchRetained
reports no Progress. RequiredComponents do not apply to SwitchRetained;
completeness is checked on the following Install. A blocking Assess refuses
SwitchRetained without rewriting retained source. A different plugin.json
name is `package_identity` before mutation. A source already bound to
another installation is `source_collision`. Successful SwitchRetained
reports `data_retained` and a `data_compatibility` next action. An
ambiguous Save of that metadata is retried; a remaining Save error is
not `unchanged` even if Inspect already shows the desired digest. Plan of Install onto
retained r1 with package r2 is `ErrUpdateRequired` and shows both phases
before confirmation. Plan includes the helper protocol
version and SHA-256 of the helper bytes; UAP managedstdio stores the same
digest. Result.NextActions cover recover, update, reprepare, and activate after
a managed commit whose client activation did not finish. Cancel before the first
durable effect returns cancelled and writes no state. A host callback or context
cancel after that commit returns incomplete with the binding retained; it is not
a bool and does not roll the managed package back. Group retry reconciles each
pending committed binding before the no-change shortcut and before client
VerifyOnly; Inspect does not invoke that callback. Client.Materialization and
Client.Activation stay separate fields.

Confirmed Apply re-reads live target/ownership before mutation and returns
`plan_changed` instead of applying a stale confirmation. Discover reports
registered local-client user-scope metadata and component capabilities, executable presence, and current
bindings without creating state or executing found files. Inspect after a group
install reports both live bindings and TreeDigest without mutating state or
running a helper; Recover of that clean observation is `already_recovered`.
Recover of a stale observation whose journals vanished still requires leftover
swap artifacts to be absent; leftover `.agentplugins-staging-*` or
`.agentplugins-backup-*` beside the recorded target is `incomplete_recovery`,
not success.
Inspect and Result expose per-client materialization/activation/authentication/verification and
required components. The external sample's flagged path runs install → inspect
→ recover → repeat → update → repair → remove → SwitchRetained → reinstall. Passing `-claude-config`
uses `Request.Targets` for the same verbs on Claude+Codex together.

Codex artifact removal requires `Request.ExternalUninstalled`. Confirmed Apply
does not invent that attestation. A required missing helper or relative configured helper is rejected
before the state file is written. Confirmed Apply returns `recovery_required`
when Inspect sees a pending journal, unfinished receipt or native intent; it does not recover
as a side effect of install/remove. Close during Apply returns `ErrHandleBusy`
without releasing the sealed snapshot. Install of a different TreeDigest for an
active binding returns `ErrUpdateRequired` before mutation. Update of one live
client CompatibilityChecks remaining live siblings; missing sibling profile
data returns `ErrCompatibilityUnavailable` before mutation.

## External sample

`example/` is a separate Go module. It does not use `replace` or import
`internal`. From that directory:

```sh
GOWORK=off go get github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins@<commit>
GOWORK=off go run .
```

Or clone `example/`, run `GOWORK=off go get` of the same package path, then
`GOWORK=off go run .`. A successful import prints `external import ok` without
creating state. Pass `-package`, `-state`, `-config`, `-helper`, and
`-client-exe` to run install → inspect → recover → repeat → update → repair → remove → SwitchRetained → reinstall.
Inspect prints `inspect-bindings` for the live clients. Add `-claude-config` for the published Claude+Codex group path.

The public import path is
`github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer`.
