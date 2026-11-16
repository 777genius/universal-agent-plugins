package claude

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacydomain "github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

const claudeOrphanedMarker = ".orphaned_at"

func verifyClaudeProjectRemovalCache(ctx context.Context, request domain.DeactivationRequest, cache string, allowOrphan bool) error {
	if err := pathpolicy.RequireContainedChild(request.Client.ConfigRoot, cache); err != nil {
		return err
	}
	if err := rejectClaudeCacheExcludedEntries(ctx, cache); err != nil {
		return err
	}
	err := verifyClaudeProjectCache(ctx, request.Client.ConfigRoot, request.DeclaredName, request.PhysicalArtifactID, request.TargetContext.ProjectRoot, cache, request.NativeObjects, true)
	if err == nil || !allowOrphan {
		return err
	}
	receipt, receiptErr := claudeProjectCacheReceipt(request.DeclaredName, request.PhysicalArtifactID, request.TargetContext.ProjectRoot, cache, request.NativeObjects)
	if receiptErr != nil {
		return receiptErr
	}
	return verifyClaudeOrphanedCache(ctx, cache, receipt.ManagedDigest)
}

// Snapshot digests intentionally omit repository metadata and workspace locks.
// A native cache may not inherit that omission as permission to delete extras.
func rejectClaudeCacheExcludedEntries(ctx context.Context, cache string) error {
	return filepath.WalkDir(cache, func(path string, entry fs.DirEntry, err error) error {
		if path == cache && os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" || !entry.IsDir() && legacydomain.IsWorkspaceLockFile(entry.Name()) {
			return fmt.Errorf("the Claude cache contains unowned snapshot-excluded entry %q", path)
		}
		return nil
	})
}

// Official Claude uninstall leaves a decimal millisecond timestamp in the
// cache root. Normalize only a private bounded snapshot, after native absence,
// then require the complete original owned digest. Never edit a foreign cache
// merely to determine whether its remaining files belong to this installation.
func verifyClaudeOrphanedCache(ctx context.Context, cache, expected string) (err error) {
	marker := filepath.Join(cache, claudeOrphanedMarker)
	info, err := os.Lstat(marker)
	if err != nil || !info.Mode().IsRegular() || info.Size() != 13 {
		return fmt.Errorf("the Claude orphaned cache marker is not recognized")
	}
	snapshot, err := (packagesnapshot.Builder{}).Build(ctx, cache)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, snapshot.Close()) }()
	if err := removeClaudeSnapshotMarker(snapshot.Root); err != nil {
		return err
	}
	digest, err := (packagesnapshot.Builder{}).Digest(ctx, snapshot.Root)
	if err != nil {
		return err
	}
	if digest != expected {
		return fmt.Errorf("the orphaned Claude cache differs from its exact ownership receipt")
	}
	actual, err := (packagesnapshot.Builder{}).Digest(ctx, cache)
	if err != nil || actual != snapshot.Digest {
		return fmt.Errorf("the orphaned Claude cache changed during removal verification")
	}
	return rejectClaudeCacheExcludedEntries(ctx, cache)
}

func removeClaudeSnapshotMarker(root string) error {
	marker := filepath.Join(root, claudeOrphanedMarker)
	info, err := os.Lstat(marker)
	if err != nil || !info.Mode().IsRegular() || info.Size() != 13 {
		return fmt.Errorf("the Claude snapshot orphan marker is not recognized")
	}
	body, err := os.ReadFile(marker)
	if err != nil || len(body) != 13 || strings.Trim(string(body), "0123456789") != "" || body[0] == '0' {
		return fmt.Errorf("the Claude orphaned cache marker is not a millisecond timestamp")
	}
	if err := os.Chmod(root, 0700); err != nil {
		return err
	}
	if err := os.Chmod(marker, 0600); err != nil {
		return err
	}
	return os.Remove(marker)
}
