package agentpluginscli

import (
	"context"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func managementUpdate(ctx context.Context, app App, requirements environment.Requirements, locked environment.Lock, aliases, clients []string) (environment.Requirements, environment.Lock, error) {
	return prepareSavedUpdates(ctx, app, environmentSnapshot{Requirements: requirements, Lock: locked}, aliases, clients)
}
