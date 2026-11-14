package claude

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) inspectPluginList(ctx context.Context, inspect inspectContext, record *domain.InstallationRecord) (domain.InstallState, bool, error) {
	result, err := a.runner().Run(ctx, ports.Command{
		Argv: []string{"claude", "plugin", "list", "--json"},
		Dir:  a.commandDirForScope(inspect.scope, inspect.workspaceRoot),
	})
	if err != nil {
		return "", false, domain.NewError(domain.ErrMutationApply, "run Claude plugin list", err)
	}
	if result.ExitCode != 0 {
		msg := strings.TrimSpace(string(result.Stderr))
		if msg == "" {
			msg = strings.TrimSpace(string(result.Stdout))
		}
		if msg == "" {
			msg = "Claude plugin list failed"
		}
		return "", false, domain.NewError(domain.ErrMutationApply, msg, nil)
	}
	var items []listedPlugin
	if err := json.Unmarshal(result.Stdout, &items); err != nil {
		return "", false, domain.NewError(domain.ErrMutationApply, "parse Claude plugin list JSON", err)
	}
	wantRef := inspect.integrationID + "@" + managedMarketplaceName(inspect.integrationID)
	if record != nil {
		if value := pluginRefFromRecord(*record); value != "" {
			wantRef = value
		}
	}
	wantScope := strings.ToLower(strings.TrimSpace(inspect.scope))
	for _, item := range items {
		if strings.TrimSpace(item.ID) != wantRef {
			continue
		}
		if wantScope != "" && strings.ToLower(strings.TrimSpace(item.Scope)) != wantScope {
			continue
		}
		if item.Enabled {
			return domain.InstallInstalled, true, nil
		}
		return domain.InstallDisabled, true, nil
	}
	return domain.InstallRemoved, true, nil
}
