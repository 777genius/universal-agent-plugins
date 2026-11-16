package opencode

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

var _ clients.PreparedSetupCompleter = (*Adapter)(nil)

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

// The transient action is produced only by this attempt's exact local MCP
// configuration and Skill checks. It is not persisted and grants no authority:
// the existing core repair path must still recheck intact ownership and obtain
// its ordinary confirmation before native MCP startup.
func (*Adapter) CanCompletePreparedSetup(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	if request.Plan.InstallIntent != domain.InstallIntentAutomatic ||
		observed.Activation != domain.ActivationPrepared || observed.Verification != domain.VerificationInstalled ||
		observed.Policy != domain.PolicyAllowed || observed.NativeEffect != domain.NativeEffectUnchanged ||
		!slices.Contains(observed.UserActions, openCodeMCPConfirmationAction) ||
		!openCodeV1DiscoverySupported(request) || !filepath.IsAbs(request.Client.ExecutablePath) {
		return false
	}
	_, servers, supported := openCodeObservableComponents(request.Plan.Components)
	if !supported || len(servers) == 0 || request.Plan.ActivePath != request.Delivery.ActivePath || !filepath.IsAbs(request.Plan.ActivePath) {
		return false
	}
	_, err := openCodeActivationLayout(request)
	return err == nil
}
