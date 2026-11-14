package domain

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
	DeliveryCursorPlugin      DeliveryKind = "cursor-plugin"
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
	ActivationNotRequired      ActivationState = "not_required"
	ActivationNativePending    ActivationState = "native_activation_pending"
	ActivationReloadPending    ActivationState = "reload_pending"
	ActivationRestartPending   ActivationState = "restart_pending"
	ActivationNewThreadPending ActivationState = "new_thread_pending"
	ActivationComplete         ActivationState = "complete"
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
	EvidenceConfirmed EvidenceClass = "confirmed_vendor_fact"
	EvidenceInference EvidenceClass = "architectural_inference"
	EvidencePolicy    EvidenceClass = "project_policy"
)

type RequestedSourceRef struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type ResolvedSourceRef struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type IntegrationRef struct {
	Raw string
}

type MigrationHint struct {
	Kind  string `json:"kind,omitempty"`
	Value string `json:"value,omitempty"`
}

type Delivery struct {
	TargetID          TargetID     `json:"target_id"`
	DeliveryKind      DeliveryKind `json:"delivery_kind"`
	Name              string       `json:"name"`
	NativeRefHint     string       `json:"native_ref_hint,omitempty"`
	CapabilitySurface []string     `json:"capability_surface,omitempty"`
}

type IntegrationManifest struct {
	IntegrationID  string             `json:"integration_id"`
	Version        string             `json:"version"`
	Description    string             `json:"description,omitempty"`
	RequestedRef   RequestedSourceRef `json:"requested_ref"`
	ResolvedRef    ResolvedSourceRef  `json:"resolved_ref"`
	SourceDigest   string             `json:"source_digest"`
	ManifestDigest string             `json:"manifest_digest"`
	Deliveries     []Delivery         `json:"deliveries"`
	Migration      *MigrationHint     `json:"migration,omitempty"`
}

type CatalogPolicySnapshot struct {
	Installation   string `json:"installation,omitempty"`
	Authentication string `json:"authentication,omitempty"`
	Category       string `json:"category,omitempty"`
}

type InstallPolicy struct {
	Scope           string `json:"scope" yaml:"scope,omitempty"`
	AutoUpdate      bool   `json:"auto_update" yaml:"auto_update,omitempty"`
	AdoptNewTargets string `json:"adopt_new_targets" yaml:"adopt_new_targets,omitempty"`
	AllowPrerelease bool   `json:"allow_prerelease" yaml:"allow_prerelease,omitempty"`
}

type WorkspaceLock struct {
	APIVersion   string                     `json:"api_version" yaml:"api_version"`
	Integrations []WorkspaceLockIntegration `json:"integrations" yaml:"integrations"`
}

type WorkspaceLockIntegration struct {
	Source  string        `json:"source" yaml:"source"`
	Version string        `json:"version,omitempty" yaml:"version,omitempty"`
	Targets []string      `json:"targets,omitempty" yaml:"targets,omitempty"`
	Policy  InstallPolicy `json:"policy,omitempty" yaml:"policy,omitempty"`
}

type NativeObjectRef struct {
	Kind            string          `json:"kind"`
	Name            string          `json:"name,omitempty"`
	Path            string          `json:"path,omitempty"`
	ProtectionClass ProtectionClass `json:"protection_class,omitempty"`
}

type TargetInstallation struct {
	TargetID                TargetID                     `json:"target_id"`
	DeliveryKind            DeliveryKind                 `json:"delivery_kind"`
	CapabilitySurface       []string                     `json:"capability_surface,omitempty"`
	State                   InstallState                 `json:"state"`
	NativeRef               string                       `json:"native_ref,omitempty"`
	ActivationState         ActivationState              `json:"activation_state,omitempty"`
	InteractiveAuthState    string                       `json:"interactive_auth_state,omitempty"`
	CatalogPolicy           *CatalogPolicySnapshot       `json:"catalog_policy,omitempty"`
	EnvironmentRestrictions []EnvironmentRestrictionCode `json:"environment_restrictions,omitempty"`
	SourceAccessState       string                       `json:"source_access_state,omitempty"`
	OwnedNativeObjects      []NativeObjectRef            `json:"owned_native_objects,omitempty"`
	AdapterMetadata         map[string]any               `json:"adapter_metadata,omitempty"`
}

type InstallationRecord struct {
	IntegrationID      string                          `json:"integration_id"`
	RequestedSourceRef RequestedSourceRef              `json:"requested_source_ref"`
	ResolvedSourceRef  ResolvedSourceRef               `json:"resolved_source_ref"`
	ResolvedVersion    string                          `json:"resolved_version"`
	SourceDigest       string                          `json:"source_digest"`
	ManifestDigest     string                          `json:"manifest_digest"`
	Policy             InstallPolicy                   `json:"policy"`
	WorkspaceRoot      string                          `json:"workspace_root,omitempty"`
	Targets            map[TargetID]TargetInstallation `json:"targets"`
	LastCheckedAt      string                          `json:"last_checked_at"`
	LastUpdatedAt      string                          `json:"last_updated_at"`
}

type JournalStep struct {
	Target string `json:"target"`
	Action string `json:"action"`
	Status string `json:"status"`
}

type OperationRecord struct {
	SchemaVersion int           `json:"schema_version"`
	OperationID   string        `json:"operation_id"`
	Type          string        `json:"type"`
	IntegrationID string        `json:"integration_id"`
	Status        string        `json:"status"`
	StartedAt     string        `json:"started_at"`
	Steps         []JournalStep `json:"steps"`
}

type Report struct {
	OperationID          string         `json:"operation_id,omitempty"`
	Summary              string         `json:"summary"`
	RequestedTargetCount int            `json:"requested_target_count,omitempty"`
	SkippedTargets       []string       `json:"skipped_targets,omitempty"`
	Targets              []TargetReport `json:"targets,omitempty"`
	Warnings             []string       `json:"warnings,omitempty"`
}

type TargetReport struct {
	IntegrationID            string                 `json:"integration_id,omitempty"`
	TargetID                 string                 `json:"target"`
	DeliveryKind             string                 `json:"delivery_kind,omitempty"`
	CapabilitySurface        []string               `json:"capability_surface,omitempty"`
	ActionClass              string                 `json:"action_class,omitempty"`
	State                    string                 `json:"state,omitempty"`
	ActivationState          string                 `json:"activation_state,omitempty"`
	InteractiveAuthState     string                 `json:"interactive_auth_state,omitempty"`
	RestartRequired          bool                   `json:"restart_required,omitempty"`
	ReloadRequired           bool                   `json:"reload_required,omitempty"`
	NewThreadRequired        bool                   `json:"new_thread_required,omitempty"`
	CatalogPolicy            *CatalogPolicySnapshot `json:"catalog_policy,omitempty"`
	EnvironmentRestrictions  []string               `json:"environment_restrictions,omitempty"`
	VolatileOverrideDetected bool                   `json:"volatile_override_detected,omitempty"`
	TrustResolutionSource    string                 `json:"trust_resolution_source,omitempty"`
	SourceAccessState        string                 `json:"source_access_state,omitempty"`
	EvidenceKey              string                 `json:"evidence_key,omitempty"`
	ManualSteps              []string               `json:"manual_steps,omitempty"`
}
