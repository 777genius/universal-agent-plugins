package loader

import (
	"context"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// OpenAILoader is an explicit compatibility importer. The standard Loader
// never falls back to this format when root plugin.json is absent or invalid.
type OpenAILoader struct{ Loader }

func (native OpenAILoader) Load(ctx context.Context, input domain.LoadInput) (domain.PackageEnvelope, error) {
	if err := ctx.Err(); err != nil {
		return domain.PackageEnvelope{}, err
	}
	root, err := filepath.Abs(input.SnapshotRoot)
	if err != nil {
		return domain.PackageEnvelope{}, domain.FatalLoad("snapshot_invalid", ".codex-plugin/plugin.json", "resolve package snapshot root", err)
	}
	manifest, components, diagnostics, manifestDigest, err := native.loadOpenAIPluginManifest(filepath.Join(root, ".codex-plugin", "plugin.json"))
	if err != nil {
		return domain.PackageEnvelope{}, err
	}
	if err := rejectDiscoverableHooks(root); err != nil {
		return domain.PackageEnvelope{}, domain.FatalLoad("official_hooks_unsupported", "hooks/hooks.json", "lifecycle hooks are not supported by this importer; remove the hooks directory before installation", err)
	}
	mcp, componentDiagnostics := native.loadOpenAIMCP(filepath.Join(root, ".mcp.json"), components.MCP)
	diagnostics = append(diagnostics, componentDiagnostics...)
	app, componentDiagnostics := native.loadApp(filepath.Join(root, ".app.json"), components.App, false)
	diagnostics = append(diagnostics, componentDiagnostics...)
	var skills map[string]domain.Skill
	var invalid []string
	var invalidRoot bool
	if components.Skills {
		skills, invalid, invalidRoot, componentDiagnostics = loadSkills(filepath.Join(root, "skills"))
		diagnostics = append(diagnostics, componentDiagnostics...)
	}
	inventory := domain.ComponentInventory{MCPPresent: mcp.Present, MCPEnabled: mcp.Enabled, MCPServers: sortedMCPServerNames(mcp.Servers), AppPresent: app.Present, AppBindings: sortedAppBindingNames(app.Bindings), Skills: sortedSkillNames(skills), InvalidSkills: invalid, InvalidSkillsRoot: invalidRoot}
	for name := range mcp.InvalidServer {
		inventory.InvalidMCPServer = append(inventory.InvalidMCPServer, name)
	}
	sort.Strings(inventory.InvalidMCPServer)
	executable := append([]string(nil), input.ExecutableFiles...)
	sort.Strings(executable)
	return domain.PackageEnvelope{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDOpenAIPlugin, Manifest: manifest, MCP: mcp, App: app, Skills: skills, Inventory: inventory, Diagnostics: diagnostics, Source: input.Source, TreeDigest: input.TreeDigest, ManifestDigest: manifestDigest, ExecutableFiles: executable, SnapshotRoot: root}, nil
}
