package domain

const (
	LegacyStateSchemaVersion   = 2
	PreviousStateSchemaVersion = 3
	StateSchemaVersion         = 4
)

type MaterializationState string
type ActivationState string
type AuthenticationState string
type PolicyState string
type VerificationState string
type OriginMode string
type DistributionKind string
type DataReceiptState string
type NativeIdentityState string
type PluginDataDisposition string
type PluginDataOwnership string
type PluginDataCompatibility string

const (
	OriginModeDirectory OriginMode = "directory"
	OriginModeDirect    OriginMode = "direct"

	DistributionUpstream        DistributionKind = "upstream"
	DistributionCommunityBridge DistributionKind = "community_bridge"
	DistributionCommunity       DistributionKind = "community"

	DataReceiptOwned   DataReceiptState = "owned"
	DataReceiptUnknown DataReceiptState = "unknown"
	DataReceiptStale   DataReceiptState = "stale"

	NativeIdentityAbsent        NativeIdentityState = "absent"
	NativeIdentityManaged       NativeIdentityState = "managed"
	NativeIdentityUnmanaged     NativeIdentityState = "unmanaged"
	NativeIdentityIndeterminate NativeIdentityState = "indeterminate"

	PluginDataNone     PluginDataDisposition = "none"
	PluginDataRetained PluginDataDisposition = "retained"

	PluginDataOwnershipNone          PluginDataOwnership = "none"
	PluginDataOwnershipOwned         PluginDataOwnership = "owned"
	PluginDataOwnershipIndeterminate PluginDataOwnership = "indeterminate"

	PluginDataCompatibilityNotApplicable PluginDataCompatibility = "not_applicable"
	PluginDataCompatibilityNotProven     PluginDataCompatibility = "not_proven"
)

const PluginDataCompatibilityWarning = "existing PLUGIN_DATA is retained, but cross-distribution data compatibility is not guaranteed unless proven by the plugin distributions"

// PluginDataDecision is the public switch decision for persistent plugin data.
// It deliberately exposes ownership evidence without exposing the owned path.
type PluginDataDecision struct {
	Disposition   PluginDataDisposition   `json:"disposition"`
	Present       bool                    `json:"present"`
	ReceiptCount  int                     `json:"receipt_count"`
	Ownership     PluginDataOwnership     `json:"ownership"`
	Compatibility PluginDataCompatibility `json:"compatibility"`
	Warning       string                  `json:"warning,omitempty"`
}

type NativeIdentityObservation struct {
	State                     NativeIdentityState `json:"state"`
	Digest                    string              `json:"digest,omitempty"`
	ReceiptReconciled         bool                `json:"receipt_reconciled,omitempty"`
	NativeDiscoveryReconciled bool                `json:"native_discovery_reconciled,omitempty"`
	NativeDiscoveryState      NativeIdentityState `json:"-"`
	NativeDiscoveryAttempted  bool                `json:"-"`
}

const (
	MaterializationAbsent       MaterializationState = "absent"
	MaterializationStaged       MaterializationState = "staged"
	MaterializationMaterialized MaterializationState = "materialized"
	MaterializationDegraded     MaterializationState = "degraded"

	ActivationNotRequired ActivationState = "not_required"
	ActivationPrepared    ActivationState = "prepared"
	ActivationManual      ActivationState = "manual_activation_required"
	ActivationActive      ActivationState = "active"
	ActivationFailed      ActivationState = "failed"

	AuthenticationNotRequired AuthenticationState = "not_required"
	AuthenticationNotChecked  AuthenticationState = "not_checked"
	AuthenticationPending     AuthenticationState = "auth_pending"
	AuthenticationComplete    AuthenticationState = "authenticated"
	AuthenticationFailed      AuthenticationState = "failed"

	PolicyAllowed          PolicyState = "allowed"
	PolicyBlocked          PolicyState = "blocked"
	PolicyApprovalRequired PolicyState = "approval_required"

	VerificationNotRun       VerificationState = "not_run"
	VerificationPackageValid VerificationState = "package_validated"
	VerificationInstalled    VerificationState = "installation_verified"
	VerificationRuntime      VerificationState = "runtime_verified"
	VerificationFailed       VerificationState = "failed"
)

type SourceBinding struct {
	SourceBindingID  string `json:"source_binding_id"`
	RequestedSource  string `json:"requested_source"`
	CanonicalSource  string `json:"canonical_source"`
	Repository       string `json:"repository,omitempty"`
	PackageSubpath   string `json:"package_subpath,omitempty"`
	ResolvedRevision string `json:"resolved_revision"`
	TreeDigest       string `json:"tree_digest"`
	Publisher        string `json:"publisher,omitempty"`
}

type PackageBinding struct {
	LoaderKind     string             `json:"loader_kind"`
	FormatID       string             `json:"format_id"`
	SchemaURI      string             `json:"schema_uri"`
	DeclaredName   string             `json:"declared_name"`
	Version        string             `json:"version,omitempty"`
	ManifestDigest string             `json:"manifest_digest"`
	Inventory      ComponentInventory `json:"inventory"`
}

type NativeObjectOwnership struct {
	ObjectID        string `json:"object_id"`
	Kind            string `json:"kind"`
	LogicalName     string `json:"logical_name,omitempty"`
	Path            string `json:"path,omitempty"`
	SourceRelative  string `json:"source_relative,omitempty"`
	BeforeDigest    string `json:"before_digest,omitempty"`
	ManagedDigest   string `json:"managed_digest,omitempty"`
	ProtectionClass string `json:"protection_class"`
	UserModified    bool   `json:"user_modified,omitempty"`
}

type MutationReceipt struct {
	OperationID      string `json:"operation_id"`
	OperationGroupID string `json:"operation_group_id,omitempty"`
	Sequence         int    `json:"sequence"`
	MutationType     string `json:"mutation_type"`
	ClientBindingID  string `json:"client_binding_id"`
	ActivePath       string `json:"active_path,omitempty"`
	StagingPath      string `json:"staging_path,omitempty"`
	BackupPath       string `json:"backup_path,omitempty"`
	BeforeDigest     string `json:"before_digest,omitempty"`
	AfterDigest      string `json:"after_digest,omitempty"`
	Phase            string `json:"phase"`
}

// ClientPackageRevision records the exact portable package revision that was
// projected into one client. Installation.Package is the latest accepted
// revision, while individual clients may temporarily converge one at a time.
type ClientPackageRevision struct {
	Version          string           `json:"version,omitempty"`
	ResolvedRevision string           `json:"resolved_revision,omitempty"`
	TreeDigest       string           `json:"tree_digest"`
	ManifestDigest   string           `json:"manifest_digest"`
	DistributionID   string           `json:"distribution_id,omitempty"`
	ReleaseSequence  uint64           `json:"release_sequence,omitempty"`
	CatalogEvidence  *CatalogEvidence `json:"catalog_evidence,omitempty"`
}

type ClientBinding struct {
	InstallIntent    InstallIntent           `json:"install_intent,omitempty"`
	ClientBindingID  string                  `json:"client_binding_id"`
	ClientID         string                  `json:"client_id"`
	Scope            string                  `json:"scope"`
	TargetLocator    string                  `json:"target_locator"`
	PhysicalArtifact string                  `json:"physical_artifact_id"`
	Materialization  MaterializationState    `json:"materialization"`
	Activation       ActivationState         `json:"activation"`
	Authentication   AuthenticationState     `json:"authentication"`
	Policy           PolicyState             `json:"policy"`
	Verification     VerificationState       `json:"verification"`
	PackageRevision  *ClientPackageRevision  `json:"package_revision,omitempty"`
	DataReceiptID    string                  `json:"data_receipt_id,omitempty"`
	AffectedSurfaces []string                `json:"affected_surfaces,omitempty"`
	NativeObjects    []NativeObjectOwnership `json:"native_objects,omitempty"`
	Receipts         []MutationReceipt       `json:"receipts,omitempty"`
	UpdatedAt        string                  `json:"updated_at"`
}

// DirectoryOrigin is the minimum signed Directory provenance needed to make
// lifecycle decisions without copying mutable product or policy metadata into
// local state. DistributionID and DesiredReleaseSequence, bound to
// Installation.Source.ResolvedRevision, form the immutable desired release
// identity.
type DirectoryOrigin struct {
	ProductID              string           `json:"product_id"`
	DistributionID         string           `json:"distribution_id"`
	DistributionKind       DistributionKind `json:"distribution_kind"`
	DesiredReleaseSequence uint64           `json:"desired_release_sequence"`
	SnapshotSchema         int              `json:"snapshot_schema,omitempty"`
	SnapshotSequence       uint64           `json:"snapshot_sequence,omitempty"`
	SnapshotDigest         string           `json:"snapshot_digest,omitempty"`
}

// DataReceipt proves ownership of one persistent PLUGIN_DATA directory. It is
// installation-level because multiple logical clients can share one physical
// backend. Package replacement and binding removal never consume the receipt.
type DataReceipt struct {
	DataReceiptID   string           `json:"data_receipt_id"`
	PhysicalBackend string           `json:"physical_backend_id"`
	Scope           string           `json:"scope"`
	Locator         string           `json:"locator"`
	OwnershipDigest string           `json:"ownership_digest"`
	State           DataReceiptState `json:"state"`
	CreatedAt       string           `json:"created_at,omitempty"`
	UpdatedAt       string           `json:"updated_at,omitempty"`
}

type Installation struct {
	InstallPreferences []InstallPreference      `json:"install_preferences,omitempty"`
	InstallationID     string                   `json:"installation_id"`
	DeclaredName       string                   `json:"declared_name"`
	Source             SourceBinding            `json:"source"`
	Package            PackageBinding           `json:"package"`
	OriginMode         OriginMode               `json:"origin_mode,omitempty"`
	Directory          *DirectoryOrigin         `json:"directory,omitempty"`
	OperationGroupID   string                   `json:"operation_group_id,omitempty"`
	DataReceipts       map[string]DataReceipt   `json:"data_receipts,omitempty"`
	DataRetained       bool                     `json:"data_retained,omitempty"`
	Clients            map[string]ClientBinding `json:"clients"`
	NeedsRebind        bool                     `json:"needs_rebind,omitempty"`
	CreatedAt          string                   `json:"created_at"`
	UpdatedAt          string                   `json:"updated_at"`
}

type StateFileV2 struct {
	SchemaVersion       int               `json:"schema_version"`
	Installations       []Installation    `json:"installations"`
	TransactionReceipts []MutationReceipt `json:"transaction_receipts,omitempty"`
}
