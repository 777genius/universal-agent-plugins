package claude

import (
	"context"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/authoredpath"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (a Adapter) syncManagedMarketplace(ctx context.Context, manifest domain.IntegrationManifest, sourceRoot string) (string, string, string, error) {
	root := filepath.Clean(sourceRoot)
	managedRoot := managedMarketplaceRoot(a.userHome(), manifest.IntegrationID)
	if err := a.validateManagedMarketplaceRoot(manifest.IntegrationID, managedRoot); err != nil {
		return "", "", "", domain.NewError(domain.ErrMutationApply, "validate Claude managed marketplace root", err)
	}
	parent := filepath.Dir(managedRoot)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", "", "", domain.NewError(domain.ErrMutationApply, "prepare Claude managed marketplace parent", err)
	}
	tmpRoot, err := os.MkdirTemp(parent, manifest.IntegrationID+".tmp-*")
	if err != nil {
		return "", "", "", domain.NewError(domain.ErrMutationApply, "create Claude managed marketplace temp root", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmpRoot)
		}
	}()

	marketplaceName := managedMarketplaceName(manifest.IntegrationID)
	pluginDir := filepath.Join(tmpRoot, "plugins", manifest.IntegrationID)
	if err := a.materializeManagedSource(ctx, manifest, root, pluginDir); err != nil {
		return "", "", "", err
	}
	if err := writeClaudeMarketplace(filepath.Join(tmpRoot, ".claude-plugin", "marketplace.json"), marketplaceName, manifest, "./plugins/"+manifest.IntegrationID); err != nil {
		return "", "", "", err
	}
	if err := os.RemoveAll(managedRoot); err != nil && !os.IsNotExist(err) {
		return "", "", "", domain.NewError(domain.ErrMutationApply, "replace Claude managed marketplace root", err)
	}
	if err := os.Rename(tmpRoot, managedRoot); err != nil {
		return "", "", "", domain.NewError(domain.ErrMutationApply, "activate Claude managed marketplace root", err)
	}
	cleanup = false
	return managedRoot, marketplaceName, manifest.IntegrationID + "@" + marketplaceName, nil
}

func (a Adapter) materializeManagedSource(ctx context.Context, manifest domain.IntegrationManifest, sourceRoot, pluginDir string) error {
	if fileExists(filepath.Join(sourceRoot, ".claude-plugin", "plugin.json")) && !authoredpath.HasManifest(sourceRoot) {
		return copyNativeClaudePackage(sourceRoot, pluginDir)
	}
	return a.materializeAuthoredClaudeSource(ctx, manifest, sourceRoot, pluginDir)
}

func writeClaudeMarketplace(path, marketplaceName string, manifest domain.IntegrationManifest, sourceRoot string) error {
	body, err := marshalJSON(map[string]any{
		"name": marketplaceName,
		"owner": map[string]any{
			"name": "plugin-kit-ai",
		},
		"plugins": []map[string]any{
			{
				"name":        manifest.IntegrationID,
				"source":      sourceRoot,
				"description": manifest.Description,
				"version":     manifest.Version,
			},
		},
	})
	if err != nil {
		return domain.NewError(domain.ErrMutationApply, "marshal Claude marketplace catalog", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return domain.NewError(domain.ErrMutationApply, "prepare Claude marketplace catalog dir", err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return domain.NewError(domain.ErrMutationApply, "write Claude marketplace catalog", err)
	}
	return nil
}
