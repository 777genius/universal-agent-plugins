package domain

type ClientID string
type DetectionStatus string
type InstallScope string
type PackageMode string
type ActivationMode string
type SupportLevel string
type PlanStatus string
type ComponentKind string

const (
	ClientCodex    ClientID = "codex"
	ClientChatGPT  ClientID = "chatgpt"
	ClientCursor   ClientID = "cursor"
	ClientCopilot  ClientID = "copilot"
	ClientVSCode   ClientID = "vscode"
	ClientKiro     ClientID = "kiro"
	ClientClaude   ClientID = "claude"
	ClientGemini   ClientID = "gemini"
	ClientOpenCode ClientID = "opencode"
	ClientCline    ClientID = "cline"
	ClientWindsurf ClientID = "windsurf"

	DetectionNotDetected DetectionStatus = "not_detected"
	DetectionDetected    DetectionStatus = "detected"

	ScopeUser    InstallScope = "user"
	ScopeProject InstallScope = "project"

	PackageNative     PackageMode = "native"
	PackageProjection PackageMode = "compatibility_projection"
	PackageBridge     PackageMode = "client_bridge"
	PackagePrepared   PackageMode = "prepared_package"

	ActivationAutomatic ActivationMode = "automatic"
	ActivationByClient  ActivationMode = "client_managed"
	ActivationByUser    ActivationMode = "manual"

	SupportNative      SupportLevel = "native"
	SupportProjected   SupportLevel = "projected"
	SupportPrepared    SupportLevel = "prepared"
	SupportUnsupported SupportLevel = "unsupported"

	PlanReady                    PlanStatus = "ready"
	PlanPrepared                 PlanStatus = "prepared"
	PlanManualActivationRequired PlanStatus = "manual_activation_required"
	PlanUnsupported              PlanStatus = "unsupported"

	ComponentSkill     ComponentKind = "skill"
	ComponentMCPServer ComponentKind = "mcp_server"
	ComponentApp       ComponentKind = "app"
	ComponentExtension ComponentKind = "extension"
)

// ClientDefinition is the declarative identity contract shared by the CLI,
// Directory validation, planning, and read-only detection. Operational client
// mutation remains in client-specific adapters and is intentionally not part
// of this registry.
type ClientDefinition struct {
	ID                    ClientID
	DisplayName           string
	BackendFamily         string
	DirectoryDelivery     string
	CatalogPackage        string
	LegacyCatalogRequired bool
	Capabilities          ClientCapabilities
}

var clientDefinitions = []ClientDefinition{
	clientDefinition(ClientCodex, "OpenAI Codex", "codex", "managed", "projected", true, PackageProjection, SupportProjected, SupportProjected, SupportUnsupported),
	clientDefinition(ClientChatGPT, "ChatGPT", "chatgpt", "manual_activation", "projected", false, PackageProjection, SupportProjected, SupportUnsupported, SupportUnsupported),
	clientDefinition(ClientCursor, "Cursor", "cursor", "managed", "native", true, PackageNative, SupportNative, SupportNative, SupportNative),
	clientDefinition(ClientCopilot, "GitHub Copilot CLI", "github-copilot", "managed", "native", true, PackageNative, SupportNative, SupportNative, SupportNative),
	clientDefinition(ClientVSCode, "Visual Studio Code", "github-copilot", "prepared", "prepared", true, PackagePrepared, SupportPrepared, SupportPrepared, SupportPrepared),
	clientDefinition(ClientKiro, "Kiro", "kiro", "managed", "native", true, PackageNative, SupportNative, SupportNative, SupportUnsupported),
	withActivation(clientDefinition(ClientClaude, "Claude Code", "claude", "managed", "projected", false, PackageProjection, SupportProjected, SupportProjected, SupportUnsupported), ActivationAutomatic),
	clientDefinition(ClientGemini, "Gemini CLI", "gemini", "managed", "native", false, PackageNative, SupportNative, SupportNative, SupportUnsupported),
	withActivation(clientDefinition(ClientOpenCode, "OpenCode", "opencode", "managed", "prepared", false, PackagePrepared, SupportPrepared, SupportPrepared, SupportUnsupported), ActivationAutomatic),
	withActivation(clientDefinition(ClientCline, "Cline", "cline", "managed", "native", false, PackageNative, SupportNative, SupportNative, SupportUnsupported), ActivationAutomatic),
	clientDefinition(ClientWindsurf, "Windsurf / Devin", "windsurf", "prepared", "prepared", false, PackagePrepared, SupportPrepared, SupportPrepared, SupportPrepared),
}

func withActivation(definition ClientDefinition, activation ActivationMode) ClientDefinition {
	definition.Capabilities.ActivationMode = activation
	return definition
}

func clientDefinition(id ClientID, displayName, backendFamily, delivery, catalogPackage string, legacyRequired bool, packageMode PackageMode, skill, mcp, extension SupportLevel) ClientDefinition {
	transports := map[string]SupportLevel{"stdio": mcp, "streamable-http": mcp, "sse": mcp}
	// Codex rejects native SSE; OpenCode remote fallback cannot preserve SSE-first.
	if id == ClientOpenCode || id == ClientCodex {
		transports["sse"] = SupportUnsupported
	}
	appSupport := SupportUnsupported
	if id == ClientChatGPT {
		appSupport = SupportProjected
	}
	return ClientDefinition{
		ID: id, DisplayName: displayName, BackendFamily: backendFamily,
		DirectoryDelivery: delivery, CatalogPackage: catalogPackage, LegacyCatalogRequired: legacyRequired,
		Capabilities: ClientCapabilities{
			ClientID: id, PackageMode: packageMode, ActivationMode: ActivationByUser,
			Scopes: []InstallScope{ScopeUser}, SkillSupport: skill, MCPTransports: transports,
			AppSupport: appSupport, ExtensionSupport: extension,
		},
	}
}

// ClientDefinitions returns registry entries in stable user-facing order.
func ClientDefinitions() []ClientDefinition {
	result := make([]ClientDefinition, len(clientDefinitions))
	for index, definition := range clientDefinitions {
		result[index] = definition
		result[index].Capabilities.Scopes = append([]InstallScope(nil), definition.Capabilities.Scopes...)
		result[index].Capabilities.MCPTransports = make(map[string]SupportLevel, len(definition.Capabilities.MCPTransports))
		for transport, support := range definition.Capabilities.MCPTransports {
			result[index].Capabilities.MCPTransports[transport] = support
		}
	}
	return result
}

func ClientDefinitionFor(id ClientID) (ClientDefinition, bool) {
	for _, definition := range ClientDefinitions() {
		if definition.ID == id {
			return definition, true
		}
	}
	return ClientDefinition{}, false
}

func SupportedClientIDs() []ClientID {
	definitions := ClientDefinitions()
	result := make([]ClientID, len(definitions))
	for index, definition := range definitions {
		result[index] = definition.ID
	}
	return result
}

func IsSupportedClient(id ClientID) bool {
	_, ok := ClientDefinitionFor(id)
	return ok
}

func SameClientBackend(first, second ClientID) bool {
	if first == second {
		return true
	}
	firstDefinition, firstOK := ClientDefinitionFor(first)
	secondDefinition, secondOK := ClientDefinitionFor(second)
	return firstOK && secondOK && firstDefinition.BackendFamily == secondDefinition.BackendFamily
}

type ClientSurface struct {
	ID       string `json:"id"`
	Detected bool   `json:"detected"`
	Evidence string `json:"evidence,omitempty"`
}

type DetectedClient struct {
	ClientID    ClientID        `json:"client_id"`
	DisplayName string          `json:"display_name"`
	Status      DetectionStatus `json:"status"`
	Version     string          `json:"version,omitempty"`
	Surfaces    []ClientSurface `json:"surfaces,omitempty"`
	// ExecutablePath and ConfigRoot are operational locators. They must never be
	// emitted by the public JSON renderer because they can reveal the user home.
	ExecutablePath string `json:"-"`
	ConfigRoot     string `json:"-"`
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
}

type DeliveryPlan struct {
	PersonalChatGPTPreparation bool `json:"-"`

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
	TargetAnchor string `json:"-"`
	TargetRoot   string `json:"-"`
	ActivePath   string `json:"-"`
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
	Client            DetectedClient `json:"client"`
	Plan              DeliveryPlan   `json:"plan"`
	Delivery          StagedDelivery `json:"delivery"`
	DeclaredName      string         `json:"declared_name"`
	Replacing         bool           `json:"replacing"`
	Interactive       bool           `json:"interactive"`
	BackendExecutable string         `json:"-"`
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
	Activation             ActivationState     `json:"activation"`
	Authentication         AuthenticationState `json:"authentication"`
	Policy                 PolicyState         `json:"policy"`
	Verification           VerificationState   `json:"verification"`
	UserActions            []string            `json:"user_actions,omitempty"`
	LocalActions           []string            `json:"-"`
	ActivationAttested     bool                `json:"activation_attested,omitempty"`
	AuthenticationAttested bool                `json:"authentication_attested,omitempty"`
	// AuthoritativeObservation marks recognized negative verifier evidence.
	// It is transient control-plane metadata and is never persisted as state.
	AuthoritativeObservation bool `json:"-"`
}

type DeactivationRequest struct {
	Client              DetectedClient          `json:"client"`
	DeclaredName        string                  `json:"declared_name"`
	CurrentActivation   ActivationState         `json:"current_activation"`
	Interactive         bool                    `json:"interactive"`
	ExternalUninstalled bool                    `json:"external_uninstalled"`
	Confirmed           bool                    `json:"confirmed"`
	PhysicalArtifactID  string                  `json:"physical_artifact_id"`
	BackendExecutable   string                  `json:"-"`
	ManagedArtifactPath string                  `json:"-"`
	NativeObjects       []NativeObjectOwnership `json:"-"`
}

type DeactivationOutcome struct {
	Activation              ActivationState `json:"activation"`
	ArtifactRemovalAllowed  bool            `json:"artifact_removal_allowed"`
	ExternalRemovalComplete bool            `json:"external_removal_complete"`
	UserActions             []string        `json:"user_actions,omitempty"`
	LocalActions            []string        `json:"-"`
}
