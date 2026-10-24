package domain

import "strings"

// LifecycleKind is how the installer drives a client's host after a package is
// on disk. It is policy, not a serialized capability: JSON ClientCapabilities
// stay the public compatibility contract.
type LifecycleKind string

const (
	LifecycleCLIRegistry  LifecycleKind = "cli_registry"
	LifecycleNativeConfig LifecycleKind = "native_config"
	LifecycleManual       LifecycleKind = "manual"
	LifecyclePrepared     LifecycleKind = "prepared"
)

// ClientTraits are the declarative installer policies that used to hide behind
// ClientID switches. They are not serialized; a new client is added by filling
// this struct on its ClientDefinition row, not by teaching usecase another id.
type ClientTraits struct {
	// SavedDependenciesDeferred records owner qualification scope, not SDK or
	// imperative adapter availability. Existing registered APIs remain usable.
	SavedDependenciesDeferred bool
	// QualifiedLocalInstallerFacade keeps local-file route qualification
	// separate from saved-dependency support; broad registration cannot widen it.
	QualifiedLocalInstallerFacade     bool
	InstallIntents                    []InstallIntent
	LifecycleKind                     LifecycleKind
	UsesManagedStdioLauncher          bool
	HonorsOpenAIMCPAuthHints          bool
	SupportsPreparedRecovery          bool
	RequiresPersonalMappingForPrepare bool
	ReportsMCPToolNamespaceCollision  bool
	BindsNativeProfileRoot            bool
	TracksNativeEffects               bool
	// EnforcesExplicitSkillInvocation means the client has a verified way to
	// keep a skill out of the model's automatic choice, so an explicit-only
	// command can be delivered. The adapter must then implement
	// clients.ExplicitSkillDecorator. Backend siblings must agree.
	EnforcesExplicitSkillInvocation bool
	// DiscoversNativeHooks means staging must carry the package-root hooks/
	// and commands/ surfaces through to the delivered tree: the client
	// discovers them there by convention (Qoder plugin layout) instead of
	// through the portable hooks contract other clients cannot consume.
	DiscoversNativeHooks bool
}

// ClientTraitsFor returns the declared traits for a known client. Unknown ids
// yield the zero value, which admits no prepare intent and no native lifecycle.
func ClientTraitsFor(id ClientID) ClientTraits {
	definition, ok := ClientDefinitionFor(id)
	if !ok {
		return ClientTraits{}
	}
	return definition.Traits
}

// RequiresNativeProjector reports whether staging this client must produce
// native object ownership through a Projector. A missing projector is
// fail-closed: empty native objects would look like a successful stage.
func RequiresNativeProjector(id ClientID) bool {
	definition, ok := ClientDefinitionFor(id)
	if !ok {
		return false
	}
	if definition.Capabilities.PackageMode == PackageNative || definition.Traits.LifecycleKind == LifecycleNativeConfig {
		return true
	}
	return id == ClientVSCode
}

func RequiresNativeProjectorForScope(id ClientID, scope InstallScope) bool {
	if scope != ScopeProject {
		return RequiresNativeProjector(id)
	}
	definition, ok := ClientDefinitionFor(id)
	return ok && definition.ProjectPolicy.Valid()
}

// Allows reports whether this client's table lists the intent. Validate still
// accepts historical empty automatic intent even when the slice omits it;
// callers that need that exception must go through Validate.
func (traits ClientTraits) Allows(intent InstallIntent) bool {
	for _, allowed := range traits.InstallIntents {
		if allowed == intent {
			return true
		}
	}
	return false
}

// SharesBackend is the two-argument form of a shared physical backend. Copilot
// and VS Code are the only pair today; callers should not name them.
func SharesBackend(first, second ClientID) bool {
	return SameClientBackend(first, second)
}

// ShouldReadOnlyVerify reports whether verify-only Activate is meaningful for
// this client given the host executable and install intent. The branches are
// the declared traits, not a ClientID list: native-config always, CLI-registry
// hosts that expose a managed launcher or OpenAI auth hints or a sibling
// backend when an executable is present, and prepare-capable hosts that are
// not ChatGPT-style personal mapping when the intent is prepare or the
// executable names the client.
func ShouldReadOnlyVerify(id ClientID, executable string, intent InstallIntent) bool {
	return ShouldReadOnlyVerifyForScope(id, ScopeUser, executable, intent)
}

func ShouldReadOnlyVerifyForScope(id ClientID, scope InstallScope, executable string, intent InstallIntent) bool {
	traits := (SelectedDelivery{}).EffectiveTraitsForScope(id, scope)
	switch {
	case traits.LifecycleKind == LifecycleNativeConfig:
		return true
	case traits.LifecycleKind == LifecycleCLIRegistry && traits.UsesManagedStdioLauncher:
		return strings.TrimSpace(executable) != ""
	case traits.HonorsOpenAIMCPAuthHints:
		return strings.TrimSpace(executable) != ""
	case len(BackendSiblingsForScope(id, scope)) > 0:
		return strings.TrimSpace(executable) != ""
	case traits.Allows(InstallIntentPrepare) && !traits.RequiresPersonalMappingForPrepare:
		if intent == InstallIntentPrepare {
			return true
		}
		return strings.Contains(strings.ToLower(executable), string(id))
	default:
		return false
	}
}
