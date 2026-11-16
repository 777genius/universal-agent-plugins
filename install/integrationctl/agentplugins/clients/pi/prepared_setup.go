package pi

import (
	"path/filepath"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PreparedSetupCompleter = (*ProjectAdapter)(nil)

// CanCompletePreparedSetup admits only an intact project installation whose
// fresh read-only verification reported that its trust record is missing,
// such as one installed before UAP set up Pi project trust. It performs no
// I/O; the core's confirmed repair rechecks ownership before activation.
func (*ProjectAdapter) CanCompletePreparedSetup(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	if observed.Activation != domain.ActivationPrepared || observed.Verification != domain.VerificationInstalled || observed.Policy != domain.PolicyAllowed || observed.ActivationAttested ||
		request.Client.ClientID != domain.ClientPi || request.Plan.ClientID != domain.ClientPi || request.Plan.PackageMode != domain.PackageNative ||
		request.Plan.Scope != domain.ScopeProject || request.Plan.TargetContext.Scope != domain.ScopeProject || request.Plan.InstallIntent != domain.InstallIntentAutomatic {
		return false
	}
	project := request.Plan.TargetContext.ProjectRoot
	anchor := filepath.Join(project, ".pi")
	if cleanRoot(project) != nil || cleanRoot(request.Client.ConfigRoot) != nil || request.Client.ConfigRoot != request.Plan.TargetContext.UserConfigRoot ||
		request.Plan.NativeRegistryRoot != anchor || request.Plan.TargetAnchor != anchor || request.Plan.TargetRoot != filepath.Join(anchor, "extensions") ||
		filepath.Dir(request.Plan.ActivePath) != request.Plan.TargetRoot || request.Delivery.ActivePath != request.Plan.ActivePath {
		return false
	}
	return slices.Contains(observed.UserActions, piProjectTrustPendingAction)
}
