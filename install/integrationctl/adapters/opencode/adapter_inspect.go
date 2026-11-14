package opencode

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) Inspect(ctx context.Context, in ports.InspectInput) (ports.InspectResult, error) {
	surface := a.inspectSurface(in.Scope, workspaceRootFromInspectInput(in))
	config := surface.ConfigPath
	present, _ := statelessRestrictions(surface, config)
	if in.Record != nil {
		if target, ok := in.Record.Targets[domain.TargetOpenCode]; ok {
			config = configPathFromTarget(target, config)
			present, _ = statelessRestrictions(surface, config)
		}
	}
	if in.Record != nil {
		if target, ok := in.Record.Targets[domain.TargetOpenCode]; ok && present {
			return a.inspectTrackedConfig(ctx, target, config, surface)
		}
	}
	return a.inspectSurfaceState(config, surface), nil
}
