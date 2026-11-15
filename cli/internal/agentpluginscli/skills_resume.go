package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// An explicit selected Skills install can finish an exact owned preparation.
// Every effect is an existing SDK operation shown before confirmation. Ordinary
// dependency sync keeps its non-retrying policy for prepared bindings.
func prepareSkillsResumes(ctx context.Context, adapter *dependencyInstaller, view installer.Inspection, actions []dependencyAction, paths map[string]string) error {
	for i, action := range actions {
		binding, exact := unchangedSkillsBinding(view, action, adapter.app.environmentID(), string(adapter.app.scope()), paths[action.Alias+"\x00"+action.ClientID])
		if !exact || !skillsResumeCandidate(binding) {
			continue
		}
		resumed, plan, err := prepareSkillsResume(ctx, adapter, action)
		if err != nil {
			return &dependencyPreflightError{Alias: action.Alias, ClientID: action.ClientID, Cause: err}
		}
		// Unknown authentication alone never authorizes a mutation. The current
		// native planner must positively admit credential-free installation.
		if binding.Activation == string(domain.ActivationActive) && plan.Delivery.Authentication != string(domain.AuthenticationNotRequired) {
			continue
		}
		actions[i] = resumed
	}
	return nil
}

func skillsResumeCandidate(binding installer.InspectedBinding) bool {
	if binding.Materialization != string(domain.MaterializationMaterialized) || binding.Policy != string(domain.PolicyAllowed) {
		return false
	}
	return binding.Activation == string(domain.ActivationPrepared) && !binding.LifecycleConverged ||
		binding.Activation == string(domain.ActivationActive) && binding.Authentication == string(domain.AuthenticationNotChecked)
}

func prepareSkillsResume(ctx context.Context, adapter *dependencyInstaller, original dependencyAction) (dependencyAction, installer.Plan, error) {
	action := managementNativeRepair(original)
	// Legacy inventory hydration belongs only to the candidate being reviewed.
	pin := *action.Package
	action.Package = &pin
	action.Reason = "Finish setup for the installed Skill"
	if err := adapter.preflightProjectVersions(ctx, []dependencyAction{action}); err != nil {
		return action, installer.Plan{}, err
	}
	plan, err := prepareSkillsResumeRepair(ctx, adapter, action)
	if ctx.Err() != nil {
		return action, installer.Plan{}, ctx.Err()
	}
	if !skillsProjectionRefreshRequired(err) {
		return action, plan, err
	}
	// The same immutable source and repair receipt authorize the SDK's explicit
	// projection refresh. Its dry run independently requires intact owned bytes.
	// This is planning only; Apply never changes the reviewed operation.
	action.Action = environment.DesiredAction(installer.OpRefreshProjection)
	action.Reason = "Refresh the installed Skill projection and finish setup"
	prepared, err := adapter.Prepare(ctx, action)
	if err != nil {
		return action, installer.Plan{}, err
	}
	defer func() { _ = prepared.Close() }()
	return action, prepared.Plan(), nil
}

func prepareSkillsResumeRepair(ctx context.Context, adapter *dependencyInstaller, action dependencyAction) (installer.Plan, error) {
	prepared, err := adapter.Prepare(ctx, action)
	if err != nil {
		return installer.Plan{}, err
	}
	defer func() { _ = prepared.Close() }()
	projectionAndRegistrationComplete, err := adapter.app.Installer.VerifyPreparedProjection(ctx, prepared)
	if err != nil {
		return installer.Plan{}, err
	}
	if !projectionAndRegistrationComplete {
		// A prepared CLI registration can be incomplete while the immutable
		// package and repair authority remain valid. Keep the explicit repair
		// in review; only its accepted native result may qualify readiness.
		return prepared.Plan(), nil
	}
	return prepared.Plan(), nil
}

func skillsProjectionRefreshRequired(err error) bool {
	for err != nil {
		// A joined failure may include a foreign-object or cleanup error. Only
		// the single typed projection refusal selects another operation.
		if _, multiple := err.(interface{ Unwrap() []error }); multiple {
			return false
		}
		next := errors.Unwrap(err)
		if next == nil {
			return errors.Is(err, domain.ErrProjectionRefreshRequired) || errors.Is(err, domain.ErrProjectionPinChangeRequired)
		}
		err = next
	}
	return false
}
