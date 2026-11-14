package domain

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

const (
	LegacyStateSchemaVersion   = 2
	PreviousStateSchemaVersion = 3
	PreviousStateSchemaV4      = 4
	StateSchemaVersion         = 5
)

// EnvironmentAuthority is narrow proof that a managed mutation belongs to one
// dependency environment alias and target. It is copied into the transaction
// receipt before the first managed effect and never permits nil-owner adoption.
type EnvironmentAuthority struct {
	EnvironmentID   string `json:"environment_id"`
	Alias           string `json:"alias"`
	ClientID        string `json:"client_id"`
	ReceiptID       string `json:"receipt_id,omitempty"`
	InstallationID  string `json:"installation_id,omitempty"`
	ClientBindingID string `json:"client_binding_id,omitempty"`
	ExpectedAction  string `json:"expected_action"`
	SourceKey       string `json:"source_key,omitempty"`
}

// EnvironmentReceipt associates a portable dependency selection with an
// existing installation and binding. It is local state, never lock or source
// identity, and remains present until terminal removal commits.
type EnvironmentReceipt struct {
	ReceiptID        string            `json:"receipt_id"`
	EnvironmentID    string            `json:"environment_id"`
	Alias            string            `json:"alias"`
	ClientID         string            `json:"client_id"`
	InstallationID   string            `json:"installation_id"`
	ClientBindingID  string            `json:"client_binding_id"`
	SourceKey        string            `json:"source_key"`
	PackageDigest    string            `json:"package_digest"`
	ProfileAuthority *ProfileAuthority `json:"profile_authority,omitempty"`
	DesiredAction    string            `json:"desired_action"`
	Phase            string            `json:"phase"`
}

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
	SourceKind       string `json:"source_kind,omitempty"`
	SourceURL        string `json:"source_url,omitempty"`
	ArchiveSHA256    string `json:"archive_sha256,omitempty"`
	StableSourceKey  string `json:"stable_source_key,omitempty"`
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

// CursorHookReceipt carries the primitive facts of the pure hook planner.
// Zero is no ownership proof; omitzero preserves Local and legacy wire bytes.
type CursorHookReceipt struct {
	Version         int    `json:"version"`
	Event           string `json:"event"`
	Executable      string `json:"executable"`
	Selector        string `json:"selector"`
	Shell           string `json:"shell"`
	EntryDigest     string `json:"entry_digest"`
	RemainderDigest string `json:"remainder_digest"`
}

type NativeObjectOwnership struct {
	CursorReceipt   CursorHookReceipt `json:"cursor_receipt,omitzero"`
	ObjectID        string            `json:"object_id"`
	Kind            string            `json:"kind"`
	LogicalName     string            `json:"logical_name,omitempty"`
	Path            string            `json:"path,omitempty"`
	SourceRelative  string            `json:"source_relative,omitempty"`
	BeforeDigest    string            `json:"before_digest,omitempty"`
	ManagedDigest   string            `json:"managed_digest,omitempty"`
	ProtectionClass string            `json:"protection_class"`
	UserModified    bool              `json:"user_modified,omitempty"`
}

type MutationReceipt struct {
	// PublicationParentAuthority records ancestry for a fresh standalone swap;
	// it is structural evidence, not a physical profile ownership grant.
	PublicationParentAuthority *ProfileAuthority      `json:"publication_parent_authority,omitempty"`
	DirectoryProof             json.RawMessage        `json:"physical_directory_proof,omitempty"`
	ProfileOwners              []PhysicalProfileOwner `json:"profile_owners,omitempty"`
	DataReceiptID              string                 `json:"data_receipt_id,omitempty"`
	OperationID                string                 `json:"operation_id"`
	OperationGroupID           string                 `json:"operation_group_id,omitempty"`
	Sequence                   int                    `json:"sequence"`
	MutationType               string                 `json:"mutation_type"`
	ClientBindingID            string                 `json:"client_binding_id"`
	ActivePath                 string                 `json:"active_path,omitempty"`
	StagingPath                string                 `json:"staging_path,omitempty"`
	BackupPath                 string                 `json:"backup_path,omitempty"`
	BeforeDigest               string                 `json:"before_digest,omitempty"`
	AfterDigest                string                 `json:"after_digest,omitempty"`
	Phase                      string                 `json:"phase"`
	EnvironmentAuthority       *EnvironmentAuthority  `json:"environment_authority,omitempty"`
}

// ValidatePublicationParent checks the exact original role without filesystem I/O.
func (r MutationReceipt) ValidatePublicationParent(standalone bool) error {
	parent := r.PublicationParentAuthority
	if parent == nil {
		if standalone && r.MutationType == "directory_swap" {
			return fmt.Errorf("standalone publication parent authority is required")
		}
		return nil
	}
	if !standalone || r.MutationType != "directory_swap" || parent.IsZero() || r.BeforeDigest != "" ||
		(r.Phase != "state_committed" && r.Phase != "committed") {
		return fmt.Errorf("invalid standalone publication parent role")
	}
	root := parent.Facts().CanonicalRoot
	for _, path := range []string{r.ActivePath, r.StagingPath, r.BackupPath} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Dir(path) != root {
			return fmt.Errorf("standalone publication parent differs from receipt paths")
		}
	}
	return nil
}

// ClientPackageRevision records the exact portable package revision that was
// projected into one client. Installation.Package is the latest accepted
// revision, while individual clients may temporarily converge one at a time.
type ClientPackageRevision struct {
	Skill            *SkillSourceFacts    `json:"skill,omitempty"`
	WebSkill         *WebSkillSourceFacts `json:"web_skill,omitempty"`
	Version          string               `json:"version,omitempty"`
	ResolvedRevision string               `json:"resolved_revision,omitempty"`
	TreeDigest       string               `json:"tree_digest"`
	ManifestDigest   string               `json:"manifest_digest"`
	DistributionID   string               `json:"distribution_id,omitempty"`
	ReleaseSequence  uint64               `json:"release_sequence,omitempty"`
	CatalogEvidence  *CatalogEvidence     `json:"catalog_evidence,omitempty"`
}
