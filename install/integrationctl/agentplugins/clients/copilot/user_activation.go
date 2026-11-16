package copilot

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// activateCopilotUserPlugin registers the managed directory marketplace and
// installs its plugin through the Copilot CLI, then verifies the listing.
//
// Copilot loads directory marketplaces live: `plugin update` exits 0 with
// "there is nothing to update" even when the plugin is disabled or was never
// installed, while `plugin install` is idempotent and (re)enables it (observed
// with Copilot CLI 1.0.112). The shared Copilot/VS Code flow treats a
// successful `plugin update` as installed and skips `plugin install`, so a
// retry after a failed first install left the plugin registered but disabled.
// This lane therefore always installs after refreshing the marketplace.
func activateCopilotUserPlugin(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	run := func(args ...string) error {
		result, err := shared.RunClientCommand(ctx, env.Runner, shared.CopilotCLIName, request.BackendExecutable, args...)
		return shared.NativeProbeFailure(result, err)
	}
	register := []string{"plugin", "marketplace", "add", request.Delivery.ActivePath}
	refresh := []string{"plugin", "marketplace", "update", marketplace}
	first, fallback := register, refresh
	if request.Replacing {
		first, fallback = refresh, register
	}
	if err := run(first...); err != nil {
		if domain.IsUnclosedClientProbe(err) {
			return err
		}
		if fallbackErr := run(fallback...); fallbackErr != nil {
			return fmt.Errorf("register managed Copilot marketplace: %w; fallback: %w", err, fallbackErr)
		}
	}
	if err := run("plugin", "install", request.DeclaredName+"@"+marketplace); err != nil {
		if !request.Replacing && !domain.IsUnclosedClientProbe(err) {
			// A fresh registration is rolled back; a replacement keeps the
			// previously working marketplace for repair.
			if rollbackErr := run("plugin", "marketplace", "remove", marketplace); rollbackErr != nil {
				return fmt.Errorf("install managed Copilot plugin: %w; rollback marketplace: %w", err, rollbackErr)
			}
		}
		return err
	}
	return shared.VerifyCopilotPluginWithListingRecognition(ctx, env, request, recognizeCopilotUserActivation)
}
