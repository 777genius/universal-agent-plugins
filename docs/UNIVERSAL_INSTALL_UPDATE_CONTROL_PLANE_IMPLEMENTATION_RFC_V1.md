# Universal Install And Update Control Plane - Implementation RFC V1

Status: Draft v1

Date: 2026-04-08

This RFC turns the architecture in [UNIVERSAL_INSTALL_UPDATE_CONTROL_PLANE_PLAN.md](./UNIVERSAL_INSTALL_UPDATE_CONTROL_PLANE_PLAN.md) into a concrete implementation plan for `plugin-kit-ai`.

It is intentionally conservative:

- prefer documented vendor mutation surfaces
- keep unsupported automation out of adapters
- make state, evidence, and repair first-class from day one

## Scope

This RFC defines:

- package and module layout
- Go domain types and interfaces
- state, lock, journal, and evidence-registry schemas
- use-case boundaries
- adapter contract rules
- Phase 1 implementation order

This RFC does not define:

- final CLI UX copy
- every adapter implementation detail
- enterprise administration support beyond blocked-layer detection

## Fit With Current Repo

Current composition roots already separate facade, use case, and ports:

- [ARCHITECTURE.md](./ARCHITECTURE.md)
- [install.go](../install/plugininstall/install.go)
- [installer.go](../install/plugininstall/usecase/installer.go)

V1 should follow the same style:

- one public facade package
- internal use cases behind ports
- concrete adapter wiring only in the facade
- CLI calls the facade, not adapters directly

## Proposed Module

Recommended new module:

- `install/integrationctl`

Recommended Go module path:

- `github.com/777genius/plugin-kit-ai/install/integrationctl`

Reason:

- matches the existing `plugininstall` packaging style
- keeps lifecycle control separate from binary-release install logic
- allows isolated tests and dependencies

## Repository Strategy

V1 should stay inside the current repository as an in-repo module.

Why:

- the lifecycle engine still evolves together with authored manifest normalization, delivery generation, publishing policy, and compatibility logic in this repository
- splitting into a separate repository now would add release and version coordination overhead before the facade API has stabilized
- a monorepo module still gives a clean import boundary, isolated tests, and strict port or adapter separation

Boundary:

- `install/integrationctl` owns lifecycle domain types, use cases, ports, adapters, state, locks, journals, and evidence handling
- root `plugin-kit-ai` code owns authored-source validation, delivery generation, publishing, and top-level CLI composition
- bootstrap scripts and one-line install flows should call the module facade instead of reimplementing lifecycle logic

Future extraction to a separate repository is reasonable only after:

- the facade API is stable
- at least one external consumer needs the module directly
- manifest and delivery schemas have slowed down materially
- the installer release cadence diverges from the rest of the repository

## Package Layout

Recommended initial layout:

```text
install/integrationctl/
  go.mod
  integrationctl.go
  domain/
    ids.go
    enums.go
    errors.go
    manifest.go
    installation.go
    journal.go
    plan.go
    policy.go
  ports/
    adapters.go
    evidence.go
    filesystem.go
    journal.go
    lock.go
    manifest.go
    process.go
    source.go
    state.go
    time.go
  usecase/
    add.go
    update.go
    remove.go
    repair.go
    sync.go
    list.go
    doctor.go
    shared_plan.go
  adapters/
    fs/
    jsonstate/
    locks/
    manifest/
    source/
    evidence/
    claude/
    gemini/
    cursor/
    codex/
    opencode/
```

Public CLI wiring stays outside:

```text
cli/internal/app/
cli/cmd/plugin-kit-ai/
```

## Public Facade

`integrationctl.go` should be the only public composition root in the module.

Recommended surface:

```go
package integrationctl

import "context"

type AddParams struct {
    Source             string
    Targets            []string
    Scope              string
    AutoUpdate         *bool
    AdoptNewTargets    string
    AllowPrerelease    *bool
    DryRun             bool
}

type UpdateParams struct {
    Name               string
    All                bool
    DryRun             bool
}

type RemoveParams struct {
    Name               string
    DryRun             bool
}

type RepairParams struct {
    Name               string
    DryRun             bool
}

type SyncParams struct {
    DryRun             bool
}

type Result struct {
    OperationID string
    Summary     string
    Report      Report
}

func Add(ctx context.Context, p AddParams) (Result, error)
func Update(ctx context.Context, p UpdateParams) (Result, error)
func Remove(ctx context.Context, p RemoveParams) (Result, error)
func Repair(ctx context.Context, p RepairParams) (Result, error)
func Sync(ctx context.Context, p SyncParams) (Result, error)
func List(ctx context.Context) (Report, error)
func Doctor(ctx context.Context) (Report, error)
func ExitCodeFromErr(err error) int
```

Rule:

- facade methods return normalized reports
- vendor-native output never leaks directly as the public API

## Domain Model

### Core enums

Recommended enums:

```go
type TargetID string

const (
    TargetClaude   TargetID = "claude"
    TargetCodex    TargetID = "codex"
    TargetGemini   TargetID = "gemini"
    TargetCursor   TargetID = "cursor"
    TargetOpenCode TargetID = "opencode"
)

type DeliveryKind string

const (
    DeliveryClaudeMarketplace DeliveryKind = "claude-marketplace-plugin"
    DeliveryCodexMarketplace  DeliveryKind = "codex-marketplace-plugin"
    DeliveryGeminiExtension   DeliveryKind = "gemini-extension"
    DeliveryCursorMCP         DeliveryKind = "cursor-mcp"
    DeliveryOpenCodePlugin    DeliveryKind = "opencode-plugin"
)

type InstallState string

const (
    InstallPrepared          InstallState = "prepared"
    InstallInstalled         InstallState = "installed"
    InstallActivationPending InstallState = "activation_pending"
    InstallAuthPending       InstallState = "auth_pending"
    InstallDisabled          InstallState = "disabled"
    InstallDegraded          InstallState = "degraded"
    InstallRemoved           InstallState = "removed"
)

type ActivationState string

const (
    ActivationNotRequired   ActivationState = "not_required"
    ActivationNativePending ActivationState = "native_activation_pending"
    ActivationReloadPending ActivationState = "reload_pending"
    ActivationRestartPending ActivationState = "restart_pending"
    ActivationNewThreadPending ActivationState = "new_thread_pending"
    ActivationComplete      ActivationState = "complete"
)

type EnvironmentRestrictionCode string

const (
    RestrictionManagedPolicyBlock  EnvironmentRestrictionCode = "managed_policy_block"
    RestrictionTrustRequired       EnvironmentRestrictionCode = "trust_required"
    RestrictionSourceAuthRequired  EnvironmentRestrictionCode = "source_auth_required"
    RestrictionNativeAuthRequired  EnvironmentRestrictionCode = "native_auth_required"
    RestrictionNativeActivation    EnvironmentRestrictionCode = "native_activation_required"
    RestrictionRestartRequired     EnvironmentRestrictionCode = "restart_required"
    RestrictionReloadRequired      EnvironmentRestrictionCode = "reload_required"
    RestrictionNewThreadRequired   EnvironmentRestrictionCode = "new_thread_required"
    RestrictionSourceToolMissing   EnvironmentRestrictionCode = "source_tool_missing"
    RestrictionSourceShapeInvalid  EnvironmentRestrictionCode = "source_shape_unsupported"
    RestrictionReadOnlyNativeLayer EnvironmentRestrictionCode = "read_only_native_layer"
    RestrictionVolatileOverride    EnvironmentRestrictionCode = "volatile_override_layer"
)

type ProtectionClass string

const (
    ProtectionUserMutable   ProtectionClass = "user_mutable"
    ProtectionWorkspace     ProtectionClass = "workspace_mutable"
    ProtectionRemoteDefault ProtectionClass = "remote_default"
    ProtectionAdminManaged  ProtectionClass = "admin_managed"
)

type EvidenceClass string

const (
    EvidenceConfirmed  EvidenceClass = "confirmed_vendor_fact"
    EvidenceInference  EvidenceClass = "architectural_inference"
    EvidencePolicy     EvidenceClass = "project_policy"
)
```

### Core structs

Recommended minimum:

```go
type IntegrationRef struct {
    Raw string
}

type RequestedSourceRef struct {
    Kind  string `json:"kind"`
    Value string `json:"value"`
}

type ResolvedSourceRef struct {
    Kind  string `json:"kind"`
    Value string `json:"value"`
}

type IntegrationManifest struct {
    IntegrationID   string
    Version         string
    RequestedRef    RequestedSourceRef
    ResolvedRef     ResolvedSourceRef
    SourceDigest    string
    ManifestDigest  string
    Deliveries      []Delivery
    Migration       *MigrationHint
}

type Delivery struct {
    TargetID           TargetID
    DeliveryKind       DeliveryKind
    Name               string
    NativeRefHint      string
    CapabilitySurface  []string
}

type MigrationHint struct {
    Kind  string
    Value string
}

type CatalogPolicySnapshot struct {
    Installation string `json:"installation,omitempty"`
    Authentication string `json:"authentication,omitempty"`
    Category string `json:"category,omitempty"`
}

type InstallPolicy struct {
    Scope            string `json:"scope"`
    AutoUpdate       bool   `json:"auto_update"`
    AdoptNewTargets  string `json:"adopt_new_targets"`
    AllowPrerelease  bool   `json:"allow_prerelease"`
}

type NativeObjectRef struct {
    Kind            string          `json:"kind"`
    Name            string          `json:"name,omitempty"`
    Path            string          `json:"path,omitempty"`
    ProtectionClass ProtectionClass `json:"protection_class,omitempty"`
}

type TargetInstallation struct {
    TargetID                TargetID                    `json:"target_id"`
    DeliveryKind            DeliveryKind                `json:"delivery_kind"`
    State                   InstallState                `json:"state"`
    NativeRef               string                      `json:"native_ref,omitempty"`
    ActivationState         ActivationState             `json:"activation_state,omitempty"`
    InteractiveAuthState    string                      `json:"interactive_auth_state,omitempty"`
    EnvironmentRestrictions []EnvironmentRestrictionCode `json:"environment_restrictions,omitempty"`
    SourceAccessState       string                      `json:"source_access_state,omitempty"`
    OwnedNativeObjects      []NativeObjectRef           `json:"owned_native_objects,omitempty"`
    AdapterMetadata         map[string]any              `json:"adapter_metadata,omitempty"`
}

type InstallationRecord struct {
    IntegrationID      string                        `json:"integration_id"`
    RequestedSourceRef RequestedSourceRef            `json:"requested_source_ref"`
    ResolvedSourceRef  ResolvedSourceRef             `json:"resolved_source_ref"`
    ResolvedVersion    string                        `json:"resolved_version"`
    SourceDigest       string                        `json:"source_digest"`
    ManifestDigest     string                        `json:"manifest_digest"`
    Policy             InstallPolicy                 `json:"policy"`
    Targets            map[TargetID]TargetInstallation `json:"targets"`
    LastCheckedAt      string                        `json:"last_checked_at"`
    LastUpdatedAt      string                        `json:"last_updated_at"`
}
```

## Domain Errors

Recommended typed domain errors:

```go
type Code string

const (
    ErrUsage                 Code = "usage"
    ErrSourceResolve         Code = "source_resolve"
    ErrManifestLoad          Code = "manifest_load"
    ErrUnsupportedTarget     Code = "unsupported_target"
    ErrEnvironmentBlocked    Code = "environment_blocked"
    ErrStateConflict         Code = "state_conflict"
    ErrLockAcquire           Code = "lock_acquire"
    ErrActivationPending     Code = "activation_pending"
    ErrAuthPending           Code = "auth_pending"
    ErrMutationApply         Code = "mutation_apply"
    ErrRepairApply           Code = "repair_apply"
    ErrEvidenceViolation     Code = "evidence_violation"
)

type Error struct {
    Code    Code
    Message string
    Cause   error
}
```

Rule:

- errors must be portable across adapters
- vendor command stderr belongs in details, not in the error type itself

## Adapter Port

This is the core interface that everything else depends on.

Recommended contract:

```go
type TargetAdapter interface {
    ID() domain.TargetID
    Capabilities(context.Context) (Capabilities, error)
    Inspect(context.Context, InspectInput) (InspectResult, error)
    PlanInstall(context.Context, PlanInstallInput) (AdapterPlan, error)
    ApplyInstall(context.Context, ApplyInput) (ApplyResult, error)
    PlanUpdate(context.Context, PlanUpdateInput) (AdapterPlan, error)
    ApplyUpdate(context.Context, ApplyInput) (ApplyResult, error)
    PlanRemove(context.Context, PlanRemoveInput) (AdapterPlan, error)
    ApplyRemove(context.Context, ApplyInput) (ApplyResult, error)
    Repair(context.Context, RepairInput) (ApplyResult, error)
}
```

Recommended supporting types:

```go
type Capabilities struct {
    InstallMode               string
    SupportsNativeUpdate      bool
    SupportsNativeRemove      bool
    SupportsLinkMode          bool
    SupportsAutoUpdatePolicy  bool
    SupportsScopeUser         bool
    SupportsScopeProject      bool
    SupportsScopeLocal        bool
    SupportsRepair            bool
    RequiresRestart           bool
    RequiresReload            bool
    RequiresNewThread         bool
    MayTriggerInteractiveAuth bool
    SupportedSourceKinds      []string
    EvidenceKey               string
}

type PlanInstallInput struct {
    Manifest domain.IntegrationManifest
    Policy   domain.InstallPolicy
    Inspect  InspectResult
}

type PlanUpdateInput struct {
    CurrentRecord domain.InstallationRecord
    NextManifest  domain.IntegrationManifest
    Inspect       InspectResult
}

type PlanRemoveInput struct {
    Record  domain.InstallationRecord
    Inspect InspectResult
}

type RepairInput struct {
    Record  domain.InstallationRecord
    Inspect InspectResult
}

type InspectInput struct {
    Record *domain.InstallationRecord
    Scope  string
}

type InspectResult struct {
    TargetID                domain.TargetID
    Installed               bool
    State                   domain.InstallState
    ActivationState         domain.ActivationState
    InteractiveAuthState    string
    CatalogPolicy           *domain.CatalogPolicySnapshot
    ConfigPrecedenceContext []string
    EnvironmentRestrictions []domain.EnvironmentRestrictionCode
    VolatileOverrideDetected bool
    TrustResolutionSource   string
    SourceAccessState       string
    OwnedNativeObjects      []domain.NativeObjectRef
    ObservedNativeObjects   []domain.NativeObjectRef
    SettingsFiles           []string
    Warnings                []string
    EvidenceClass           domain.EvidenceClass
}

type AdapterPlan struct {
    TargetID           domain.TargetID
    ActionClass        string
    Summary            string
    Commands           []string
    PathsTouched       []string
    OwnedNativeObjects []domain.NativeObjectRef
    RestartRequired    bool
    ReloadRequired     bool
    NewThreadRequired  bool
    ManualSteps        []string
    Blocking           bool
    EvidenceKey        string
}

type ApplyInput struct {
    Plan       AdapterPlan
    Manifest   domain.IntegrationManifest
    Record     *domain.InstallationRecord
}

type ApplyResult struct {
    TargetID                domain.TargetID
    State                   domain.InstallState
    ActivationState         domain.ActivationState
    InteractiveAuthState    string
    OwnedNativeObjects      []domain.NativeObjectRef
    Warnings                []string
    ManualSteps             []string
    RestartRequired         bool
    ReloadRequired          bool
    NewThreadRequired       bool
    SourceAccessState       string
    EnvironmentRestrictions []domain.EnvironmentRestrictionCode
    VolatileOverrideDetected bool
    EvidenceClass           domain.EvidenceClass
}
```

Hard rules:

- `Inspect` never mutates
- `Plan*` never mutates
- `Apply*` mutates only owned native objects
- adapters never mutate admin-managed or ambiguous layers
- adapters must encode vendor-specific blockers through `EnvironmentRestrictionCode` values instead of free-form strings when the blocker affects control-flow
- adapters must treat environment variables, command-line overrides, and other volatile selection layers as observed state, not persistent mutation targets
- catalog policy such as marketplace defaults may inform plan generation, but `installed` and related lifecycle states must come from observed native state, not catalog metadata alone
- `Repair` should prefer vendor-documented isolate, disable, or owned-entry detachment steps before cache clearing, broad cleanup, or reinstall
- when patching shared native config, adapters should replace only adapter-managed keys on adapter-owned entries and preserve unmanaged fields
- whole-file overwrite is allowed only when the file is fully adapter-owned or the vendor-documented workflow explicitly requires replacement semantics

## Supporting Ports

Recommended ports:

```go
type SourceResolver interface {
    Resolve(context.Context, domain.IntegrationRef) (ResolvedSource, error)
}

type ManifestLoader interface {
    Load(context.Context, ResolvedSource) (domain.IntegrationManifest, error)
}

type StateStore interface {
    Load(context.Context) (StateFile, error)
    Save(context.Context, StateFile) error
}

type LockManager interface {
    Acquire(context.Context, string) (UnlockFunc, error)
}

type OperationJournal interface {
    Start(context.Context, OperationRecord) error
    AppendStep(context.Context, string, JournalStep) error
    Finish(context.Context, string, string) error
    ListOpen(context.Context) ([]OperationRecord, error)
}

type EvidenceRegistry interface {
    Get(context.Context, string) (EvidenceEntry, error)
}

type FileSystem interface {
    ReadFile(context.Context, string) ([]byte, error)
    WriteFileAtomic(context.Context, string, []byte, uint32) error
    MkdirAll(context.Context, string, uint32) error
    Stat(context.Context, string) (PathInfo, error)
}

type ProcessRunner interface {
    Run(context.Context, Command) (CommandResult, error)
}

type StateFile struct {
    SchemaVersion int                         `json:"schema_version"`
    Installations []domain.InstallationRecord `json:"installations"`
}

type OperationRecord struct {
    OperationID   string        `json:"operation_id"`
    Type          string        `json:"type"`
    IntegrationID string        `json:"integration_id"`
    Status        string        `json:"status"`
    StartedAt     string        `json:"started_at"`
    Steps         []JournalStep `json:"steps"`
}

type JournalStep struct {
    Target string `json:"target"`
    Action string `json:"action"`
    Status string `json:"status"`
}

type EvidenceEntry struct {
    Key           string   `json:"key"`
    Claim         string   `json:"claim"`
    EvidenceClass string   `json:"evidence_class"`
    URLs          []string `json:"urls"`
}

type ResolvedSource struct {
    Kind        string
    Requested   domain.RequestedSourceRef
    Resolved    domain.ResolvedSourceRef
    LocalPath   string
    SourceDigest string
    ImportRoots []string
    FailureClass string
}

type PathInfo struct {
    Exists bool
    IsDir  bool
}

type Command struct {
    Argv []string
    Env  []string
    Dir  string
}

type CommandResult struct {
    ExitCode int
    Stdout   []byte
    Stderr   []byte
}
```

## State Schema

Recommended state file:

- `~/.plugin-kit-ai/state.json`

Recommended JSON:

```json
{
  "schema_version": 1,
  "installations": [
    {
      "integration_id": "context7",
      "requested_source_ref": {
        "kind": "github_repo_path",
        "value": "777genius/universal-plugins-for-ai-agents//plugins/context7"
      },
      "resolved_source_ref": {
        "kind": "git_commit",
        "value": "https://github.com/777genius/universal-plugins-for-ai-agents@8f0f1d8"
      },
      "resolved_version": "1.4.0",
      "source_digest": "sha256:abc",
      "manifest_digest": "sha256:def",
      "policy": {
        "scope": "user",
        "auto_update": true,
        "adopt_new_targets": "auto",
        "allow_prerelease": false
      },
      "targets": {
        "claude": {
          "target_id": "claude",
          "delivery_kind": "claude-marketplace-plugin",
          "state": "installed",
          "native_ref": "context7@portable-mcp"
        },
        "codex": {
          "target_id": "codex",
          "delivery_kind": "codex-marketplace-plugin",
          "state": "activation_pending",
          "activation_state": "awaiting_plugin_browser_install"
        }
      },
      "last_checked_at": "2026-04-08T11:00:00Z",
      "last_updated_at": "2026-04-08T11:00:00Z"
    }
  ]
}
```

## Source Resolution Rules

V1 source resolution should be stricter than `git clone` plus path guessing.

Rules:

- resolvers should support local paths, GitHub-style repository references, and raw git URLs as distinct source kinds
- resolvers should return an immutable resolved reference when possible, such as a git commit
- resolvers should expose typed failure classes such as `auth`, `not_found`, `network`, and `tool_missing`
- resolvers may surface multiple candidate import roots when nested plugin roots are discoverable through marketplace metadata or similar index files
- import-root discovery belongs to source preparation, not to target mutation

## Mutation Safety Rules

V1 mutation primitives should be stricter than a plain `write file` helper.

Rules:

- atomic writes should prefer temp-file plus rename and allow explicit fallback behavior for filesystem edge cases such as cross-device or platform rename limitations
- backups complement journals; they do not replace journals
- every config-patching adapter should use field-aware merge semantics instead of one generic deep merge
- maps that model managed entry sets, such as MCP server maps, may require replacement semantics for the managed subset so removal stays correct
- post-write validation or re-inspection should happen before an operation is considered committed

## Lock Schema

Recommended workspace lock:

- `<repo>/.plugin-kit-ai.lock`

Recommended YAML:

```yaml
api_version: v1
integrations:
  - source: github:777genius/universal-plugins-for-ai-agents//plugins/context7
    version: 1.4.0
    targets:
      - claude
      - gemini
      - cursor
    policy:
      scope: project
      auto_update: true
      adopt_new_targets: manual
      allow_prerelease: false
```

Rule:

- lock describes desired workspace intent
- state describes actual local machine state
- lock must not store secrets, approval decisions, auth tokens, or machine-specific absolute paths
- state may record references to user-owned settings files or native objects, but should not duplicate secret values out of vendor-owned storage

## Operation Journal Schema

Recommended path:

- `~/.plugin-kit-ai/operations/<operation-id>.json`

Recommended JSON:

```json
{
  "operation_id": "op_2026_04_08_001",
  "type": "add",
  "integration_id": "context7",
  "status": "in_progress",
  "started_at": "2026-04-08T11:00:00Z",
  "steps": [
    {
      "target": "claude",
      "action": "install_missing",
      "status": "applied"
    },
    {
      "target": "codex",
      "action": "await_activation",
      "status": "awaiting_user_activation"
    }
  ]
}
```

Rule:

- journal starts before first mutation
- journal finishes only after state commit or explicit degraded conclusion

## Evidence Registry Schema

Recommended path:

- `docs/generated/integrationctl_evidence_registry.json`

Recommended JSON:

```json
{
  "schema_version": 1,
  "entries": [
    {
      "key": "claude.marketplace.autoupdate.startup",
      "claim": "Claude can auto-update marketplaces and installed plugins at startup.",
      "evidence_class": "confirmed_vendor_fact",
      "urls": [
        "https://code.claude.com/docs/en/discover-plugins"
      ]
    },
    {
      "key": "codex.install.browser.only",
      "claim": "Codex docs clearly document plugin-browser installation but do not clearly document a standalone non-interactive install command.",
      "evidence_class": "architectural_inference",
      "urls": [
        "https://developers.openai.com/codex/plugins",
        "https://developers.openai.com/codex/plugins/build"
      ]
    }
  ]
}
```

Rule:

- non-internal adapter capabilities must carry an `evidence_key`
- tests fail if the key is missing or references the wrong evidence class

## Use Cases

### Add

Recommended flow:

1. resolve source
2. load manifest
3. inspect current state
4. inspect each selected target
5. build integration plan
6. create operation journal
7. acquire lock
8. apply adapter installs
9. write updated state
10. finish journal

### Update

Recommended flow:

1. load record
2. resolve latest allowed source
3. load new manifest
4. inspect current targets
5. compute reconcile actions
6. apply updates
7. adopt new targets if policy permits
8. persist state and journal

### Repair

Recommended flow:

1. inspect state
2. inspect native layers
3. inspect open journals
4. classify issue:
   - drift
   - activation pending
   - auth pending
   - source unreachable
   - corruption
5. repair minimally

## Planning Engine

Recommended shared internal abstraction:

```go
type IntegrationPlan struct {
    OperationID string
    Actions     []TargetAction
    Blocking    bool
    Warnings    []string
}

type TargetAction struct {
    TargetID     domain.TargetID
    ActionClass  string
    AdapterPlan  ports.AdapterPlan
}
```

Action classes for V1:

- `install_missing`
- `update_version`
- `adopt_new_target`
- `migrate_source`
- `repair_drift`
- `remove_orphaned_target`
- `await_activation`
- `await_auth_completion`
- `noop`

## Adapters In Phase 1

### Claude

V1 should implement:

- inspect marketplace and installed-plugin state
- plan and apply add/update/remove via documented commands
- scope-aware state in `.claude/settings.json`
- seed-managed and `strictKnownMarketplaces` blockers

### Gemini

V1 should implement:

- install, update, uninstall, enable, disable, link
- user-owned settings preservation
- trusted-folder restrictions as environment blockers

### Cursor

V1 should implement:

- inspect effective config layer
- patch owned MCP entries only
- login state surfaced but not automated beyond documented surfaces

### Codex

V1 should implement:

- marketplace preparation
- cache and config inspection
- disable-state inspection
- `activation_pending` output when plugin-browser activation is still needed

### OpenCode

V1 should implement:

- config and plugin-directory projection
- precedence-aware inspect
- JSONC-safe patching
- managed-layer blocking and remote-layer non-ownership

## Report Contract

Recommended normalized report:

```go
type Report struct {
    OperationID string         `json:"operation_id,omitempty"`
    Summary     string         `json:"summary"`
    Targets     []TargetReport `json:"targets"`
    Warnings    []string       `json:"warnings,omitempty"`
}

type TargetReport struct {
    TargetID                string   `json:"target"`
    DeliveryKind            string   `json:"delivery_kind"`
    ActionClass             string   `json:"action_class"`
    State                   string   `json:"state"`
    ActivationState         string   `json:"activation_state,omitempty"`
    InteractiveAuthState    string   `json:"interactive_auth_state,omitempty"`
    RestartRequired         bool     `json:"restart_required,omitempty"`
    ReloadRequired          bool     `json:"reload_required,omitempty"`
    NewThreadRequired       bool     `json:"new_thread_required,omitempty"`
    EnvironmentRestrictions []string `json:"environment_restrictions,omitempty"`
    VolatileOverrideDetected bool    `json:"volatile_override_detected,omitempty"`
    TrustResolutionSource   string   `json:"trust_resolution_source,omitempty"`
    SourceAccessState       string   `json:"source_access_state,omitempty"`
    EvidenceKey             string   `json:"evidence_key,omitempty"`
    ManualSteps             []string `json:"manual_steps,omitempty"`
}
```

## Phase 1 Breakdown

Recommended order:

1. Create module skeleton and public facade.
2. Implement domain enums, errors, and core structs.
3. Implement JSON state store.
4. Implement lock manager.
5. Implement operation journal.
6. Implement evidence registry loader and tests.
7. Implement shared planning engine.
8. Implement `list`, `doctor`, and dry-run `add`.
9. Implement Claude adapter.
10. Implement Gemini adapter.
11. Implement Cursor adapter.
12. Add adapter contract tests, including restriction and activation normalization.

## Tests

Required V1 coverage:

- state atomic write tests
- lock contention tests
- journal recovery tests
- evidence-key contract tests
- plan diff tests
- fixture tests for:
  - new target added in later version
  - activation pending
  - auth pending
  - source unreachable with last-known-good still valid
  - managed-layer mutation refusal

## Deliverables For First PR

Recommended first PR scope only:

- module skeleton
- domain model
- JSON schemas as docs fixtures
- evidence registry
- state, lock, and journal adapters
- `list` and dry-run `add`

Reason:

- this yields a testable base without prematurely hard-coding vendor behavior

## Final Rule

V1 should be shippable only if it can represent:

- installed
- activation pending
- auth pending
- degraded
- blocked by managed policy
- blocked by source access

without lying about success and without mutating undocumented vendor surfaces.
