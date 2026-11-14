package usecase

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (service Service) scope() domain.InstallScope {
	if service.ProjectRoot != "" {
		return domain.ScopeProject
	}
	return domain.ScopeUser
}

func (service Service) targetContext(client domain.DetectedClient, scope domain.InstallScope) domain.TargetContext {
	return domain.TargetContext{Scope: scope, ProjectRoot: service.ProjectRoot, UserConfigRoot: client.ConfigRoot}
}

func (service Service) validateNativeBinding(ctx context.Context, binding domain.ClientBinding, client domain.DetectedClient) error {
	scope := domain.InstallScope(binding.Scope)
	if scope != domain.ScopeProject {
		return validateNativeBinding(binding, client)
	}
	if service.ProjectRoot == "" || service.Targets == nil {
		return fmt.Errorf("project binding requires its explicit project target resolver")
	}
	target, err := service.Targets.ResolveTarget(ctx, client, scope, binding.PhysicalArtifact)
	if err != nil {
		return err
	}
	if target.TargetContext != service.targetContext(client, scope) || target.ActivePath != binding.TargetLocator ||
		target.NativeRegistryRoot == "" || target.NativeRegistryRoot != binding.NativeProfileRoot {
		return fmt.Errorf("project binding differs from its selected layout or native registry")
	}
	return validateNativeBindingAt(binding, client, target.NativeRegistryRoot)
}

func nativeRegistryForBinding(binding domain.ClientBinding, client domain.DetectedClient) string {
	if binding.Scope == string(domain.ScopeProject) {
		return binding.NativeProfileRoot
	}
	if binding.NativeProfileRoot != "" {
		return binding.NativeProfileRoot
	}
	return client.ConfigRoot
}
