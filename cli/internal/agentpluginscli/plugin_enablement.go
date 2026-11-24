package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func validatePluginEnablementSelection(cmd *cobra.Command, app App, opts *options) error {
	if !opts.confirmPluginEnable {
		return nil
	}
	if !cmd.Root().PersistentFlags().Changed("scope") || !cmd.Root().PersistentFlags().Changed("target") || opts.scope != "project" || strings.Contains(opts.targetInput, ",") || strings.TrimSpace(opts.target) == "" {
		return fmt.Errorf("--confirm-plugin-enable requires one explicit --target and --scope project")
	}
	targets, err := parseTargetOption(opts.target)
	if err != nil {
		return err
	}
	if len(targets) != 1 || !clients.SupportsPluginEnablement(app.ClientRegistry, targets[0], domain.ScopeProject) {
		return fmt.Errorf("selected target does not support explicit Project plugin enablement")
	}
	return nil
}

// Explicit enablement needs the selected executable's actual version even for
// a preview. Ordinary discovery and repair remain free of version execution.
func selectedPluginEnablementVersion(ctx context.Context, app App, opts *options, client domain.DetectedClient) (domain.DetectedClient, error) {
	if !opts.confirmPluginEnable || opts.scope != "project" || strings.TrimSpace(client.Version) != "" || !clients.SupportsPluginEnablement(app.ClientRegistry, client.ClientID, domain.ScopeProject) {
		return client, nil
	}
	return probeSelectedClientVersion(ctx, app, client)
}

func probeSelectedClientVersion(ctx context.Context, app App, client domain.DetectedClient) (domain.DetectedClient, error) {
	targeted, ok := app.Detector.(ports.TargetedVersionProbingClientDetector)
	if !ok {
		// Preserve the adapter's exact-version refusal; never probe every client.
		return client, nil
	}
	probed, err := targeted.DetectTargetsWithVersionProbe(ctx, []domain.ClientID{client.ClientID})
	if err != nil {
		return client, fmt.Errorf("selected native lifecycle version detection: %w", err)
	}
	version, err := selectedProjectVersion(client, probed)
	if err != nil {
		return client, err
	}
	client.Version = version
	return client, nil
}

func selectRepairClient(ctx context.Context, cmd *cobra.Command, app App, opts *options, installation domain.Installation, detected []domain.DetectedClient, reader io.Reader) (domain.DetectedClient, map[domain.ClientID]domain.DetectedClient, error) {
	selected, observed, err := selectBoundClient(cmd, app, opts, installation, detected, true, reader)
	if err != nil {
		return selected, observed, err
	}
	if _, err := selectedRepairBinding(installation, selected.ClientID, domain.InstallScope(opts.scope)); err != nil {
		return selected, observed, err
	}
	selected, err = selectedPluginEnablementVersion(ctx, app, opts, selected)
	if err == nil {
		observed[selected.ClientID] = selected
	}
	return selected, observed, err
}

func (adapter *dependencyInstaller) resolvePreparedDependencyClient(ctx context.Context, view installer.Inspection, action *environment.PreparedBinding, client *domain.DetectedClient) error {
	if err := resolveDependencyBindingIdentity(view, action); err != nil {
		return err
	}
	if action.Action != "repair" {
		return nil
	}
	selected, err := selectedPluginEnablementVersion(ctx, adapter.app, adapter.opts, *client)
	if err != nil {
		return err
	}
	*client = selected
	adapter.detected[selected.ClientID] = selected
	return nil
}

func resolveDependencyBindingIdentity(view installer.Inspection, action *environment.PreparedBinding) error {
	for _, receipt := range view.EnvironmentReceipts {
		if receipt.EnvironmentID == action.Authority.EnvironmentID && receipt.Alias == action.Alias {
			if action.InstallationID == "" {
				action.InstallationID = receipt.InstallationID
				action.Authority.InstallationID = receipt.InstallationID
			}
			if receipt.ClientID == action.ClientID && action.Action == environment.ActionInstall {
				return installer.ErrPlanChanged
			}
		}
	}
	return nil
}
