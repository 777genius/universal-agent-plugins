package claude

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func listClaudeDeactivation(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (legacyports.CommandResult, error) {
	if request.TargetContext.Scope != domain.ScopeProject {
		return runClaudeList(ctx, env, request.BackendExecutable, request.Client.ConfigRoot, request.ManagedArtifactPath)
	}
	command, err := claudeProjectListCommand(request.BackendExecutable, request.Client, request.TargetContext, request.NativeRegistryRoot, request.ManagedArtifactPath)
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	return runPreparedClaudeList(ctx, env, command)
}
