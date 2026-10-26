package grok

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.StagingLayout = (*Adapter)(nil)

// Keep unpublished bytes beside the watched plugins root. Discard may carry
// only TargetRoot, whose parent identifies the same deterministic anchor.
func (*Adapter) StagingBase(plan domain.DeliveryPlan) string {
	if plan.TargetAnchor != "" {
		return plan.TargetAnchor
	}
	return filepath.Dir(plan.TargetRoot)
}
func (*Adapter) ValidateTargetLayout(plan domain.DeliveryPlan) error {
	if plan.Scope == domain.ScopeProject || plan.TargetContext.Scope == domain.ScopeProject || plan.TargetContext.ProjectRoot != "" {
		_, err := grokPlanRoot(domain.DetectedClient{ClientID: domain.ClientGrok, ConfigRoot: plan.TargetContext.UserConfigRoot}, plan)
		return err
	}
	return nil
}
