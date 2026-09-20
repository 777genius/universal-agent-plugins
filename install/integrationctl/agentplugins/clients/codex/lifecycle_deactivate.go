package codex

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Deactivate removes the managed Codex plugin and marketplace after the
// operator confirms, or records the manual steps when the CLI is missing.
func (*Adapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	if !request.ExternalUninstalled {
		return shared.RequireExternalUninstall(outcome, false, "uninstall the plugin in Codex, then rerun remove with `--external-uninstalled` (also use the flag if it was never activated)"), nil
	}
	if !request.Confirmed {
		return shared.RequireExternalUninstall(outcome, true, ""), nil
	}
	if strings.TrimSpace(request.PhysicalArtifactID) == "" {
		return outcome, fmt.Errorf("managed Codex marketplace identity is missing")
	}
	return deactivateConfirmedCodex(ctx, env, request, outcome)
}

func deactivateConfirmedCodex(ctx context.Context, env clients.Env, request domain.DeactivationRequest, outcome domain.DeactivationOutcome) (domain.DeactivationOutcome, error) {
	marketplace := shared.ManagedMarketplaceName(request.PhysicalArtifactID)
	registered, err := ManagedCodexMarketplaceRegistered(request.Client.ConfigRoot, marketplace, request.ManagedArtifactPath)
	if err != nil {
		return outcome, err
	}
	pluginEntryPresent, err := ManagedCodexPluginEntryPresent(request.Client.ConfigRoot, request.DeclaredName, marketplace)
	if err != nil {
		return outcome, err
	}
	if !shared.HasClientCLI(env, request.BackendExecutable) {
		return deactivateCodexWithoutCLI(outcome, request, marketplace, registered, pluginEntryPresent)
	}
	profileRoot := request.Client.ConfigRoot
	if err := validateProfile(profileRoot, ""); err != nil {
		return outcome, err
	}
	request.Client.ConfigRoot = profileRoot
	if err := removeCodexPlugin(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, request.DeclaredName+"@"+marketplace); err != nil {
		return outcome, err
	}
	if registered {
		if err := removeCodexMarketplace(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, marketplace); err != nil {
			return outcome, err
		}
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}

func deactivateCodexWithoutCLI(outcome domain.DeactivationOutcome, request domain.DeactivationRequest, marketplace string, registered, pluginEntryPresent bool) (domain.DeactivationOutcome, error) {
	if !registered && !pluginEntryPresent {
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	}
	outcome.Activation = domain.ActivationManual
	outcome.ArtifactRemovalAllowed = false
	outcome.UserActions = []string{fmt.Sprintf("run `codex plugin remove %s@%s --json`, then `codex plugin marketplace remove %s --json`, then retry removal", request.DeclaredName, marketplace, marketplace)}
	return outcome, nil
}

func removeCodexMarketplace(ctx context.Context, env clients.Env, root, executable, marketplace string) error {
	remove, err := runCodex(ctx, env, root, executable, "plugin", "marketplace", "remove", marketplace, "--json")
	if err != nil && !shared.CommandOutputContains(remove, "not configured or installed") {
		return fmt.Errorf("remove managed Codex marketplace %s: %w", marketplace, err)
	}
	return nil
}

func removeCodexPlugin(ctx context.Context, env clients.Env, root, executable, pluginSpec string) error {
	remove, err := runCodex(ctx, env, root, executable, "plugin", "remove", pluginSpec, "--json")
	if err != nil && !shared.CommandOutputContains(remove, "not configured or installed") {
		return fmt.Errorf("remove managed Codex plugin %s: %w", pluginSpec, err)
	}
	return nil
}
