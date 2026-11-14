package planner

import (
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (planner Planner) requestContext(request domain.PlanRequest) (domain.TargetContext, error) {
	target, err := domain.NewTargetContext(request.Scope, request.ProjectRoot, request.Client.ConfigRoot)
	if err != nil {
		return domain.TargetContext{}, err
	}
	if target.ProjectRoot != planner.ProjectRoot {
		return domain.TargetContext{}, fmt.Errorf("request project root differs from configured planner")
	}
	return target, nil
}

func (planner Planner) validateProjectRegistry(plan domain.DeliveryPlan) error {
	if plan.Scope != domain.ScopeProject {
		return nil
	}
	if _, ok := clients.As[clients.NativeRegistryLayout](planner.Registry, plan.ClientID); !ok {
		return fmt.Errorf("project native registry layout is required for %s", plan.ClientID)
	}
	root := plan.NativeRegistryRoot
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return fmt.Errorf("project native registry root must be explicit absolute and clean")
	}
	if root != plan.TargetContext.ProjectRoot {
		if err := planner.Paths.RequireContainedChild(plan.TargetContext.ProjectRoot, root); err != nil {
			return fmt.Errorf("unsafe project native registry: %w", err)
		}
	}
	return nil
}
