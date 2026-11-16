package claude

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const claudeProjectBundleDirectory = ".agentplugins-plugin"

// ProjectMarketplaceName keeps equal installation ids from independent project
// stores separate in Claude's shared user-profile marketplace registry.
func ProjectMarketplaceName(physical, project string) string {
	return shared.ManagedMarketplaceName(physical + "\x00" + filepath.Clean(project))
}

func claudeProjectVersion(declared string) string {
	if declared == "" {
		return "0.0.0"
	}
	return declared
}

// Keep the existing sanitized projection intact inside a local marketplace.
// The outer directory deliberately has no plugin.json: an explicitly installed
// project plugin must not also be auto-discovered as a second @skills-dir copy.
func projectClaudeMarketplace(root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == claudeProjectBundleDirectory {
			return fmt.Errorf("portable package uses reserved Claude project bundle path")
		}
	}
	bundle := filepath.Join(root, claudeProjectBundleDirectory)
	if err := os.Mkdir(bundle, 0700); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := os.Rename(filepath.Join(root, entry.Name()), filepath.Join(bundle, entry.Name())); err != nil {
			return err
		}
	}
	innerPlan := plan
	innerPlan.ActivePath = filepath.Join(plan.ActivePath, claudeProjectBundleDirectory)
	// Version is optional in the portable format. Give its native projection
	// a stable cache leaf; same-version content updates use exact reinstall.
	envelope.Manifest.Version = claudeProjectVersion(envelope.Manifest.Version)
	if err := ProjectClaude(bundle, envelope, innerPlan, dataPath); err != nil {
		return err
	}
	return shared.WriteJSON(filepath.Join(root, ".claude-plugin", "marketplace.json"), map[string]any{
		"name":    ProjectMarketplaceName(plan.PhysicalArtifactID, plan.TargetContext.ProjectRoot),
		"owner":   map[string]any{"name": "Agent Plugins CLI"},
		"plugins": []map[string]any{{"name": envelope.Manifest.Name, "source": "./" + claudeProjectBundleDirectory}},
	})
}

func (*Adapter) ProjectActiveNative(ctx context.Context, root string, _ domain.PackageEnvelope, plan domain.DeliveryPlan, _ string) ([]domain.NativeObjectOwnership, error) {
	if plan.Scope != domain.ScopeProject {
		return nil, nil
	}
	current, err := hasClaudeProjectMarketplace(root, plan.DeclaredName, plan.PhysicalArtifactID)
	if err != nil {
		return nil, err
	}
	if !current {
		return nil, domain.ErrProjectionRefreshRequired
	}
	return claudeProjectCacheProjection(ctx, root, plan)
}

func hasClaudeProjectMarketplace(root, name, physical string) (bool, error) {
	document, err := readClaudeJSONObject(root, filepath.Join(".claude-plugin", "marketplace.json"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	plugins, ok := document["plugins"].([]any)
	project := filepath.Dir(filepath.Dir(filepath.Dir(root)))
	if document["name"] != ProjectMarketplaceName(physical, project) || !ok || len(plugins) != 1 {
		return false, fmt.Errorf("the Claude project marketplace differs from the owned package")
	}
	plugin, ok := plugins[0].(map[string]any)
	if !ok || plugin["name"] != name || plugin["source"] != "./"+claudeProjectBundleDirectory {
		return false, fmt.Errorf("the Claude project marketplace has an unexpected plugin source")
	}
	if _, err := os.Lstat(filepath.Join(root, ".claude-plugin", "plugin.json")); !os.IsNotExist(err) {
		return false, fmt.Errorf("the Claude project marketplace also claims implicit skills-directory discovery")
	}
	manifest, err := readClaudeJSONObject(root, filepath.Join(claudeProjectBundleDirectory, ".claude-plugin", "plugin.json"))
	if err != nil {
		return false, fmt.Errorf("read Claude project bundle manifest: %w", err)
	}
	if manifest["name"] != name {
		return false, fmt.Errorf("the Claude project bundle has no exact owned plugin manifest")
	}
	return true, nil
}
