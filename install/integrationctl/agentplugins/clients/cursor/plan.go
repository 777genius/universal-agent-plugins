package cursor

import (
	"context"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.TargetLayout = (*Adapter)(nil)
	_ clients.PlanRefiner  = (*Adapter)(nil)
)

// TargetRoot retains the legacy editor package preparation layout. Native
// scanning of this location remains an S4 qualification, not a path assertion.
func (adapter *Adapter) TargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string) (string, string, error) {
	if mode != domain.PackageNative && mode != domain.PackagePrepared {
		return shared.ManagedTargetRoot(client, mode, managedRoot)
	}
	root, err := adapter.ResolveProfileRoot(client.ConfigRoot)
	if err != nil {
		return "", "", err
	}
	return root, filepath.Join(root, "plugins", "local"), nil
}

func (*Adapter) RefinePlan(ctx context.Context, _ clients.PlanInput, plan *domain.DeliveryPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if plan.Status == domain.PlanUnsupported {
		return nil
	}
	plan.Status = domain.PlanManualActivationRequired
	// PackageMode describes the retained manifest/layout contract. Readiness
	// and selected components carry preparation status independently of it.
	plan.Activation = domain.ActivationPrepared
	for i := range plan.Components {
		if plan.Components[i].Support == domain.SupportNative {
			plan.Components[i].Support = domain.SupportPrepared
		}
	}
	plan.Warnings = shared.AppendUnique(plan.Warnings, "Cursor editor package discovery is unverified; agent CLI plugins and native Stop are not qualified")
	plan.UserActions = shared.AppendUnique(plan.UserActions, "reload Cursor, then verify the prepared plugin and selected components are visible and enabled")
	return nil
}
