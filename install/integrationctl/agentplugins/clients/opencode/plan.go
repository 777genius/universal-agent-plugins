package opencode

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PlanRefiner = (*Adapter)(nil)

// Prior MCP receipts also require the config codec when the desired set is
// empty. Directory skill removal does not use that codec.
func (*Adapter) OwnedOpenCodeNativeRequirements(objects []domain.NativeObjectOwnership) (skills, config bool) {
	for _, object := range objects {
		switch object.Kind {
		case OpenCodeMCPObjectKind, OpenCodeV2MCPObjectKind:
			config = true
		case openCodeSkillKind:
			skills = true
		}
	}
	return
}

func (*Adapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	*plan = plan.WithNativeProjectionRefresh(nativeDialectProjectionChanged(in.PreviousNativeObjects, in.Client.OpenCodeHost))
	if in.Client.OpenCodeHost == nil && hasPlannedOpenCodeNative(*plan) {
		return clients.ErrOpenCodeAdapterUnavailable
	}
	if in.Client.OpenCodeHost != nil {
		skills, transports, err := clients.PlannedOpenCodeNativeRequirements(in.Envelope, *plan)
		if err != nil {
			return err
		}
		if err := in.Client.OpenCodeHost.ValidateNative(skills, transports); err != nil {
			return err
		}
		plan.OpenCodeHost = in.Client.OpenCodeHost
	}
	root := in.Client.ConfigRoot
	if in.TargetContext.Scope == domain.ScopeProject {
		layout, err := openCodeTargetLayout(in.Client, in.TargetContext, plan.NativeRegistryRoot)
		if err != nil {
			return err
		}
		root = layout.Root
	}
	shared.PromoteNativeReady(plan, root, shared.OnlyNativeComponents(plan.Components))
	plan.UserActions = shared.AppendUnique(plan.UserActions, "agentplugins will install the package's skills and MCP servers; restart OpenCode when complete")
	return nil
}

// A host change requires new package projection bytes even at the same source
// revision. This only requests staging; existing receipt/transition validation
// still decides whether the recorded objects can safely change dialect.
func nativeDialectProjectionChanged(previous []domain.NativeObjectOwnership, host domain.OpenCodeHostAuthority) bool {
	authority, ok := host.(interface{ ConfigDialect() string })
	if !ok {
		return false
	}
	target := authority.ConfigDialect()
	for _, object := range previous {
		if object.Kind == OpenCodeMCPObjectKind && target == "opencode_v2" || object.Kind == OpenCodeV2MCPObjectKind && target == "opencode_v1" {
			return true
		}
	}
	return false
}
