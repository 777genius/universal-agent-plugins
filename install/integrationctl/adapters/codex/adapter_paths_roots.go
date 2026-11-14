package codex

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
)

func (a Adapter) projectRoot(workspaceRoot string) string {
	return pathpolicy.ProjectRoot(workspaceRoot, a.ProjectRoot)
}

func (a Adapter) effectiveProjectRoot(workspaceRoot string) string {
	return pathpolicy.EffectiveGitRoot(workspaceRoot, a.ProjectRoot)
}

func (a Adapter) userHome() string {
	return pathpolicy.UserHome(a.UserHome)
}

func normalizedScope(scope string) string {
	return pathpolicy.NormalizeScope(scope)
}

func (a Adapter) marketplaceRoot(scope string, workspaceRoot string) string {
	if normalizedScope(scope) == "project" {
		return filepath.Join(a.effectiveProjectRoot(workspaceRoot), ".agents", "plugins")
	}
	return filepath.Join(a.userHome(), ".agents", "plugins")
}

func (a Adapter) catalogPath(scope string, workspaceRoot string) string {
	return filepath.Join(a.marketplaceRoot(scope, workspaceRoot), "marketplace.json")
}

func (a Adapter) pluginRoot(scope, workspaceRoot, integrationID string) string {
	return filepath.Join(a.marketplaceRoot(scope, workspaceRoot), "plugins", integrationID)
}

func (a Adapter) cachePath(marketplaceName, integrationID string) string {
	return filepath.Join(a.userHome(), ".codex", "plugins", "cache", marketplaceName, integrationID, "local")
}

func (a Adapter) pathsForScope(scope, workspaceRoot, integrationID string) codexSurfacePaths {
	scope = normalizedScope(scope)
	return codexSurfacePaths{
		Scope:         scope,
		WorkspaceRoot: workspaceRoot,
		CatalogPath:   a.catalogPath(scope, workspaceRoot),
		PluginRoot:    a.pluginRoot(scope, workspaceRoot, integrationID),
		ConfigPath:    filepath.Join(a.userHome(), ".codex", "config.toml"),
	}
}
