package opencode

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

var (
	_ clients.PreparedSetupCompleter          = (*Adapter)(nil)
	_ clients.PreparedMaterializationVerifier = (*Adapter)(nil)
)

const openCodeMCPConfirmationAction = "confirm installation to start OpenCode and verify the installed local MCP servers"

// This is a command ABI gate, not an activation receipt. Native configuration,
// permission, exact Skills and selected MCP observations must still agree.
func openCodeV1DiscoverySupported(request domain.ActivationRequest) bool {
	host, ok := request.Plan.OpenCodeHost.(*opencodehost.NativePrepared)
	if !ok || host == nil {
		return false
	}
	profile := host.Profile()
	return profile.Family == opencodehost.ModernV1 && profile.ConfigDialect == opencodehost.DialectV1 &&
		opencodehost.ValidVersion(profile.Version) && !strings.ContainsAny(profile.Version, "-+")
}

// Classification describes the prospective acting request, not the read-only
// observation request that produced the transient action. The core separately
// requires fresh unchanged observation before asking this classifier.
// The transient action is produced only by this attempt's exact local MCP
// configuration and Skill checks. It is not persisted and grants no authority:
// the existing core repair path must still recheck intact ownership and obtain
// its ordinary confirmation before native MCP startup.
func (*Adapter) CanCompletePreparedSetup(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	if request.VerifyOnly || !openCodePreparedMCPObservation(request, observed) {
		return false
	}
	_, err := openCodeActivationLayout(request)
	return err == nil
}

// This method performs no filesystem reads. Freshness comes from the core's
// immediately preceding Activate call and complete owned-projection checks;
// the marker alone is not evidence for a historical or caller-authored packet.
func (*Adapter) PreparedMaterializationVerified(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	return request.VerifyOnly && !request.ActivationComplete &&
		!observed.ActivationAttested && !observed.AuthenticationAttested && !observed.AuthoritativeObservation &&
		observed.LocalEntryObservation == nil && len(request.PreviousNativeObjects) > 0 &&
		slices.Equal(request.PreviousNativeObjects, request.Delivery.NativeObjects) &&
		slices.Equal(request.PreviousNativeObjects, observed.NativeObjects) &&
		openCodePreparedMCPObservation(request, observed) && openCodeMaterializationContext(request)
}

func openCodePreparedMCPObservation(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	if request.Plan.InstallIntent != domain.InstallIntentAutomatic ||
		observed.Activation != domain.ActivationPrepared || observed.Verification != domain.VerificationInstalled ||
		observed.Policy != domain.PolicyAllowed || observed.NativeEffect != domain.NativeEffectUnchanged ||
		!slices.Contains(observed.UserActions, openCodeMCPConfirmationAction) ||
		!openCodeV1DiscoverySupported(request) || !filepath.IsAbs(request.Client.ExecutablePath) {
		return false
	}
	_, servers, supported := openCodeObservableComponents(request.Plan.Components)
	return supported && len(servers) > 0 && request.Plan.ActivePath == request.Delivery.ActivePath && filepath.IsAbs(request.Plan.ActivePath)
}

func openCodeMaterializationContext(request domain.ActivationRequest) bool {
	if request.Client.ClientID != domain.ClientOpenCode || request.Plan.ClientID != request.Client.ClientID ||
		request.Delivery.ClientID != request.Client.ClientID || request.Plan.Scope != request.Plan.TargetContext.Scope {
		return false
	}
	target, err := domain.NewTargetContext(request.Plan.TargetContext.Scope, request.Plan.TargetContext.ProjectRoot, request.Plan.TargetContext.UserConfigRoot)
	if err != nil || target != request.Plan.TargetContext || (target.Scope != domain.ScopeUser && target.Scope != domain.ScopeProject) ||
		(target.UserConfigRoot != "" && target.UserConfigRoot != request.Client.ConfigRoot) {
		return false
	}
	root := request.Client.ConfigRoot
	if target.Scope == domain.ScopeProject {
		root = target.ProjectRoot
	}
	return root != "" && filepath.IsAbs(root) && request.Plan.NativeRegistryRoot == root
}
