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
	InstallIntents                    []InstallIntent
	LifecycleKind                     LifecycleKind
	UsesManagedStdioLauncher          bool
	HonorsOpenAIMCPAuthHints          bool
	SupportsPreparedRecovery          bool
	RequiresPersonalMappingForPrepare bool
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
	traits := ClientTraitsFor(id)
	switch {
	case traits.LifecycleKind == LifecycleNativeConfig:
		return true
	case traits.LifecycleKind == LifecycleCLIRegistry && traits.UsesManagedStdioLauncher:
		return strings.TrimSpace(executable) != ""
	case traits.HonorsOpenAIMCPAuthHints:
		return strings.TrimSpace(executable) != ""
	case len(BackendSiblings(id)) > 0:
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
