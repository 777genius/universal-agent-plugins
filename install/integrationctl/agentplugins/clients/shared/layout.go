package shared

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.StagingLayout = DefaultStagingLayout{}

// ManagedTargetRoot is the default clients.TargetLayout: a client that does not
// own a discovery directory of its own gets its package under the managed root,
// namespaced by client id.
func ManagedTargetRoot(client domain.DetectedClient, _ domain.PackageMode, managedRoot string) (anchor, root string, err error) {
	if strings.TrimSpace(managedRoot) == "" {
		return "", "", fmt.Errorf("managed client root is required")
	}
	return managedRoot, filepath.Join(managedRoot, "clients", string(client.ClientID)), nil
}

// DefaultStagingLayout is the clients.StagingLayout for every client that
// stages directly under the plan target root and needs no layout rule beyond
// the generic path validation.
type DefaultStagingLayout struct{}

// StagingBase reports where the transaction staging directory is created.
func (DefaultStagingLayout) StagingBase(plan domain.DeliveryPlan) string { return plan.TargetRoot }

// ValidateTargetLayout adds nothing to the generic containment checks.
func (DefaultStagingLayout) ValidateTargetLayout(domain.DeliveryPlan) error { return nil }

// PromoteNativeReady marks a plan ready and prepared when the client has a
// configuration root to write into and the selection is one it can deliver
// whole. An unsupported plan is never promoted: readiness is an upgrade over a
// usable plan, not a way to overrule the generic pipeline.
func PromoteNativeReady(plan *domain.DeliveryPlan, configRoot string, eligible bool) {
	if plan.Status == domain.PlanUnsupported || strings.TrimSpace(configRoot) == "" || !eligible {
		return
	}
	plan.Status = domain.PlanReady
	plan.Activation = domain.ActivationPrepared
}
