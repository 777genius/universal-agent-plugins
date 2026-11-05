package domain

type ClientSurface struct {
	ID       string `json:"id"`
	Detected bool   `json:"detected"`
	Evidence string `json:"evidence,omitempty"`
}

// OpenCodeHostAuthority is a read-only, immutable prepared-host port. It does
// no probing. The client contract owns the closed parent profile snapshot.
type OpenCodeHostAuthority interface {
	ValidateNative(skills bool, transports []string) error
}

type DetectedClient struct {
	ProfileAuthority *ProfileAuthority     `json:"-"`
	ProfileNamespace string                `json:"-"`
	OpenCodeHost     OpenCodeHostAuthority `json:"-"`
	ClientID         ClientID              `json:"client_id"`
	DisplayName      string                `json:"display_name"`
	Status           DetectionStatus       `json:"status"`
	Version          string                `json:"version,omitempty"`
	Surfaces         []ClientSurface       `json:"surfaces,omitempty"`
	// ExecutablePath and ConfigRoot are operational locators. They must never be
	// emitted by the public JSON renderer because they can reveal the user home.
	ExecutablePath string `json:"-"`
	ConfigRoot     string `json:"-"`
	// NativeConfigRoot is the observed original native user root for adapters
	// with independent legacy and current profiles; never emitted publicly.
	NativeConfigRoot string `json:"-"`
	// DetectionError is private diagnostic evidence. A failed client must not
	// contribute paths or become a lifecycle target.
	DetectionError error `json:"-"`
}

type ClientCapabilities struct {
	ClientID         ClientID                `json:"client_id"`
	PackageMode      PackageMode             `json:"package_mode"`
	ActivationMode   ActivationMode          `json:"activation_mode"`
	Scopes           []InstallScope          `json:"scopes"`
	SkillSupport     SupportLevel            `json:"skill_support"`
	MCPTransports    map[string]SupportLevel `json:"mcp_transports,omitempty"`
	AppSupport       SupportLevel            `json:"app_support"`
	ExtensionSupport SupportLevel            `json:"extension_support"`
}

type ComponentDecision struct {
	Kind    ComponentKind `json:"kind"`
	Name    string        `json:"name"`
	Support SupportLevel  `json:"support"`
	Reason  string        `json:"reason,omitempty"`
	// Source and SourceName are set only for a component the installer
	// renders, such as ComponentSourceCommand and the command's name.
	Source     string `json:"source,omitempty"`
	SourceName string `json:"source_name,omitempty"`
}

type DeliveryPlan struct {
	requiresNativeProjectionRefresh bool

	TargetContext              TargetContext           `json:"-"`
	profileAuthority           *ProfileAuthority       `json:"-"`
	profileNamespace           string                  `json:"-"`
	PreviousNativeObjects      []NativeObjectOwnership `json:"-"`
	LocalEntryObservation      *LocalEntryObservation  `json:"-"`
	SelectedDelivery           SelectedDelivery        `json:"-"`
	OpenCodeHost               OpenCodeHostAuthority   `json:"-"`
	PersonalChatGPTPreparation bool                    `json:"-"`

	InstallIntent      InstallIntent       `json:"install_intent,omitempty"`
	ClientID           ClientID            `json:"client_id"`
	Scope              InstallScope        `json:"scope"`
	Status             PlanStatus          `json:"status"`
	PackageMode        PackageMode         `json:"package_mode"`
	Activation         ActivationState     `json:"activation"`
	Authentication     AuthenticationState `json:"authentication"`
	Policy             PolicyState         `json:"policy"`
	Verification       VerificationState   `json:"verification"`
	PhysicalArtifactID string              `json:"physical_artifact_id"`
	// DeclaredName and NativeRegistry* are preflight-only identity inputs. They
	// are deliberately excluded from public output because registry locators
	// can reveal local user paths.
	DeclaredName             string `json:"-"`
	DeclaredVersion          string `json:"-"`
	NativeRegistryRoot       string `json:"-"`
	NativeRegistryExecutable string `json:"-"`
	// LocalPreparationAuthorized records validated package evidence or an explicit
	// personal Context7 registration receipt that permits
	// creation of the local prepared package even when a remote, manually
	// activated registry cannot be observed. It is never evidence that the
	// remote identity is free, activated, authenticated, or verified.
	LocalPreparationAuthorized bool                `json:"-"`
	Components                 []ComponentDecision `json:"components,omitempty"`
	UserActions                []string            `json:"user_actions,omitempty"`
	// LocalActions can contain operational paths and are rendered only in
	// human-readable output. They must never be emitted by the public JSON API.
	LocalActions []string     `json:"-"`
	Warnings     []string     `json:"warnings,omitempty"`
	Diagnostics  []Diagnostic `json:"diagnostics,omitempty"`
	// TargetRoot and ActivePath are intentionally excluded from public JSON.
	TargetAnchor string `json:"-"`
	TargetRoot   string `json:"-"`
	ActivePath   string `json:"-"`
}

// DeliveryTarget contains deterministic operational paths computed from the
// configured client roots. Persisted state must be checked against this value
// before any destructive operation.
type DeliveryTarget struct {
	TargetContext      TargetContext     `json:"-"`
	NativeRegistryRoot string            `json:"-"`
	profileAuthority   *ProfileAuthority `json:"-"`
	profileNamespace   string            `json:"-"`
	TargetAnchor       string            `json:"-"`
	TargetRoot         string            `json:"-"`
	ActivePath         string            `json:"-"`
}

type OpenAIMCPAuthHint struct {
	OAuthResource     string `json:"oauth_resource,omitempty"`
	BearerTokenEnvVar string `json:"bearer_token_env_var,omitempty"`
}

type CompatibilityHints struct {
	// Compatibility preserves generic, per-client catalog requirements. The
	// OpenAI map remains for legacy projection consumers and is not authoritative
	// for whether authentication is required.
	Compatibility map[string]CatalogCompatibility `json:"compatibility,omitempty"`
	OpenAIMCPAuth map[string]OpenAIMCPAuthHint    `json:"openai_mcp_auth,omitempty"`
}

type StagedDelivery struct {
	ClientID       ClientID                `json:"client_id"`
	OwnedBase      string                  `json:"-"`
	ActivePath     string                  `json:"-"`
	StagingPath    string                  `json:"-"`
	ArtifactDigest string                  `json:"artifact_digest"`
	NativeObjects  []NativeObjectOwnership `json:"native_objects,omitempty"`
}

type ActivationRequest struct {
	// PackageUnchanged means activation reuses the committed, verified managed
	// package without a new package transaction. External CLI effects may still
	// be required; VerifyOnly describes those effects independently.
	PackageUnchanged  bool                  `json:"-"`
	NativeAttempt     NativeAttemptIdentity `json:"-"`
	Client            DetectedClient        `json:"client"`
	Plan              DeliveryPlan          `json:"plan"`
	Delivery          StagedDelivery        `json:"delivery"`
	DeclaredName      string                `json:"declared_name"`
	Replacing         bool                  `json:"replacing"`
	Interactive       bool                  `json:"interactive"`
	BackendExecutable string                `json:"-"`
	// PreviousNativeObjects binds replacement preflight to the exact native
	// objects recorded by the currently installed package revision.
	PreviousNativeObjects []NativeObjectOwnership `json:"-"`
	// VerifyOnly forbids client mutation and asks the provider to inspect the
	// current client state. ActivationComplete is an explicit user attestation
	// accepted only when verification is unavailable or returns unknown evidence.
	VerifyOnly         bool `json:"verify_only,omitempty"`
	ActivationComplete bool `json:"activation_complete,omitempty"`
}

type ActivationOutcome struct {
	LocalEntryObservation  *LocalEntryObservation `json:"-"`
	Activation             ActivationState        `json:"activation"`
	Authentication         AuthenticationState    `json:"authentication"`
	Policy                 PolicyState            `json:"policy"`
	Verification           VerificationState      `json:"verification"`
	UserActions            []string               `json:"user_actions,omitempty"`
	LocalActions           []string               `json:"-"`
	ActivationAttested     bool                   `json:"activation_attested,omitempty"`
	AuthenticationAttested bool                   `json:"authentication_attested,omitempty"`
	// AuthoritativeObservation marks recognized negative verifier evidence.
	// It is transient control-plane metadata and is never persisted as state.
	AuthoritativeObservation bool `json:"-"`
	// Native effect evidence is transient and distinct from client verification.
	NativeEffect  NativeEffectState       `json:"-"`
	NativeObjects []NativeObjectOwnership `json:"-"`
}

type DeactivationRequest struct {
	TargetContext         TargetContext           `json:"-"`
	NativeRegistryRoot    string                  `json:"-"`
	LocalEntryObservation *LocalEntryObservation  `json:"-"`
	RemoveOwnedEntry      bool                    `json:"-"`
	SelectedDelivery      SelectedDelivery        `json:"-"`
	Client                DetectedClient          `json:"client"`
	DeclaredName          string                  `json:"declared_name"`
	CurrentActivation     ActivationState         `json:"current_activation"`
	Interactive           bool                    `json:"interactive"`
	ExternalUninstalled   bool                    `json:"external_uninstalled"`
	Confirmed             bool                    `json:"confirmed"`
	PhysicalArtifactID    string                  `json:"physical_artifact_id"`
	BackendExecutable     string                  `json:"-"`
	ManagedArtifactPath   string                  `json:"-"`
	NativeObjects         []NativeObjectOwnership `json:"-"`
}

type DeactivationOutcome struct {
	// NativeEffect is transient evidence of a native transaction's actual
	// effects. An ordinary error never establishes that removal was unchanged.
	NativeEffect            NativeEffectState `json:"-"`
	Activation              ActivationState   `json:"activation"`
	ArtifactRemovalAllowed  bool              `json:"artifact_removal_allowed"`
	ExternalRemovalComplete bool              `json:"external_removal_complete"`
	UserActions             []string          `json:"user_actions,omitempty"`
	LocalActions            []string          `json:"-"`
}

func supportsLocalDelivery(id ClientID) bool { return id == ClientVSCode }

func (p DeliveryPlan) WithProfileAuthority(token *ProfileAuthority, namespace string) DeliveryPlan {
	p.profileAuthority = CloneProfileAuthority(token)
	p.profileNamespace = namespace
	return p
}
func (p DeliveryPlan) ProfileAuthority() *ProfileAuthority {
	return CloneProfileAuthority(p.profileAuthority)
}
func (p DeliveryPlan) ProfileNamespace() string { return p.profileNamespace }

func (p DeliveryTarget) WithProfileAuthority(token *ProfileAuthority, namespace string) DeliveryTarget {
	p.profileAuthority = CloneProfileAuthority(token)
	p.profileNamespace = namespace
	return p
}
func (p DeliveryTarget) ProfileAuthority() *ProfileAuthority {
	return CloneProfileAuthority(p.profileAuthority)
}
func (p DeliveryTarget) ProfileNamespace() string { return p.profileNamespace }
