package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Called by the accepted batch's recheck while the environment lock is held.
// A repeat may report saved readiness only after fresh native verification of
// that exact owned binding. Preparing an observation never applies a repair.
func (p *PreparedSkillsBatch) observeUnchangedSkills(ctx context.Context) (map[string]installer.ClientResult, error) {
	paths := map[string]string{}
	for _, action := range p.actions {
		if action.Action == string(environment.ActionUnchanged) {
			paths[skillsBindingKey(action.Alias, action.ClientID, action.Action)] = action.TargetPath
		}
	}
	if len(paths) == 0 {
		return nil, nil
	}
	view, err := p.app.Installer.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	if view.Recovery.Required {
		return nil, installer.ErrRecoveryRequired
	}
	adapter := &dependencyInstaller{app: p.app, opts: p.app.skillsOptions(), detected: p.request.detected, beforeApply: p.checkInputs}
	ready := map[string]installer.ClientResult{}
	for _, action := range p.request.prepared {
		key := skillsBindingKey(action.Alias, action.ClientID, string(action.Action))
		client, eligible := unchangedSkillsClient(view, action, p.app.environmentID(), string(p.app.scope()), paths[key])
		if !eligible {
			continue
		}
		// Repair preparation may hydrate a legacy executable inventory. Keep
		// that observation separate from the retained, already reviewed plan.
		pin := *action.Package
		action.Package = &pin
		coherent, err := managementNativeBindingCoherent(ctx, adapter, action)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, installer.ErrPlanChanged) || domain.IsUnclosedClientProbe(err) {
			return nil, errors.Join(ErrSkillsPlanStale, err)
		}
		if err == nil && coherent {
			ready[key] = client
		}
	}
	current, err := p.app.Installer.Inspect(ctx)
	if err != nil || current.Recovery.Required || current.Recovery.StateDigest != view.Recovery.StateDigest {
		return nil, errors.Join(ErrSkillsPlanStale, err)
	}
	return ready, nil
}

func unchangedSkillsClient(view installer.Inspection, action environment.PreparedBinding, environmentID, scope, target string) (installer.ClientResult, bool) {
	binding, exact := unchangedSkillsBinding(view, action, environmentID, scope, target)
	if !exact || !unchangedSkillsLifecycleReady(binding) {
		return installer.ClientResult{}, false
	}
	return installer.ClientResult{ClientID: binding.ClientID, BindingID: binding.BindingID, TreeDigest: binding.TreeDigest,
		Materialization: binding.Materialization, Activation: binding.Activation, Authentication: binding.Authentication,
		Policy: binding.Policy, Verification: binding.Verification}, true
}

func unchangedSkillsBinding(view installer.Inspection, action environment.PreparedBinding, environmentID, scope, target string) (installer.InspectedBinding, bool) {
	if action.Action != environment.ActionUnchanged || action.Package == nil || action.Authority == nil ||
		action.ActivationComplete || action.AuthComplete || target == "" || view.Recovery.Required {
		return installer.InspectedBinding{}, false
	}
	for _, installation := range view.Installations {
		if installation.InstallationID != action.InstallationID || installation.SourceKey != action.Package.SourceKey {
			continue
		}
		for _, binding := range installation.Bindings {
			if binding.BindingID != action.BindingID || binding.ClientID != action.ClientID || binding.Scope != scope ||
				binding.TargetPath != target || binding.TreeDigest != action.Package.TreeDigest || binding.ProjectionStatus != "" ||
				!unchangedSkillsOwner(binding.Owner, action, environmentID) {
				continue
			}
			return binding, true
		}
	}
	return installer.InspectedBinding{}, false
}

func unchangedSkillsLifecycleReady(binding installer.InspectedBinding) bool {
	return binding.Materialization == string(domain.MaterializationMaterialized) && binding.Policy == string(domain.PolicyAllowed) &&
		fullyInstalled(domain.ActivationOutcome{Activation: domain.ActivationState(binding.Activation),
			Authentication: domain.AuthenticationState(binding.Authentication), Verification: domain.VerificationState(binding.Verification)})
}

func unchangedSkillsOwner(owner *domain.EnvironmentReceipt, action environment.PreparedBinding, environmentID string) bool {
	authority := action.Authority
	return owner != nil && owner.ReceiptID != "" && owner.ReceiptID == authority.ReceiptID &&
		owner.EnvironmentID == environmentID && authority.EnvironmentID == environmentID &&
		owner.Alias == action.Alias && authority.Alias == action.Alias &&
		owner.ClientID == action.ClientID && authority.ClientID == action.ClientID &&
		owner.InstallationID == action.InstallationID && authority.InstallationID == action.InstallationID &&
		owner.ClientBindingID == action.BindingID && authority.ClientBindingID == action.BindingID &&
		owner.SourceKey == action.Package.SourceKey && authority.SourceKey == action.Package.SourceKey &&
		owner.PackageDigest == action.Package.TreeDigest && authority.ExpectedAction == string(environment.ActionUnchanged)
}
