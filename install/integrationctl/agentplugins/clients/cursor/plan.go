package cursor

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.TargetLayout = (*Adapter)(nil)
	_ clients.PlanRefiner  = (*Adapter)(nil)
)

// TargetRoot delivers the native package into the local plugin directory Cursor
// itself scans, rather than under the managed root.
func (*Adapter) TargetRoot(client domain.DetectedClient, mode domain.PackageMode, managedRoot string) (string, string, error) {
	if mode != domain.PackageNative {
		return shared.ManagedTargetRoot(client, mode, managedRoot)
	}
	if strings.TrimSpace(client.ConfigRoot) == "" {
		return "", "", fmt.Errorf("the Cursor config root is unavailable")
	}
	return client.ConfigRoot, filepath.Join(client.ConfigRoot, "plugins", "local"), nil
}

func (*Adapter) RefinePlan(_ context.Context, _ clients.PlanInput, plan *domain.DeliveryPlan) error {
	plan.UserActions = shared.AppendUnique(plan.UserActions, "reload Cursor, then verify the plugin appears before using its components")
	return nil
}
