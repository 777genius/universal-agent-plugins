package codex

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) validateManagedPaths(integrationID string, paths codexSurfacePaths) error {
	if err := pathpolicy.ValidateLeafID(integrationID); err != nil {
		return fmt.Errorf("invalid Codex integration id: %w", err)
	}
	expected := a.pathsForScope(paths.Scope, paths.WorkspaceRoot, integrationID)
	if err := pathpolicy.RequireExactPath(expected.CatalogPath, paths.CatalogPath); err != nil {
		return fmt.Errorf("unsafe Codex catalog path: %w", err)
	}
	if err := pathpolicy.RequireExactPath(expected.PluginRoot, paths.PluginRoot); err != nil {
		return fmt.Errorf("unsafe Codex plugin root: %w", err)
	}
	return nil
}

func (a Adapter) pathsForRecord(record domain.InstallationRecord) codexSurfacePaths {
	workspaceRoot := workspaceRootFromRecord(record)
	paths := a.pathsForScope(record.Policy.Scope, workspaceRoot, record.IntegrationID)
	target, ok := record.Targets[domain.TargetCodex]
	if !ok {
		return paths
	}
	paths.CatalogPath = catalogPathFromTarget(target, paths.CatalogPath)
	paths.PluginRoot = pluginRootFromTarget(target, paths.PluginRoot)
	return paths
}

func workspaceRootFromInspectInput(in ports.InspectInput) string {
	return pathpolicy.WorkspaceRootFromInspect(in)
}

func workspaceRootFromApplyInput(in ports.ApplyInput) string {
	return pathpolicy.WorkspaceRootFromApply(in)
}

func workspaceRootFromRecord(record domain.InstallationRecord) string {
	return pathpolicy.WorkspaceRootFromRecord(record)
}

func protectionForScope(scope string) domain.ProtectionClass {
	return pathpolicy.ProtectionForScope(scope)
}

func ownedObjects(scope, catalogPath, pluginRoot, pluginName string) []domain.NativeObjectRef {
	return []domain.NativeObjectRef{
		{
			Kind:            "marketplace_catalog",
			Path:            catalogPath,
			ProtectionClass: protectionForScope(scope),
		},
		{
			Kind:            "marketplace_entry",
			Name:            pluginName,
			Path:            catalogPath,
			ProtectionClass: protectionForScope(scope),
		},
		{
			Kind:            "plugin_root",
			Name:            pluginName,
			Path:            pluginRoot,
			ProtectionClass: protectionForScope(scope),
		},
	}
}

func fileExists(path string) bool {
	return pathpolicy.FileExists(path)
}
