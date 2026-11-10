package domain

type ClientBinding struct {
	ProfileAuthority      *ProfileAuthority      `json:"profile_authority,omitempty"`
	ProfileNamespace      string                 `json:"profile_namespace,omitempty"`
	LocalEntryObservation *LocalEntryObservation `json:"local_entry_observation,omitempty"`
	SelectedDelivery      SelectedDelivery       `json:"selected_delivery,omitzero"`
	PendingNativeIntent   *PendingNativeIntent   `json:"pending_native_intent,omitempty"`
	InstallIntent         InstallIntent          `json:"install_intent,omitempty"`
	ClientBindingID       string                 `json:"client_binding_id"`
	ClientID              string                 `json:"client_id"`
	Scope                 string                 `json:"scope"`
	TargetLocator         string                 `json:"target_locator"`
	PhysicalArtifact      string                 `json:"physical_artifact_id"`
	Materialization       MaterializationState   `json:"materialization"`
	Activation            ActivationState        `json:"activation"`
	Authentication        AuthenticationState    `json:"authentication"`
	Policy                PolicyState            `json:"policy"`
	Verification          VerificationState      `json:"verification"`
	PackageRevision       *ClientPackageRevision `json:"package_revision,omitempty"`
	DataReceiptID         string                 `json:"data_receipt_id,omitempty"`
	AffectedSurfaces      []string               `json:"affected_surfaces,omitempty"`
	// NativeProfileRoot binds a registration to its selected native profile.
	NativeProfileRoot string `json:"native_profile_root,omitempty"`
	// NativeActivationAttempt marks a potentially unacknowledged native effect.
	NativeActivationAttempt string                  `json:"native_activation_attempt,omitempty"`
	NativeObjects           []NativeObjectOwnership `json:"native_objects,omitempty"`
	Receipts                []MutationReceipt       `json:"receipts,omitempty"`
	UpdatedAt               string                  `json:"updated_at"`
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
	LocalChatGPTMapping *ChatGPTLocalMapping `json:"local_chatgpt_mapping,omitempty"`

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
	SchemaVersion       int                  `json:"schema_version"`
	Installations       []Installation       `json:"installations"`
	TransactionReceipts []MutationReceipt    `json:"transaction_receipts,omitempty"`
	EnvironmentReceipts []EnvironmentReceipt `json:"environment_receipts,omitempty"`
}

// PhysicalProfileOwner freezes durable scope independently of a surviving binding.
type PhysicalProfileOwner struct {
	Namespace       string            `json:"namespace"`
	InstallationID  string            `json:"installation_id"`
	ClientID        string            `json:"client_id"`
	ClientBindingID string            `json:"client_binding_id"`
	Authority       *ProfileAuthority `json:"authority"`
}

func CloneProfileAuthority(a *ProfileAuthority) *ProfileAuthority {
	if a == nil {
		return nil
	}
	copied := *a
	return &copied
}
func SameProfileAuthority(a, b *ProfileAuthority) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

// LifecycleConverged is the shared persisted lifecycle completion contract.
func (client ClientBinding) LifecycleConverged() bool {
	if client.InstallIntent == InstallIntentPrepare {
		return client.Materialization == MaterializationMaterialized && client.Activation == ActivationPrepared && client.Verification == VerificationPackageValid
	}
	authComplete := client.Authentication == AuthenticationNotRequired || client.Authentication == AuthenticationComplete
	return client.Materialization == MaterializationMaterialized &&
		client.Activation == ActivationActive &&
		client.Verification == VerificationInstalled && authComplete
}
