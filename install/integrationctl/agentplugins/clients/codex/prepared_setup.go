package codex

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PreparedSetupCompleter = (*Adapter)(nil)

func (*Adapter) CanCompletePreparedSetup(request domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	return observed.Policy == domain.PolicyAllowed && request.Plan.PackageMode == domain.PackageProjection &&
		request.Plan.ActivePath == request.Delivery.ActivePath && cleanAbsolute(request.Plan.ActivePath) &&
		filepath.Dir(request.Plan.NativeRegistryRoot) != request.Plan.NativeRegistryRoot &&
		observed.Activation == domain.ActivationPrepared && observed.Verification == domain.VerificationInstalled &&
		request.Plan.Scope == domain.ScopeProject && request.Plan.TargetContext.Scope == domain.ScopeProject &&
		request.Plan.InstallIntent == domain.InstallIntentAutomatic &&
		request.Plan.NativeRegistryRoot == request.Plan.TargetContext.ProjectRoot &&
		request.Client.ConfigRoot == request.Plan.TargetContext.UserConfigRoot &&
		cleanAbsolute(request.Plan.NativeRegistryRoot) && cleanAbsolute(request.Client.ConfigRoot) &&
		shared.OnlyNativeComponents(request.Plan.Components) && cleanAbsolute(request.BackendExecutable)
}
