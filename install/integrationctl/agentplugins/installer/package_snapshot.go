package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// snapshotLocalPackage uses packagedigest executable overrides so Windows
// host FileMode (no 0111 on regular files) does not drop logical bin/ helpers
// from TreeDigest. AcquireLocal hashes POSIX bits from the checkout.
// LocalPackageTreeDigest is the canonical TreeDigest of a local package root.
// It snapshots into TempRoot, does not write installer state, and does not
// report Progress. Optional Assess still binds that digest.
func (e *Engine) LocalPackageTreeDigest(ctx context.Context, packageRoot string) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("%w: context is required", ErrInvalidRequest)
	}
	if packageRoot == "" || !validRoot(packageRoot) {
		return "", fmt.Errorf("%w: PackageRoot must be an explicit absolute clean path", ErrInvalidRequest)
	}
	if overlappingRoots(e.cfg.TempRoot, packageRoot) {
		return "", fmt.Errorf("%w: TempRoot must not overlap PackageRoot", ErrInvalidRequest)
	}
	if err := os.MkdirAll(e.cfg.TempRoot, 0700); err != nil {
		return "", err
	}
	snapshot, err := snapshotLocalPackage(ctx, e.cfg.TempRoot, packageRoot)
	if err != nil {
		return "", err
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	if err := e.assessSnapshot(ctx, snapshot, nil); err != nil {
		return "", err
	}
	return snapshot.TreeDigest, nil
}

func snapshotLocalPackage(ctx context.Context, tempRoot, packageRoot string) (domain.PackageSnapshot, error) {
	absolute, err := filepath.Abs(packageRoot)
	if err != nil {
		return domain.PackageSnapshot{}, fmt.Errorf("acquire local package: resolve source failed")
	}
	executables, err := declaredPackageExecutables(absolute)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	source := domain.SourceIdentity{RequestedSource: packageRoot, CanonicalSource: filepath.Clean(absolute), SourceBindingHint: "direct-local"}
	snapshot, err := (packagedigest.Builder{TempRoot: tempRoot}).SnapshotWithExecutables(ctx, absolute, source, executables)
	if err != nil {
		return domain.PackageSnapshot{}, fmt.Errorf("acquire local package: snapshot package content failed")
	}
	return snapshot, nil
}

func snapshotRequestPackage(ctx context.Context, tempRoot string, req Request) (domain.PackageSnapshot, error) {
	if req.AcquiredSnapshot != nil {
		return snapshotAcquiredRequest(ctx, tempRoot, req)
	}
	if req.ExecutableFiles == nil {
		return snapshotLocalPackage(ctx, tempRoot, req.PackageRoot)
	}
	for _, relative := range req.ExecutableFiles {
		if relative == "." || relative == ".." || path.IsAbs(relative) || path.Clean(relative) != relative || strings.HasPrefix(relative, "../") || strings.Contains(relative, `\`) || filepath.VolumeName(relative) != "" {
			return domain.PackageSnapshot{}, fmt.Errorf("%w: invalid snapshot executable path", ErrInvalidRequest)
		}
		info, err := os.Lstat(filepath.Join(req.PackageRoot, filepath.FromSlash(relative)))
		if err != nil || !info.Mode().IsRegular() {
			return domain.PackageSnapshot{}, fmt.Errorf("%w: snapshot executable is not a regular package file", ErrInvalidRequest)
		}
	}
	source := domain.SourceIdentity{RequestedSource: req.PackageRoot, CanonicalSource: req.PackageRoot, SourceBindingHint: "direct-local"}
	return (packagedigest.Builder{TempRoot: tempRoot}).SnapshotWithExecutables(ctx, req.PackageRoot, source, req.ExecutableFiles)
}

func snapshotAcquiredRequest(ctx context.Context, tempRoot string, req Request) (domain.PackageSnapshot, error) {
	if err := validateAcquiredRequest(req); err != nil {
		return domain.PackageSnapshot{}, err
	}
	acquired := req.AcquiredSnapshot
	files := append([]string{}, acquired.ExecutableFiles...)
	if err := validateAcquiredExecutables(req.PackageRoot, files); err != nil {
		return domain.PackageSnapshot{}, err
	}
	snapshot, err := (packagedigest.Builder{TempRoot: tempRoot}).SnapshotWithExecutables(ctx, req.PackageRoot, acquired.Source, files)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	if snapshot.TreeDigest != acquired.TreeDigest || snapshot.DigestAlgorithm != acquired.DigestAlgorithm ||
		!equalStrings(snapshot.ExecutableFiles, acquired.ExecutableFiles) || snapshot.Source != acquired.Source {
		_ = packagedigest.Remove(snapshot)
		return domain.PackageSnapshot{}, fmt.Errorf("%w: acquired snapshot identity changed while resealing bytes", ErrInvalidRequest)
	}
	snapshot.OriginMode = acquired.OriginMode
	snapshot.Directory = cloneAcquiredDirectory(acquired.Directory)
	snapshot.ExecutableFiles = append([]string{}, acquired.ExecutableFiles...)
	return snapshot, nil
}

func cloneAcquiredDirectory(origin *domain.DirectoryOrigin) *domain.DirectoryOrigin {
	if origin == nil {
		return nil
	}
	cloned := *origin
	return &cloned
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func declaredPackageExecutables(root string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	add := func(rel string) {
		rel = path.Clean(strings.TrimPrefix(filepath.ToSlash(rel), "./"))
		if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
			return
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || !info.Mode().IsRegular() {
			return
		}
		if _, ok := seen[rel]; ok {
			return
		}
		seen[rel] = struct{}{}
		out = append(out, rel)
	}
	raw, err := os.ReadFile(filepath.Join(root, "mcp.json"))
	if err == nil {
		var mcp struct {
			Servers map[string]struct {
				Command string `json:"command"`
			} `json:"mcpServers"`
		}
		if json.Unmarshal(raw, &mcp) == nil {
			for _, server := range mcp.Servers {
				add(server.Command)
			}
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "bin"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		add(path.Join("bin", entry.Name()))
	}
	return out, nil
}
