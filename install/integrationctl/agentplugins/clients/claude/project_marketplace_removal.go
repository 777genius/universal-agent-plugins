package claude

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
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
	// A name->inode check cannot make RemoveAll(name) safe: a concurrent
	// client can swap that name immediately after the check. Retire into an
	// exclusive private directory, recheck the moved inode and content, then
	// delete through a handle pinned to that exact moved directory.
	return retireClaudeProjectCache(ctx, request, cache, parent, before, effect)
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


// retireClaudeProjectCache is also exercised with a deterministic swap after
// the caller's last identity check. A mismatch is retained for manual recovery;
// never restore with a rename that could overwrite a new foreign cache.
func retireClaudeProjectCache(ctx context.Context, request domain.DeactivationRequest, cache string, parent *os.Root, before os.FileInfo, effect *domain.NativeEffectState) error {
	*effect = domain.NativeEffectUncertain
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return fmt.Errorf("create Claude cache retirement identity: %w", err)
	}
	quarantineName := ".agentplugins-cache-retire-" + hex.EncodeToString(nonce[:])
	if err := parent.Mkdir(quarantineName, 0700); err != nil {
		return fmt.Errorf("create exclusive Claude cache quarantine: %w", err)
	}
	quarantine, err := parent.OpenRoot(quarantineName)
	if err != nil {
		return err
	}
	defer func() { _ = quarantine.Close() }()
	held, err := quarantine.Stat(".")
	if err != nil {
		return err
	}
	named, err := parent.Lstat(quarantineName)
	if err != nil || !os.SameFile(held, named) {
		return fmt.Errorf("the private Claude cache quarantine changed before retirement")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	const retired = "retired"
	if err := parent.Rename(filepath.Base(cache), filepath.Join(quarantineName, retired)); err != nil {
		return fmt.Errorf("move the Claude cache into exclusive retirement: %w", err)
	}
	retiredPath := filepath.Join(filepath.Dir(cache), quarantineName, retired)
	retain := func(cause error) error {
		return fmt.Errorf("Claude cache retirement is uncertain; retained at %q for inspection: %w", retiredPath, cause)
	}
	moved, err := quarantine.Lstat(retired)
	if err != nil || !moved.IsDir() || !os.SameFile(before, moved) {
		return retain(fmt.Errorf("the moved cache is not the previously verified owned directory"))
	}
	owned, err := quarantine.OpenRoot(retired)
	if err != nil {
		return retain(err)
	}
	defer func() { _ = owned.Close() }()
	pinned, err := owned.Stat(".")
	if err != nil || !os.SameFile(before, pinned) {
		return retain(fmt.Errorf("the retired cache handle does not reference the owned directory"))
	}
	if err := verifyClaudeRetiredCache(ctx, request, cache, retiredPath); err != nil {
		return retain(err)
	}
	if err := ctx.Err(); err != nil {
		return retain(err)
	}
	// Remove children via the pinned directory handle, not via a basename
	// that might have been replaced after verification. Only an empty
	// directory is removed by name at the end.
	directory, err := owned.Open(".")
	if err != nil {
		return retain(err)
	}
	entries, readErr := directory.ReadDir(-1)
	closeErr := directory.Close()
	if readErr != nil {
		return retain(readErr)
	}
	if closeErr != nil {
		return retain(closeErr)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return retain(err)
		}
		if err := owned.RemoveAll(entry.Name()); err != nil {
			return retain(err)
		}
	}
	current, err := quarantine.Lstat(retired)
	if err != nil || !os.SameFile(before, current) {
		return retain(fmt.Errorf("the retired cache name changed during cleanup"))
	}
	if err := owned.Close(); err != nil {
		return retain(err)
	}
	if err := quarantine.Remove(retired); err != nil {
		return retain(err)
	}
	if err := quarantine.Close(); err != nil {
		return retain(err)
	}
	current, err = parent.Lstat(quarantineName)
	if err != nil || !os.SameFile(held, current) {
		return retain(fmt.Errorf("the private cache quarantine changed before final cleanup"))
	}
	if err := parent.Remove(quarantineName); err != nil {
		return retain(err)
	}
	*effect = domain.NativeEffectCommitted
	return nil
}

func verifyClaudeRetiredCache(ctx context.Context, request domain.DeactivationRequest, original, retired string) error {
	receipt, err := claudeProjectCacheReceipt(request.DeclaredName, request.PhysicalArtifactID, request.TargetContext.ProjectRoot, original, request.NativeObjects)
	if err != nil {
		return err
	}
	if err := rejectClaudeCacheExcludedEntries(ctx, retired); err != nil {
		return err
	}
	digest, err := (packagesnapshot.Builder{}).Digest(ctx, retired)
	if err == nil && digest == receipt.ManagedDigest {
		return nil
	}
	if err := verifyClaudeOrphanedCache(ctx, retired, receipt.ManagedDigest); err != nil {
		return fmt.Errorf("the retired Claude cache differs from its ownership receipt: %w", err)
	}
	return nil
}
