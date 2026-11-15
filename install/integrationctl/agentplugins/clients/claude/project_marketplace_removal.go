package claude

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The native uninstall keeps its cache. Retire only the current owned version
// while its receipt is still available, so the same saved skill can be added
// again without adopting an unowned leftover. Other versions and data stay put.
func claudeProjectRemovalCache(ctx context.Context, request domain.DeactivationRequest, allowOrphan bool) (string, error) {
	cache := ""
	for _, object := range request.NativeObjects {
		if object.Kind == claudeProjectCacheKind {
			cache = filepath.Join(request.Client.ConfigRoot, filepath.FromSlash(object.SourceRelative))
		}
	}
	if cache == "" {
		return "", nil
	}
	if _, err := claudeProjectCacheReceipt(request.DeclaredName, request.PhysicalArtifactID, request.TargetContext.ProjectRoot, cache, request.NativeObjects); err != nil {
		return "", err
	}
	if err := verifyClaudeProjectRemovalCache(ctx, request, cache, allowOrphan); err != nil {
		return "", err
	}
	return cache, nil
}

func removeClaudeProjectCache(ctx context.Context, request domain.DeactivationRequest, cache string, effect *domain.NativeEffectState) error {
	if cache == "" {
		return nil
	}
	before, err := os.Lstat(cache)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	parent, err := os.OpenRoot(filepath.Dir(cache))
	if err != nil {
		return err
	}
	defer func() { _ = parent.Close() }()
	if err := verifyClaudeProjectRemovalCache(ctx, request, cache, true); err != nil {
		return err
	}
	if err := claudeProjectCacheRemovalIdentity(request.Client.ConfigRoot, cache, parent, before); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	*effect = domain.NativeEffectUncertain
	if err := parent.RemoveAll(filepath.Base(cache)); err != nil {
		return fmt.Errorf("remove the current owned Claude project cache: %w", err)
	}
	*effect = domain.NativeEffectCommitted
	return nil
}

func claudeProjectCacheRemovalIdentity(configRoot, cache string, parent *os.Root, before os.FileInfo) error {
	if err := pathpolicy.RequireContainedChild(configRoot, cache); err != nil {
		return err
	}
	held, err := parent.Stat(".")
	if err != nil {
		return err
	}
	named, err := os.Lstat(filepath.Dir(cache))
	if err != nil || !os.SameFile(held, named) {
		return fmt.Errorf("the Claude cache parent changed before removal")
	}
	current, err := parent.Lstat(filepath.Base(cache))
	if err != nil || !current.IsDir() || !os.SameFile(before, current) {
		return fmt.Errorf("the Claude cache changed before removal")
	}
	return nil
}
