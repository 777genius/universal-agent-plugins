package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Recovery presents every required native repair before acceptance. A package
// digest alone does not prove that copied Skills or native config entries exist.
func prepareManagementNativeRecovery(ctx context.Context, app App, actions []dependencyAction) ([]dependencyAction, error) {
	adapter, err := managementNativeAdapter(ctx, app)
	if err != nil {
		return nil, err
	}
	out := append([]dependencyAction(nil), actions...)
	for i, action := range out {
		if action.Action != environment.ActionUnchanged {
			continue
		}
		coherent, err := managementNativeBindingCoherent(ctx, adapter, action)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, installer.ErrPlanChanged) {
			return nil, management.ErrPlanStale
		}
		if !coherent || err != nil {
			out[i] = managementNativeRepair(action)
			out[i].Reason = "Verify and repair the recorded native materialization before resolving interrupted work"
		}
	}
	return out, nil
}

func verifyManagementNativeBindings(ctx context.Context, app App, actions []dependencyAction) (bool, error) {
	adapter, err := managementNativeAdapter(ctx, app)
	if err != nil {
		return false, err
	}
	for _, action := range actions {
		coherent, err := managementNativeBindingCoherent(ctx, adapter, action)
		if err != nil || !coherent {
			return false, err
		}
	}
	return true, nil
}

func managementNativeAdapter(ctx context.Context, app App) (*dependencyInstaller, error) {
	detected, err := detectDependencyClients(ctx, app)
	if err != nil {
		return nil, err
	}
	return &dependencyInstaller{app: app, detected: detected, opts: &options{dryRun: true}}, nil
}

func managementNativeRepair(action dependencyAction) dependencyAction {
	action.Action = "repair"
	if action.Authority != nil {
		authority := *action.Authority
		authority.ExpectedAction = "repair"
		action.Authority = &authority
	}
	return action
}

func managementNativeBindingCoherent(ctx context.Context, adapter *dependencyInstaller, action dependencyAction) (bool, error) {
	prepared, err := adapter.Prepare(ctx, managementNativeRepair(action))
	if err != nil {
		return false, err
	}
	defer func() { _ = prepared.Close() }()
	return adapter.app.Installer.VerifyPreparedBinding(ctx, prepared)
}
