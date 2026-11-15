package claude

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

const claudeProjectCacheKind = "claude_project_plugin_cache"

// This is a desired receipt until Claude's own install and fresh listing have
// confirmed it. Hash the sanitized bundle, never the original package snapshot.
func claudeProjectCacheProjection(ctx context.Context, root string, plan domain.DeliveryPlan) ([]domain.NativeObjectOwnership, error) {
	version := claudeProjectVersion(plan.DeclaredVersion)
	if !exactAbsolute(plan.TargetContext.UserConfigRoot) || pathpolicy.ValidateLeafID(plan.DeclaredName) != nil || pathpolicy.ValidateLeafID(version) != nil {
		return nil, fmt.Errorf("the Claude project cache identity is incomplete")
	}
	bundle := filepath.Join(root, claudeProjectBundleDirectory)
	if err := pathpolicy.RequireContainedChild(root, bundle); err != nil {
		return nil, err
	}
	digest, err := (packagesnapshot.Builder{}).Digest(ctx, bundle)
	if err != nil {
		return nil, fmt.Errorf("verify sanitized Claude project bundle: %w", err)
	}
	marketplace := ProjectMarketplaceName(plan.PhysicalArtifactID, plan.TargetContext.ProjectRoot)
	return []domain.NativeObjectOwnership{{
		ObjectID: "claude-project-cache:" + plan.DeclaredName + "@" + marketplace,
		Kind:     claudeProjectCacheKind, LogicalName: plan.DeclaredName,
		// Project receipts stay inside their project. The copied cache location
		// is relative to the separately validated original user-profile authority.
		Path:           filepath.Join(plan.ActivePath, claudeProjectBundleDirectory),
		SourceRelative: filepath.ToSlash(filepath.Join("plugins", "cache", marketplace, plan.DeclaredName, version)),
		ManagedDigest:  digest, ProtectionClass: "managed",
	}}, nil
}

func verifyClaudeProjectCache(ctx context.Context, configRoot, name, physical, project, cache string, objects []domain.NativeObjectOwnership, allowMissing bool) error {
	marketplace := ProjectMarketplaceName(physical, project)
	parent := filepath.Join(configRoot, "plugins", "cache", marketplace, name)
	if filepath.Dir(cache) != parent || pathpolicy.ValidateLeafID(filepath.Base(cache)) != nil {
		return fmt.Errorf("the Claude project cache is outside the exact owned namespace")
	}
	if err := pathpolicy.RequireContainedChild(configRoot, cache); err != nil {
		return err
	}
	if _, err := os.Lstat(cache); allowMissing && os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	var receipt *domain.NativeObjectOwnership
	for i := range objects {
		if objects[i].Kind == claudeProjectCacheKind {
			if receipt != nil {
				return fmt.Errorf("the Claude project cache ownership is duplicated")
			}
			receipt = &objects[i]
		}
	}
	source := filepath.Join(project, ".claude", "skills", physical, claudeProjectBundleDirectory)
	relative := filepath.ToSlash(filepath.Join("plugins", "cache", marketplace, name, filepath.Base(cache)))
	if receipt == nil || receipt.ObjectID != "claude-project-cache:"+name+"@"+marketplace || receipt.Path != source || receipt.SourceRelative != relative || receipt.LogicalName != name || receipt.ProtectionClass != "managed" || receipt.UserModified || receipt.ManagedDigest == "" {
		return fmt.Errorf("the Claude project cache has no exact prior ownership receipt")
	}
	digest, err := (packagesnapshot.Builder{}).Digest(ctx, cache)
	if err != nil {
		return fmt.Errorf("verify owned Claude project cache: %w", err)
	}
	if digest != receipt.ManagedDigest {
		return fmt.Errorf("the Claude project cache was modified outside agentplugins; preserve it and inspect the native plugin")
	}
	return nil
}
