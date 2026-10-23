package installer

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (e *Engine) scope() domain.InstallScope {
	if e.cfg.ProjectRoot != "" {
		return domain.ScopeProject
	}
	return domain.ScopeUser
}

func (e *Engine) targetContext(client domain.DetectedClient) domain.TargetContext {
	return domain.TargetContext{Scope: e.scope(), ProjectRoot: e.cfg.ProjectRoot, UserConfigRoot: client.ConfigRoot}
}

func (e *Engine) nativeRegistryRoot(client domain.DetectedClient) (string, error) {
	if e.scope() != domain.ScopeProject {
		return client.ConfigRoot, nil
	}
	target, err := e.planner().ResolveTarget(context.Background(), client, e.scope(), "project-context")
	if err != nil {
		return "", err
	}
	return target.NativeRegistryRoot, nil
}

func preparedTargetContext(prepared *PreparedOperation, clientID string) domain.TargetContext {
	client := prepared.detected[domain.ClientID(clientID)]
	if client.ClientID == "" && string(prepared.client.ClientID) == clientID {
		client = prepared.client
	}
	return domain.TargetContext{Scope: prepared.req.Scope, ProjectRoot: prepared.req.ProjectRoot, UserConfigRoot: client.ConfigRoot}
}
