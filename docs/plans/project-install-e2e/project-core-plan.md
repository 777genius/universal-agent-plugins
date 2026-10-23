# Project-scope core: research and implementation plan

Status: two independent plan review/fix rounds complete; awaiting parent
implementation authorization. Initial research base
`2bf6c3d703640ffbecc14346996709b4aec36ddc`; contracts and hardcoded scope paths
rechecked at implementation base `3ea24e3014ddf6679ca86729c56f1069112b0d3a`.
Assigned worktree: `/workspace/scratch/2819fbb48645/work/uap-project`, branch
`feat/project-install-e2e`. This plan requires two independent
plan review/fix rounds and parent authorization before code changes.

## Governing scope

Read `AGENTS.md`, `docs/AUTHORING_CAPABILITY_PRESERVATION.md`, the owner decisions
at the start of `docs/STANDARD_FIRST_AUTHORING_ENGINE_IMPLEMENTATION_PLAN.md`,
`docs/plans/uap-unified-dependency-manifest.md`, and
`docs/plans/uap-all-agent-dependencies.md`. Preserve useful legacy source and
tests, the one standard engine, current source/release distinctions, and immutable
historical release evidence. The parent's current owner instruction supersedes
old hosted-model requirements. Parent confirmed the owner's request for all
supported agents narrowly reopens VS Code for this project-scope work. Preserve
its unrelated behavior and freeze; the VS Code client lane owns its adapter,
dedicated tests and qualification. Core may add shared scope/context contracts.

Deliver explicit project scope for the existing dependency lifecycle, then
qualify each client in a separate reviewed stack. Reuse the current TOML decoder,
graph solver, lock schema, immutable store, digest/security gates, state receipts,
and filesystem/native transactions. No new solver, sandbox, automatic host trust,
HOME substitution, path dependency language, or publication belongs to core.

## Research findings

* `cli/internal/agentpluginscli/root.go` rejects project scope after recognizing it.
  `deps_commands.go` separately requires user scope. Saved add, status, sync,
  repair, remove and transfer paths use `ScopeUser` and `EnvironmentUserDefault`.
* `installer.Request` has an explicit user `ClientConfigRoot`, but no scope or
  project root. Prepare, confirmation, group and selected lifecycle code hardcode
  user scope; selection and recovery must be changed together.
* `domain.PlanRequest.Scope`, binding scope, `ComputeClientBindingID` and data
  receipt scope already exist. `clients.TargetLayout.TargetRoot` has no scope or
  project context. `planner.ResolveTarget` checks scope but resolves a user path.
* Native staging/activation already receives the complete `DeliveryPlan`. This
  is the appropriate place to carry an explicit project root without changing
  the meaning of detected user profiles or global process environment.
* `environment.Coordinator` and `Observe` hardcode `user-default`; these are
  environment ownership policy, not solver or lock-format requirements.
* `cli/cmd/agentplugins/compose.go` currently composes one user namespace. Most
  paths already derive from that namespace, while `EnvironmentStoreRoot` is
  independently configurable. This supports isolated project mutable state with
  the existing immutable store shared across scopes.

## Contract and storage decisions

1. `--scope` remains `user` by default. Add `--project-root PATH`, valid only with
   `--scope project`. Project scope defaults the root to the invocation working
   directory; there is no implicit Git-root or parent-manifest search. The CLI
   canonicalizes an existing directory once before effects. SDK inputs require
   an explicit absolute clean project root. Refuse filesystem root, unsupported
   scopes, conflicting context, and unsafe aliases before acquisition/mutation.
2. Project intent lives at `<project>/uap.toml` and `<project>/uap.lock`, with the
   existing schema and semantics. Both are portable. Absolute project/profile/
   cache/state paths never enter manifest intent, canonical source identity,
   lock bytes, or package pins. A package's own TOML keeps its existing package
   meaning when acquired; the selected root operation determines environment
   meaning.
3. The local mutable namespace is `<project>/.uap/`: existing `state-v2.json`,
   `mutation.lock`, `environment.lock`, operations/journals, managed projections,
   installer temporary files, and retained `plugin-data` all derive from it.
   Use `project-default` for receipts within this isolated namespace; keep
   `user-default` unchanged in the current user namespace. No global project
   registry or path-derived portable environment identifier is needed.
   Before reading/creating mutation authority, use the existing path-policy
   primitives to check `.uap/` itself and every existing owned descendant
   without following symlinks, junctions or reparse aliases. Reject namespace
   aliases/ancestor escapes such as `A/.uap -> B/.uap`; string-prefix containment
   is insufficient. Bind to the selected canonical project directory and
   revalidate existing namespace identity immediately before apply/recovery.
   Do not add a global registry or new persisted portable directory identity.
4. Share the existing user `store` through `EnvironmentStoreRoot`; acquisition
   and feed caches retain their current user-root policy. Their content is not
   installed ownership. Project removal never deletes immutable store content,
   another environment's receipts, or retained data by default. Document `.uap/`
   as local generated state; do not silently edit an existing `.gitignore`.
5. An unchanged user operation preserves current paths, receipt identities,
   authority and output behavior. A, B and user have independent mutation and
   environment locks. Shared content-store concurrency keeps its existing
   digest-verified publication rules.

## Stable client interface (core owns these additions)

Add this operational, nonportable DTO in `domain/target_context.go`:

```go
type TargetContext struct {
    Scope          InstallScope `json:"-"`
    ProjectRoot    string       `json:"-"` // empty for user; canonical for project
    UserConfigRoot string       `json:"-"` // original user profile; never HOME
}
```

`UserConfigRoot` always equals the original selected `DetectedClient.ConfigRoot`.
It may be empty only when the client's existing user contract permits that.
It is never rewritten to a project directory. Context is excluded from public
JSON and portable formats. Add `TargetContext` to `clients.PlanInput` and
`domain.DeliveryPlan` and `domain.DeactivationRequest`; preserve existing public
`DeliveryPlan.Scope` and require them to agree. Also carry context through
read-only binding facts/target facts that construct lifecycle requests. Add
`ProjectRoot` to `domain.PlanRequest`. An internal helper
validates/constructs context from request scope, project root, and selected client.
All added containing `TargetContext` fields and operational `ProjectRoot` fields
also carry explicit `json:"-"` tags, or stay behind current dedicated private
projections. Existing status fields may report selected scope and operational
paths intentionally; the new DTO does not widen public JSON by default.

Keep the current user-only `clients.TargetLayout` interface unchanged. Add:

```go
type ProjectTargetLayout interface {
    ProjectTargetRoot(client domain.DetectedClient, mode domain.PackageMode,
        managedRoot string, target domain.TargetContext) (anchor, root string, err error)
}
```

The planner calls this interface only for project scope; the entry-boundary
capability gate below rejects missing project hooks before acquisition. There
is no user-layout fallback for
project scope. Project adapters implement this method and may use existing plan
hooks, with `PlanInput.TargetContext`, to select native registry/config paths and
truthful component support. Core does not choose host-specific directories.

Project adapters must also implement the existing `NativeRegistryLayout`, using
`PlanInput.TargetContext` to choose the exact project registry/config root.
The user-profile default is forbidden for project plans. Core validates the
selected native root and supplies it consistently to activation, verification
and deactivation. Add `NativeRegistryRoot` to `DeactivationRequest`; reconstruct
it from the configured namespace plus exact recorded binding and compare the
current deterministic layout before dispatch. Adapter mutation code must use
this explicit native registry rather than the preserved user's ConfigRoot.
Registry structural authority, when required by the adapter, is captured and
revalidated for that actual native registry independently of user detection.
For adapters already opting into durable physical authority, add one optional
`clients.ProjectPhysicalAuthority` hook in `clients/detection.go`:

```go
CaptureProjectAuthority(ctx context.Context, client domain.DetectedClient,
    target domain.TargetContext, nativeRegistryRoot string) (domain.ProfileAuthority, error)
```

It returns the existing token; reuse existing `RevalidateProfileAuthority` after
checking token root, scope, context and namespace agreement. Preserve current
user capture/revalidation APIs. Never manufacture a DetectedClient whose
ConfigRoot is the project. Pass the actual native project root to existing
`ValidateBindingProfile(root,binding)` and let the qualified client lane adapt
that validator only where its current policy assumes user scope. Adapters that
do not use durable physical authority retain exact native-object/path ownership
checks; project scope does not automatically opt every adapter into a new token.

Current static domain capabilities list user scope only. Each qualified client
stack owns adding `ScopeProject` to that client's declarative scopes, alongside
both project layout and native registry hooks. Add one shared read-only
`installer.ClientScopeSupported(registry, id, scope)` policy: user preserves the
current saved-client support policy; project additionally requires the declared
scope and both registered hooks. Planner and Discover use the same declaration
and hook requirements, without a second capability registry. The CLI invokes
this selected-scope check before source acquisition in add and saved dependency
entry paths (including every selected manifest target), and installer Prepare
checks it before loading/sealing local package bytes. Package/component-specific
admission remains in the existing planner after loading. A counting-acquirer
test proves unsupported scope makes zero acquisition calls or installed effects.
Core does not pre-advertise any unqualified client or remote-only ChatGPT.

Client R2 identified shared policy consumers beyond layout. Add declarative
`ClientDefinition.ProjectPolicy` (lifecycle kind and physical backend family)
and `SelectedDelivery.EffectiveTraitsForScope`, plus scope-aware backend family/
sibling/equality helpers. Qualified client stacks fill only their policy row.
Direct project components use the existing native-config attempt/recovery
policy; real project CLI plugin delivery retains CLI behavior. User helper
behavior remains unchanged. No project policy falls back to user traits or a
historical TEST selected mode. Distinct project Copilot/VS Code registries do
not share ownership merely because user backend families match. Core updates
native kernel admission, attempts, readiness/projection, profile checks,
repair/remove/recovery, group selection and data consumers once. Cover an
interrupted CLI-user/native-project edit and independent project sibling MCP
entries, with a user-sharing regression.

Use existing activation preflight to reject a positive completion flag that
contradicts known blocked/disabled project evidence. Cline additionally rejects
durable completion of session-only project MCP before the generic shortcut.
No new attestation framework is needed. ChatGPT has no native project scope:
both saved and `--no-save` project requests reject before acquisition, retaining
existing user/manual preparation. VS Code's qualified project product adapter
is selected only in the project App registry; user `vscode.New` and TEST
`NewLocal` remain unchanged. Scoped admission may qualify that project adapter
without changing the historical user saved-dependency deferral.

Add explicit `Scope` and `ProjectRoot` to `installer.Request`; empty scope retains
the established SDK user default. `ClientConfigRoot` retains its user-profile
meaning. Add `ProjectRoot` to `installer.Config` and `planner.Planner` so the
existing `DeliveryTargetResolver.ResolveTarget(ctx, client, scope, artifact)`
can reconstruct the same project target during repair, remove and recovery.
Project requests must match the engine's configured project root; a user engine
cannot accept a project request. All internally generated `PlanRequest`s carry
the selected scope/root. `Plan`, binding facts, inspect and recovery retain the
actual scope. Group requests, if used, select one common project root; existing
group client restrictions remain unchanged.

This deliberately keeps existing ports and user adapter interfaces stable.
No client should encode a project directory in `ClientConfigRoot`, rewrite HOME,
or infer the root from the active package path. A client may require separate
native config authority; it must capture/revalidate that authority using the
actual project path and the existing transaction boundaries.

## Ownership and implementation sequence

| Owner | Files and responsibilities |
| --- | --- |
| Core | `domain/target_context.go`, `domain/planning.go`, operational context/native-registry fields in `domain/clients.go`; `clients/planning.go`; planner context/layout selection and matching target resolution, including deactivation. |
| Core | `installer/{types,config,prepare,apply_confirm,group,selected_lifecycle,engine,target_facts,identity}*.go` as needed: eliminate hardcoded selected user scope, validate request/engine context, retain exact-binding recovery. Do not refactor unrelated lifecycle policy. |
| Core | `environment/{model,coordinator,observe}*.go`: add `Coordinator.EnvironmentID` with empty-to-user compatibility and `ObserveForEnvironment(view,id)`; keep `Observe(view)` as current-user wrapper. Scope every generated authority and receipt lookup. Solver/portable lock contract stays unchanged. |
| Core | `cli/cmd/agentplugins/compose.go`, `agentpluginscli/{app,root,deps_*,saved_*,install,add,update,repair,remove,list,info,outdated,doctor}*.go` only where selected runtime/scope is needed. Add scoped app composition; retain source acquisition, loaders and user-profile detection. |
| Client lane | Its `clients/<client>` adapter and dedicated fixtures/tests; adding that client's qualified `ScopeProject` declaration; narrowly shared rendering/native-config code only with core coordination; host docs/evidence; scope-aware skills/MCP plus supported native components. |
| Parent | Root review coordination, recording the narrow VS Code owner exception, integration/stack merges, final documentation/support artifact composition and publication. |
| Source lane | Generic/private native Git and archive transports, their additive decoder/lock/source/provenance/restore work, and separate top PR. Persisted local path dependency language is not requested. Core must not add source variants or second acquisition/solver paths. |

Implement core in small commits: (1) context/layout and focused unit coverage;
(2) isolated CLI/runtime composition and environment ownership; (3) lifecycle
scope propagation, exact recovery/removal and cohesive E2E. Expose project CLI
only for client scopes actually qualified by the adapter; core alone must not
claim all targets work. Each client starts implementation only after its own
two plan review/fix rounds. All commits/pushes/reviews are parent coordinated.

Scope selection must happen after Cobra parses flags and before command work.
Use one `App.ForScope(scope, projectRoot)` composition callback plus an
`app.forOptions(opts)` selector at command entry points. The callback reconstructs
the existing runtime with the selected mutable namespace and shared immutable
store; it is not a second engine. Command constructors continue receiving App
values. Do not mutate a copied App in a pre-run hook and assume existing RunE
closures observe that mutation. Tests exercise real production composition.
Audit every command reachable with project scope: no path may silently retain a
hardcoded user request. Any route not qualified in this delivery rejects project
scope before acquisition/effects. Existing signed Directory requests carry the
selected scope and continue enforcing signed scope eligibility; project support
does not override a user-only release policy or manufacture compatibility evidence.

`deps export/import` operates on the selected environment and preserves the
existing exact-byte/regular-file/race checks. When the canonical exchange pair
equals the active project pair (including default `.` from the project root),
refuse before writes and tell the caller to pass a different exchange directory.
Never silently read/write a user or parent manifest. Explicit exchange directory
transfer remains the same portable pair and locked restore contract.

## Lifecycle invariants

* Preflight unsupported targets/components and mismatched context before effects;
  existing digest/security and host detection rules still apply. Dry run neither
  creates `.uap/` nor changes project/user configs, intent, state, or journals.
* Desired-state observation, repair selection, manual-completion facts and saved
  aliases are scoped to selected namespace and environment ID. Never adopt an
  imperative or another environment's binding because its source/name matches.
* Native commands receive the actual project working directory or documented
  project flags from the client lane. Core does not automatically grant trust,
  approve MCP execution, or equate a prepared file with observed activation.
  Deactivation receives the same validated context/native registry even when
  invoked from an unrelated cwd. There is no path inference from the artifact.
* Sync remains unchanged for exact pins and converged effects. Update advances
  only selected moving requirements. Repair restores the recorded exact pin and
  retains data. Remove preserves foreign config entries and pending uninstall
  ownership; recovery remains explicit and bound to the selected namespace.
* Copied/moved `.uap/` state with old absolute locators must fail deterministic
  target/profile-namespace checks before mutation. Portable restore copies only
  manifest/lock and creates fresh receipts. No new automatic migration is added.
* An interrupted project operation cannot block or recover user/A/B operations
  in other mutable namespaces. Shared-cache corruption remains an integrity
  error and is never disguised as projection drift or repaired to a newer pin.

## Focused E2E and acceptance

Use disposable profiles, HOME/data roots, two projects A/B, real CLI composition,
local Git fixtures, the existing acquirer/loader/resolver/store and real transaction
receipts. Native process fixtures remain explicitly fixtures. The representative
adapter is chosen from the first independently qualified client stack.

1. Existing user saved lifecycle still passes with identical default paths and
   `user-default` ownership. Legacy SDK requests without scope stay user scoped.
2. Install the same package into user, A and B with different pins and aliases.
   Verify separate manifest/lock/state/data/projections and shared digest storage;
   no accidental user config changes during either project operation.
3. A locked no-op sync leaves intent, lock, owned config, state, data and journals
   byte-identical and makes no mutation/ref-refresh calls. Status reflects A only.
4. A targeted update does not affect B/user. Drift in A refuses sync; exact repair
   restores A's recorded digest and retains its data marker.
5. Copy A's manifest/lock to fresh C with an empty mutable namespace and reproduce
   exact package digests; no local paths appear in portable bytes. Copying A's
   mutable state to C instead fails safely before effects. Also export A to an
   explicit exchange directory and import/locked-restore C; source==destination
   default transfer refuses with unchanged input/state bytes. Marshal the new
   operational DTOs and portable pair and assert selected root/profile literals
   never enter public DTO output, lock bytes, intent digest or source identity.
6. Inject one filesystem interruption and one native-config interruption when
   applicable. Recover A through its own receipt/journal, preserving B/user.
7. Remove A's target intent and binding, preserving foreign config entries,
   B/user, shared cache and retained data. Keep ownership when external uninstall
   is incomplete; finish through the existing explicit attestation contract.
8. A generic contract fixture uses three different paths for the user profile,
   project native registry and managed package. It covers plan/apply/inspect/
   repair/remove, including removal invoked from an unrelated cwd; user profile
   bytes remain unchanged.
9. Negative coverage: missing/unsafe project root; project-root with user scope;
   request/engine mismatch; symlink escape; unsupported client/layout; mixed
   supported/unsupported selection; dry run with no created state; root selection
   from a nested cwd; two profiles and two same-named sibling projects. Include
   `A/.uap` as a symlink/junction to real B mutable state and assert no change to
   A, B or user bytes and no acquisition/native calls.

Run focused domain/planner/installer/environment tests, production composition
tests and the representative cohesive CLI scenario with race detection. Run the
expanded client matrix as stacks merge, then the parent's required repository
gates. Do not start duplicate broad jobs or claim live-native/runtime/release
qualification from fixture E2E. Stop optional testing once concrete risks and
required gates are covered.

## Review ledger

* Round 1: client-plans review at `project-core-review-r1.md`; fixed missing
  deactivation context/native root, explicit project NativeRegistryLayout,
  entry-boundary scope gates, and client-owned scope declarations. Incorporated
  the reviewer's follow-up minimal optional project authority capture and reuse
  of existing revalidation/validators; no fake detected user profile. Reviewer
  verified all findings closed and gave GO on 2026-10-23.
* Round 2: sources-lead review at `project-core-review-r2.md`; fixed physical
  no-follow namespace entry/revalidation gates, explicit private JSON fields,
  and project transfer source==destination refusal with explicit-exchange E2E.
  Reviewer verified all fixes and gave GO on 2026-10-23.
* Client R2: all 14 plans verified GO after scoped lifecycle/backend policy,
  preflight attestation, reachable ChatGPT unsupported, and product VS Code
  composition clarifications. These shared implementation requirements are
  recorded above and in `client-plans-review-r2.md`.
* Implementation: parent authorized after client R2 fix verification on
  2026-10-23; implementation starts with reviewable contract checkpoints.
