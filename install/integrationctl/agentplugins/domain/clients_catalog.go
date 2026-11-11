package domain

import (
	"strings"
)

type ClientID string
type DetectionStatus string
type InstallScope string
type PackageMode string
type ActivationMode string
type SupportLevel string
type PlanStatus string
type ComponentKind string

const (
	ClientCodex       ClientID = "codex"
	ClientChatGPT     ClientID = "chatgpt"
	ClientCursor      ClientID = "cursor"
	ClientCopilot     ClientID = "copilot"
	ClientVSCode      ClientID = "vscode"
	ClientKiro        ClientID = "kiro"
	ClientClaude      ClientID = "claude"
	ClientGemini      ClientID = "gemini"
	ClientOpenCode    ClientID = "opencode"
	ClientCline       ClientID = "cline"
	ClientWindsurf    ClientID = "windsurf"
	ClientGrok        ClientID = "grok"
	ClientKimi        ClientID = "kimi"
	ClientQoder       ClientID = "qoder"
	ClientPi          ClientID = "pi"
	ClientAntigravity ClientID = "antigravity"
	ClientHermes      ClientID = "hermes"

	DetectionNotDetected DetectionStatus = "not_detected"
	DetectionDetected    DetectionStatus = "detected"

	ScopeUser    InstallScope = "user"
	ScopeProject InstallScope = "project"
	ScopeLocal   InstallScope = "local"

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
	ProjectPolicy         ProjectPolicy `json:"-"`
	ID                    ClientID
	DisplayName           string
	BackendFamily         string
	DirectoryDelivery     string
	CatalogPackage        string
	LegacyCatalogRequired bool
	// PlansWithoutHostPresence marks a client that is not installed on this
	// machine at all. Planning for it produces a prepared artifact plus
	// instructions, so "not detected here" is not a planning failure.
	PlansWithoutHostPresence bool
	// DirectoryPreparationPurpose is the single bounded resolve purpose this
	// client may acquire source under. An empty value means the client has
	// none, which is the normal case.
	DirectoryPreparationPurpose DirectoryResolvePurpose
	Capabilities                ClientCapabilities
	// CommandSupport is whether the installer delivers portable commands to
	// this client as skills. Empty follows Capabilities.SkillSupport;
	// SupportUnsupported withholds every command. Backend siblings install one
	// shared package, so they must agree. Like Traits, it is installer policy
	// and not part of the public JSON capabilities contract.
	CommandSupport SupportLevel
	// Traits are installer policy for this client. They are not part of the
	// public JSON capabilities contract and must not be serialized onto it.
	Traits ClientTraits
}

var clientDefinitions = []ClientDefinition{
	withProjectScope(clientDefinition(ClientCodex, "OpenAI Codex", "codex", "managed", "projected", true, PackageProjection, SupportProjected, SupportProjected, SupportUnsupported, SupportUnsupported, SupportUnsupported, ClientTraits{
		QualifiedLocalInstallerFacade: true,
		ShowsVerifiedLocalNextAction:  true,
		InstallIntents:                []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:                 LifecycleCLIRegistry,
		HonorsOpenAIMCPAuthHints:      true,
		SupportsPreparedRecovery:      true,
		BindsNativeProfileRoot:        true,
		ProtectsModifiedUserRawSkill:  true,
	}), LifecycleNativeConfig),
	withoutHostPresence(withDirectoryPreparation(
		clientDefinition(ClientChatGPT, "ChatGPT", "chatgpt", "manual_activation", "projected", false, PackageProjection, SupportProjected, SupportUnsupported, SupportUnsupported, SupportProjected, SupportUnsupported, ClientTraits{
			InstallIntents:                    []InstallIntent{InstallIntentAutomatic, InstallIntentPrepare},
			LifecycleKind:                     LifecycleManual,
			RequiresPersonalMappingForPrepare: true,
		}),
		DirectoryResolveContext7ChatGPTPreparation)),
	withProjectScope(clientDefinition(ClientCursor, "Cursor", "cursor", "managed", "native", true, PackageNative, SupportNative, SupportNative, SupportNative, SupportUnsupported, SupportNative, ClientTraits{
		InstallIntents: []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:  LifecycleCLIRegistry,
	}), LifecycleNativeConfig),
	withProjectScope(withoutCommands(clientDefinition(ClientCopilot, "GitHub Copilot CLI", "github-copilot", "managed", "native", true, PackageNative, SupportNative, SupportNative, SupportNative, SupportUnsupported, SupportNative, ClientTraits{
		DirectoryOwnGrant:          true,
		InstallIntents:             []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:              LifecycleCLIRegistry,
		RecordsUserProfileOnCreate: true,
	})), LifecycleNativeConfig),
	withProjectScope(withoutCommands(clientDefinition(ClientVSCode, "Visual Studio Code", "github-copilot", "prepared", "prepared", true, PackagePrepared, SupportPrepared, SupportPrepared, SupportPrepared, SupportUnsupported, SupportPrepared, ClientTraits{
		SavedDependenciesDeferred: true,
		InstallIntents:            []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:             LifecycleCLIRegistry,
	})), LifecycleNativeConfig),
	withProjectScope(clientDefinition(ClientKiro, "Kiro", "kiro", "managed", "native", true, PackageNative, SupportNative, SupportNative, SupportNative, SupportUnsupported, SupportUnsupported, ClientTraits{
		InstallIntents:      []InstallIntent{InstallIntentAutomatic, InstallIntentPrepare},
		LifecycleKind:       LifecycleCLIRegistry,
		TracksNativeEffects: true,
	}), LifecycleNativeConfig),
	withProjectScope(withActivation(clientDefinition(ClientClaude, "Claude Code", "claude", "managed", "projected", false, PackageProjection, SupportProjected, SupportProjected, SupportProjected, SupportUnsupported, SupportUnsupported, ClientTraits{
		QualifiedLocalInstallerFacade: true,
		InstallIntents:                []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:                 LifecycleCLIRegistry,
		UsesManagedStdioLauncher:      true,
		ProtectsModifiedUserRawSkill:  true,
	}), ActivationAutomatic), LifecycleCLIRegistry),
	withProjectScope(clientDefinition(ClientGemini, "Gemini CLI", "gemini", "managed", "native", false, PackageNative, SupportNative, SupportNative, SupportNative, SupportUnsupported, SupportUnsupported, ClientTraits{
		InstallIntents:         []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:          LifecycleNativeConfig,
		BindsNativeProfileRoot: true,
	}), LifecycleNativeConfig),
	withProjectScope(withActivation(clientDefinition(ClientOpenCode, "OpenCode", "opencode", "managed", "prepared", false, PackagePrepared, SupportPrepared, SupportPrepared, SupportUnsupported, SupportUnsupported, SupportUnsupported, ClientTraits{
		InstallIntents:                   []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:                    LifecycleNativeConfig,
		ReportsMCPToolNamespaceCollision: true,
	}), ActivationAutomatic), LifecycleNativeConfig),
	withProjectScope(withActivation(clientDefinition(ClientCline, "Cline", "cline", "managed", "native", false, PackageNative, SupportNative, SupportNative, SupportNative, SupportUnsupported, SupportUnsupported, ClientTraits{
		InstallIntents: []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:  LifecycleNativeConfig,
	}), ActivationAutomatic), LifecycleNativeConfig),
	withProjectScope(clientDefinition(ClientWindsurf, "Windsurf / Devin", "windsurf", "prepared", "prepared", false, PackagePrepared, SupportPrepared, SupportPrepared, SupportPrepared, SupportUnsupported, SupportPrepared, ClientTraits{
		InstallIntents:                    []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:                     LifecycleNativeConfig,
		UsesManagedStdioLauncher:          true,
		RecordsSelectedNativeRootOnCreate: true,
	}), LifecycleNativeConfig),
	withProjectComponentSupport(withProjectScope(clientDefinition(ClientGrok, "Grok Build", "grok", "managed", "native", false, PackageNative, SupportNative, SupportNative, SupportUnsupported, SupportUnsupported, SupportUnsupported, ClientTraits{
		InstallIntents: []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:  LifecycleCLIRegistry,
	}), LifecyclePrepared), SupportNative, SupportNative),
	withProjectScope(clientDefinition(ClientKimi, "Kimi Code", "kimi", "managed", "native", false, PackageNative, SupportNative, SupportNative, SupportUnsupported, SupportUnsupported, SupportUnsupported, ClientTraits{
		InstallIntents: []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:  LifecycleNativeConfig,
	}), LifecycleNativeConfig),
	withProjectComponents(withProjectScope(clientDefinition(ClientQoder, "Qoder", "qoder", "managed", "native", false, PackageNative, SupportNative, SupportNative, SupportUnsupported, SupportUnsupported, SupportUnsupported, ClientTraits{
		InstallIntents:                      []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:                       LifecycleCLIRegistry,
		BindsNativeProfileRoot:              true,
		DiscoversNativeHooks:                true,
		HonorsOpenAIMCPAuthHints:            true,
		LegacyPreparedProjectPackageReceipt: true,
	}), LifecycleNativeConfig), SupportNative, SupportNative),
	withProjectComponents(withProjectScope(clientDefinition(ClientHermes, "Hermes Agent", "hermes", "managed", "native", false, PackageNative, SupportNative, SupportNative, SupportUnsupported, SupportUnsupported, SupportUnsupported, ClientTraits{
		InstallIntents:           []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:            LifecycleNativeConfig,
		BindsNativeProfileRoot:   true,
		TracksNativeEffects:      true,
		SupportsPreparedRecovery: true,
	}), LifecycleNativeConfig), SupportNative, SupportUnsupported),
	withProjectScope(clientDefinition(ClientAntigravity, "Google Antigravity", "antigravity", "managed", "native", false, PackageNative, SupportNative, SupportNative, SupportUnsupported, SupportUnsupported, SupportUnsupported, ClientTraits{
		InstallIntents: []InstallIntent{InstallIntentAutomatic},
		LifecycleKind:  LifecyclePrepared,
	}), LifecyclePrepared),
	withProjectComponents(withProjectScope(withActivation(withoutCommands(clientDefinition(ClientPi, "Pi", "pi", "managed", "native", false, PackageNative, SupportNative, SupportNative, SupportUnsupported, SupportUnsupported, SupportNative, ClientTraits{
		InstallIntents: []InstallIntent{InstallIntentAutomatic}, LifecycleKind: LifecyclePrepared,
		BindsNativeProfileRoot: true, RequiresPluginData: true, VerifiesNativeDirectory: true, ProtectsModifiedPackage: true,
		RequiresActiveNativeProjection: true,
	})), ActivationAutomatic), LifecyclePrepared), SupportNative, SupportNative),
}

func withActivation(definition ClientDefinition, activation ActivationMode) ClientDefinition {
	definition.Capabilities.ActivationMode = activation
	return definition
}

// withoutCommands withholds portable commands from a client that otherwise
// takes skills. The github-copilot family carries it while Visual Studio Code
// is frozen by the owner decision of 2026-10-16: VS Code installs through the
// package it shares with Copilot CLI, so neither may get commands. Removing it
// from both entries delivers commands to the family again.
func withoutCommands(definition ClientDefinition) ClientDefinition {
	definition.CommandSupport = SupportUnsupported
	return definition
}

func withoutHostPresence(definition ClientDefinition) ClientDefinition {
	definition.PlansWithoutHostPresence = true
	return definition
}

func withDirectoryPreparation(definition ClientDefinition, purpose DirectoryResolvePurpose) ClientDefinition {
	definition.DirectoryPreparationPurpose = purpose
	return definition
}

func clientDefinition(id ClientID, displayName, backendFamily, delivery, catalogPackage string, legacyRequired bool, packageMode PackageMode, skill, mcp, sse, app, extension SupportLevel, traits ClientTraits) ClientDefinition {
	return ClientDefinition{
		ID: id, DisplayName: displayName, BackendFamily: backendFamily,
		DirectoryDelivery: delivery, CatalogPackage: catalogPackage, LegacyCatalogRequired: legacyRequired,
		Capabilities: ClientCapabilities{
			ClientID: id, PackageMode: packageMode, ActivationMode: ActivationByUser,
			Scopes: []InstallScope{ScopeUser}, SkillSupport: skill,
			MCPTransports: map[string]SupportLevel{"stdio": mcp, "streamable-http": mcp, "sse": sse},
			AppSupport:    app, ExtensionSupport: extension,
		},
		Traits: traits,
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
		result[index].Traits.InstallIntents = append([]InstallIntent(nil), definition.Traits.InstallIntents...)
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

var clientAliases = map[string]ClientID{
	"github-copilot":  ClientCopilot,
	"vs-code":         ClientVSCode,
	"claude-code":     ClientClaude,
	"gemini-cli":      ClientGemini,
	"open-code":       ClientOpenCode,
	"devin":           ClientWindsurf,
	"grok-build":      ClientGrok,
	"kimi-code":       ClientKimi,
	"qoder-cli":       ClientQoder,
	"qoder-desktop":   ClientQoder,
	"chatgpt-desktop": ClientCodex,
	"chatgpt-web":     ClientChatGPT,
	"chatgptweb":      ClientChatGPT,
}

// ParseClientID folds case and whitespace, maps known aliases onto canonical
// ids, and otherwise returns the lowercased value. Unknown names are not
// rejected here so later diagnostics can name the supported targets.
func ParseClientID(value string) (ClientID, bool) {
	normalized := ClientID(strings.ToLower(strings.TrimSpace(value)))
	if aliased, ok := clientAliases[string(normalized)]; ok {
		return aliased, true
	}
	if IsSupportedClient(normalized) {
		return normalized, true
	}
	return normalized, false
}

// PublicTargetFlagName is the user-facing --target spelling for a canonical
// client ID. Signed Directory and state receipts keep their canonical IDs.
// A Web recovery command must never silently select the local Desktop backend.
func PublicTargetFlagName(id ClientID) string {
	if id == ClientChatGPT {
		return "chatgptweb"
	}
	return string(id)
}

// BackendSiblings returns the other clients installed through the same backend,
// in registry order. Copilot and VS Code are the only pair today: a package
// installed through the Copilot CLI is what VS Code then discovers.
func BackendSiblings(id ClientID) []ClientID {
	definition, ok := ClientDefinitionFor(id)
	if !ok {
		return nil
	}
	siblings := make([]ClientID, 0, len(clientDefinitions))
	for _, candidate := range clientDefinitions {
		if candidate.ID == id || candidate.BackendFamily != definition.BackendFamily {
			continue
		}
		siblings = append(siblings, candidate.ID)
	}
	return siblings
}

func ClientDisplayName(id ClientID) string {
	if definition, ok := ClientDefinitionFor(id); ok && definition.DisplayName != "" {
		return definition.DisplayName
	}
	return string(id)
}

func SameClientBackend(first, second ClientID) bool {
	if first == second {
		return true
	}
	firstDefinition, firstOK := ClientDefinitionFor(first)
	secondDefinition, secondOK := ClientDefinitionFor(second)
	return firstOK && secondOK && firstDefinition.BackendFamily == secondDefinition.BackendFamily
}
