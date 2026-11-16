package agentpluginscli

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func selectedRemovalVersion(ctx context.Context, app App, client domain.DetectedClient, binding domain.ClientBinding) (domain.DetectedClient, error) {
	requirement, ok := clients.As[clients.RemovalVersionRequirement](app.ClientRegistry, client.ClientID)
	if !ok || strings.TrimSpace(client.Version) != "" || !requirement.RequiresRemovalVersion(binding) {
		return client, nil
	}
	if binding.ClientID != string(client.ClientID) || binding.Scope != string(app.scope()) {
		return client, installer.ErrPlanChanged
	}
	return probeSelectedClientVersion(ctx, app, client)
}

// The ordinary removal plan has already validated the selected binding. Only
// its confirmed apply obtains missing version evidence; previews remain inert.
func prepareRemovalInputVersion(ctx context.Context, app App, installation domain.Installation, input *usecase.RemoveInput, service *usecase.Service) error {
	if _, ok := clients.As[clients.RemovalVersionRequirement](app.ClientRegistry, input.Client.ClientID); !ok || strings.TrimSpace(input.Client.Version) != "" {
		return nil
	}
	binding, err := selectedRepairBinding(installation, input.Client.ClientID, input.Scope)
	if err != nil {
		return err
	}
	selected, err := selectedRemovalVersion(ctx, app, input.Client, binding)
	if err != nil {
		return err
	}
	input.Client = selected
	service.Detected[selected.ClientID] = selected
	return nil
}

func removeWithSelectedVersion(ctx context.Context, app App, installation domain.Installation, input usecase.RemoveInput, service usecase.Service) (usecase.RemoveResult, error) {
	if input.Confirmed && !input.DryRun {
		if err := prepareRemovalInputVersion(ctx, app, installation, &input, &service); err != nil {
			return usecase.RemoveResult{}, err
		}
	}
	return service.Remove(ctx, input)
}

// Recheck runs under the existing environment lock, after acceptance and before
// desired files are published. Preserve that check before measuring any client.
func (adapter *dependencyInstaller) removalVersionRecheck(check func(context.Context) error, actions []dependencyAction) func(context.Context) error {
	if check == nil {
		return nil
	}
	return func(ctx context.Context) error {
		if err := check(ctx); err != nil {
			return err
		}
		for _, action := range actions {
			if err := adapter.prepareRemovalVersion(ctx, action); err != nil {
				return err
			}
		}
		return nil
	}
}

func (adapter *dependencyInstaller) prepareRemovalVersion(ctx context.Context, action dependencyAction) error {
	client := adapter.detected[domain.ClientID(action.ClientID)]
	if action.Action != environment.ActionRemove || strings.TrimSpace(client.Version) != "" {
		return nil
	}
	if _, ok := clients.As[clients.RemovalVersionRequirement](adapter.app.ClientRegistry, client.ClientID); !ok {
		return nil
	}
	state, err := adapter.app.StateStore.Load()
	if err != nil {
		return err
	}
	installation, err := selectInstallation(state, action.InstallationID)
	if err != nil {
		return err
	}
	binding, ok := installation.Clients[action.BindingID]
	if installation.InstallationID != action.InstallationID || !ok || binding.ClientBindingID != action.BindingID || binding.ClientID != action.ClientID || binding.Scope != string(adapter.app.scope()) {
		return installer.ErrPlanChanged
	}
	selected, err := selectedRemovalVersion(ctx, adapter.app, client, binding)
	if err != nil {
		return err
	}
	adapter.detected[selected.ClientID] = selected
	return nil
}

func (session *removeManySession) prepareRemovalVersions() error {
	for index := range session.inputs {
		if err := prepareRemovalInputVersion(session.ctx, session.app, session.installation, &session.inputs[index], &session.service); err != nil {
			return err
		}
	}
	return nil
}
