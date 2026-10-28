package agentpluginscli

import (
	"context"
	"errors"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func managementUpdate(ctx context.Context, app App, requirements environment.Requirements, locked environment.Lock, aliases, clients []string) (environment.Requirements, environment.Lock, error) {
	moving := []string{}
	state, err := app.StateStore.Load()
	if err != nil {
		return requirements, locked, err
	}
	for _, alias := range aliases {
		if len(clients) > 0 {
			desired := savedAliasTargets(requirements, domain.StateFileV2{}, alias)
			if !slices.Equal(desired, clients) {
				return requirements, locked, errors.New("update must include every target of each selected alias")
			}
		}
		req := requirements.Dependencies[alias]
		if environment.MovingRequirement(req) {
			moving = append(moving, alias)
		}
		if req.Registry == nil || req.Registry.Selector.Kind != environment.SelectorRelease {
			continue
		}
		nodeAlias := dependencyNodeAlias(requirements, locked, alias)
		installation, err := savedAliasInstallation(state, nodeAlias, nodeAlias)
		if err != nil {
			return requirements, locked, err
		}
		snapshot := environmentSnapshot{Requirements: requirements, Lock: locked}
		requirements, _, err = prepareSavedRegistryUpdate(ctx, app, true, snapshot, nodeAlias, installation)
		if err != nil {
			return requirements, locked, err
		}
	}
	next, err := (environment.Coordinator{Resolver: dependencyResolver{app: app, requirements: requirements, dryRun: true}}).ResolveSelected(ctx, requirements, locked, moving)
	return requirements, next, err
}
