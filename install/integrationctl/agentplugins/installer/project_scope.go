package installer

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (e *Engine) scope() domain.InstallScope {
	if e.cfg.ProjectRoot != "" {
		return domain.ScopeProject
	}
	return domain.ScopeUser
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

func (e *Engine) validatePrepareContext(req Request) error {
	if req.ProjectRoot != e.cfg.ProjectRoot || (req.Scope == domain.ScopeProject) != (e.cfg.ProjectRoot != "") {
		return fmt.Errorf("%w: request scope/project root differs from configured engine", ErrInvalidRequest)
	}
	if req.Scope != domain.ScopeUser && req.Scope != domain.ScopeProject {
		return fmt.Errorf("%w: unsupported request scope %q", ErrInvalidRequest, req.Scope)
	}
	if req.Scope == domain.ScopeProject {
		if err := e.validateProjectClients(req); err != nil {
			return err
		}
	}
	return e.CheckProjectNamespace()
}

func (e *Engine) validateProjectClients(req Request) error {
	ids := []string{req.ClientID}
	if len(req.Targets) != 0 {
		ids = make([]string, 0, len(req.Targets))
		for _, target := range req.Targets {
			ids = append(ids, target.ClientID)
		}
	}
	for _, id := range ids {
		if !ClientScopeSupported(e.cfg.Registry, id, req.Scope) {
			return fmt.Errorf("%w: project scope is not qualified for %s", ErrInvalidRequest, id)
		}
	}
	return nil
}
