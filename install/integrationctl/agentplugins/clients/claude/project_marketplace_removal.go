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
	// exclusive private directory and recheck the moved inode and content.
	// Retain its bytes: another native process may hold an existing cwd or fd.
	return retireClaudeProjectCache(ctx, request, cache, parent, before, effect)
}

func claudeProjectCacheRemovalIdentity(configRoot, cache string, parent *os.Root, before os.FileInfo) error {
	if err := claudeProjectCacheParentIdentity(configRoot, cache, parent); err != nil {
		return err
	}
	current, err := parent.Lstat(filepath.Base(cache))
	if err != nil || !current.IsDir() || !os.SameFile(before, current) {
		return fmt.Errorf("the Claude cache changed before removal")
	}
	return nil
}

func claudeProjectCacheParentIdentity(configRoot, cache string, parent *os.Root) error {
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
	return nil
}

type claudeRetiredCacheVerifier func(context.Context, domain.DeactivationRequest, string, string) error

// A mismatch is retained for manual recovery; never restore with a rename that
// could overwrite a new foreign cache. The verifier parameter is a local seam
// for exercising real namespace replacement around the content read.
func retireClaudeProjectCache(ctx context.Context, request domain.DeactivationRequest, cache string, parent *os.Root, before os.FileInfo, effect *domain.NativeEffectState) error {
	return retireClaudeProjectCacheWithVerifier(ctx, request, cache, parent, before, effect, verifyClaudeRetiredCache)
}

func retireClaudeProjectCacheWithVerifier(ctx context.Context, request domain.DeactivationRequest, cache string, parent *os.Root, before os.FileInfo, effect *domain.NativeEffectState, verify claudeRetiredCacheVerifier) error {
	*effect = domain.NativeEffectUncertain
	quarantine, err := newClaudeCacheQuarantine(parent)
	if err != nil {
		return err
	}
	defer func() { _ = quarantine.Close() }()
	retiredPath := filepath.Join(quarantine.Name(), "retired")
	retain := func(cause error) error {
		return fmt.Errorf("the Claude cache retirement is uncertain; original private path %q: %w", retiredPath, cause)
	}
	if err := ctx.Err(); err != nil {
		return retain(err)
	}
	if err := parent.Rename(filepath.Base(cache), filepath.Join(filepath.Base(quarantine.Name()), "retired")); err != nil {
		return retain(fmt.Errorf("move the Claude cache into exclusive retirement: %w", err))
	}
	owned, err := openClaudeRetiredCache(quarantine, before)
	if err != nil {
		return retain(err)
	}
	defer func() { _ = owned.Close() }()
	if err := verifyClaudeCacheRetirement(ctx, request, cache, parent, quarantine, owned, before, verify); err != nil {
		return retain(err)
	}
	*effect = domain.NativeEffectCommitted
	return nil
}

func newClaudeCacheQuarantine(parent *os.Root) (*os.Root, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("create Claude cache retirement identity: %w", err)
	}
	name := ".agentplugins-cache-retire-" + hex.EncodeToString(nonce[:])
	if err := parent.Mkdir(name, 0700); err != nil {
		return nil, fmt.Errorf("create exclusive Claude cache quarantine: %w", err)
	}
	quarantine, err := parent.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("open private Claude cache quarantine %q: %w", filepath.Join(parent.Name(), name), err)
	}
	if err := claudeCacheQuarantineIdentity(parent, quarantine); err != nil {
		_ = quarantine.Close()
		return nil, fmt.Errorf("private Claude cache quarantine at %q requires inspection: %w", filepath.Join(parent.Name(), name), err)
	}
	return quarantine, nil
}

func claudeCacheQuarantineIdentity(parent, quarantine *os.Root) error {
	held, err := quarantine.Stat(".")
	if err != nil {
		return err
	}
	named, err := parent.Lstat(filepath.Base(quarantine.Name()))
	if err != nil || !named.IsDir() || !os.SameFile(held, named) {
		return fmt.Errorf("the private Claude cache quarantine changed")
	}
	return nil
}

func openClaudeRetiredCache(quarantine *os.Root, before os.FileInfo) (*os.Root, error) {
	moved, err := quarantine.Lstat("retired")
	if err != nil || !moved.IsDir() || !os.SameFile(before, moved) {
		return nil, fmt.Errorf("the moved cache is not the previously verified owned directory")
	}
	owned, err := quarantine.OpenRoot("retired")
	if err != nil {
		return nil, err
	}
	pinned, err := owned.Stat(".")
	if err != nil || !os.SameFile(before, pinned) {
		_ = owned.Close()
		return nil, fmt.Errorf("the retired cache handle does not reference the owned directory")
	}
	return owned, nil
}

func verifyClaudeCacheRetirement(ctx context.Context, request domain.DeactivationRequest, original string, parent, quarantine, owned *os.Root, before os.FileInfo, verify claudeRetiredCacheVerifier) error {
	if err := claudeCacheRetirementIdentity(request.Client.ConfigRoot, original, parent, quarantine, owned, before); err != nil {
		return err
	}
	// Rejoin the pathname snapshot with both held directory identities.
	// This proves which version was retired, not closure of native writers.
	if err := verify(ctx, request, original, filepath.Join(quarantine.Name(), "retired")); err != nil {
		return err
	}
	if err := claudeCacheRetirementIdentity(request.Client.ConfigRoot, original, parent, quarantine, owned, before); err != nil {
		return err
	}
	if _, err := parent.Lstat(filepath.Base(original)); !os.IsNotExist(err) {
		if err != nil {
			return err
		}
		return fmt.Errorf("the active Claude cache name reappeared during retirement")
	}
	return ctx.Err()
}

func claudeCacheRetirementIdentity(configRoot, original string, parent, quarantine, owned *os.Root, before os.FileInfo) error {
	if err := claudeProjectCacheParentIdentity(configRoot, original, parent); err != nil {
		return err
	}
	if err := claudeCacheQuarantineIdentity(parent, quarantine); err != nil {
		return err
	}
	named, err := quarantine.Lstat("retired")
	if err != nil || !named.IsDir() || !os.SameFile(before, named) {
		return fmt.Errorf("the retired Claude cache name changed during verification")
	}
	pinned, err := owned.Stat(".")
	if err != nil || !os.SameFile(before, pinned) {
		return fmt.Errorf("the retired Claude cache handle changed during verification")
	}
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
