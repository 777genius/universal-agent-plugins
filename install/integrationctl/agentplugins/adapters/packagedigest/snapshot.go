// Package packagedigest creates sealed package snapshots and the versioned,
// framed tree digest used for immutable Agent Plugins package identity.
package packagedigest

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	icdomain "github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

const digestDomain = "agentplugins.package-tree\x00sha256\x00v1"

type Builder struct {
	TempRoot string
	Limits   domain.PackageLimits
}

type entry struct {
	rel        string
	sourcePath string
	kind       string
	mode       string
	size       int64
	info       os.FileInfo
	target     string
}

func (builder Builder) Snapshot(ctx context.Context, sourceRoot string, source domain.SourceIdentity) (domain.PackageSnapshot, error) {
	return builder.snapshot(ctx, sourceRoot, source, nil)
}

// SnapshotWithExecutables uses executable paths derived from immutable Git
// tree modes instead of host checkout permissions.
func (builder Builder) SnapshotWithExecutables(ctx context.Context, sourceRoot string, source domain.SourceIdentity, executablePaths []string) (domain.PackageSnapshot, error) {
	overrides := make(map[string]bool, len(executablePaths))
	for _, relative := range executablePaths {
		overrides[relative] = true
	}
	return builder.snapshot(ctx, sourceRoot, source, overrides)
}

func (builder Builder) snapshot(ctx context.Context, sourceRoot string, source domain.SourceIdentity, executableOverrides map[string]bool) (domain.PackageSnapshot, error) {
	entries, fileCount, totalBytes, err := inspect(ctx, sourceRoot, builder.limits(), executableOverrides)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	tempRoot, err := os.MkdirTemp(builder.TempRoot, "agentplugins-package-*")
	if err != nil {
		return domain.PackageSnapshot{}, fmt.Errorf("create package snapshot: %w", err)
	}
	root := filepath.Join(tempRoot, "root")
	cleanup := func(err error) (domain.PackageSnapshot, error) {
		_ = makeWritable(tempRoot)
		_ = os.RemoveAll(tempRoot)
		return domain.PackageSnapshot{}, err
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		return cleanup(fmt.Errorf("create package snapshot root: %w", err))
	}
	digest := sha256.New()
	frame(digest, []byte(digestDomain))
	var executable []string
	for _, item := range entries {
		if err := ctx.Err(); err != nil {
			return cleanup(err)
		}
		destination := filepath.Join(root, filepath.FromSlash(item.rel))
		switch item.kind {
		case "directory":
			if err := os.Mkdir(destination, 0o700); err != nil {
				return cleanup(fmt.Errorf("create snapshot directory %q: %w", item.rel, err))
			}
			writeEntryHeader(digest, item, 0)
		case "symlink":
			if err := os.Symlink(item.target, destination); err != nil {
				return cleanup(fmt.Errorf("create snapshot symlink %q: %w", item.rel, err))
			}
			writeEntryHeader(digest, item, 0)
		case "file":
			if item.mode == "100755" {
				executable = append(executable, item.rel)
			}
			if err := copyAndHash(item, destination, digest); err != nil {
				return cleanup(err)
			}
		}
	}
	if err := seal(root, entries); err != nil {
		return cleanup(err)
	}
	return domain.PackageSnapshot{
		Root: root, TreeDigest: "sha256:" + hex.EncodeToString(digest.Sum(nil)),
		DigestAlgorithm: domain.TreeDigestAlgorithm, FileCount: fileCount,
		TotalBytes: totalBytes, ExecutableFiles: executable, Source: source,
	}, nil
}

// Remove deletes a snapshot created by Builder after restoring directory
// permissions. Callers own snapshot lifetime explicitly.
func Remove(snapshot domain.PackageSnapshot) error {
	if strings.TrimSpace(snapshot.Root) == "" {
		return nil
	}
	base := filepath.Dir(filepath.Clean(snapshot.Root))
	if filepath.Base(snapshot.Root) != "root" || !strings.HasPrefix(filepath.Base(base), "agentplugins-package-") {
		return fmt.Errorf("refuse to remove unrecognized snapshot root %q", snapshot.Root)
	}
	_ = makeWritable(base)
	return os.RemoveAll(base)
}

func (builder Builder) limits() domain.PackageLimits {
	limits := builder.Limits
	if limits.MaxFiles <= 0 {
		limits.MaxFiles = domain.DefaultMaxFiles
	}
	if limits.MaxFileBytes <= 0 {
		limits.MaxFileBytes = domain.DefaultMaxFileBytes
	}
	if limits.MaxTreeBytes <= 0 {
		limits.MaxTreeBytes = domain.DefaultMaxTreeBytes
	}
	if limits.MaxDepth <= 0 {
		limits.MaxDepth = domain.DefaultMaxDepth
	}
	return limits
}

func inspect(ctx context.Context, sourceRoot string, limits domain.PackageLimits, executableOverrides map[string]bool) ([]entry, int, int64, error) {
	root, err := filepath.Abs(sourceRoot)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("resolve package root: %w", err)
	}
	root = filepath.Clean(root)
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("inspect package root: %w", err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return nil, 0, 0, fmt.Errorf("package root must be a real directory")
	}
	var entries []entry
	seen := map[string]string{}
	known := map[string]entry{".": {rel: ".", kind: "directory"}}
	files := 0
	nodes := 0
	var total int64
	err = filepath.WalkDir(root, func(candidate string, dirEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if candidate == root {
			return nil
		}
		relNative, err := filepath.Rel(root, candidate)
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(relNative)
		if rel == ".git" {
			if dirEntry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !dirEntry.IsDir() && icdomain.IsWorkspaceLockFile(rel) {
			return nil
		}
		nodes++
		if nodes > limits.MaxFiles {
			return fmt.Errorf("package exceeds max path count %d", limits.MaxFiles)
		}
		if err := validatePath(rel, limits.MaxDepth, seen); err != nil {
			return err
		}
		info, err := os.Lstat(candidate)
		if err != nil {
			return err
		}
		item := entry{rel: rel, sourcePath: candidate, info: info}
		switch {
		case info.IsDir():
			item.kind, item.mode = "directory", "040000"
		case info.Mode()&os.ModeSymlink != 0:
			item.kind, item.mode = "symlink", "120000"
			item.target, err = os.Readlink(candidate)
			if err != nil {
				return fmt.Errorf("read symlink %q: %w", rel, err)
			}
			if err := validateLinkText(item.target); err != nil {
				return fmt.Errorf("unsafe symlink %q: %w", rel, err)
			}
			if int64(len(item.target)) > limits.MaxFileBytes {
				return fmt.Errorf("package symlink %q target exceeds max size %d bytes", rel, limits.MaxFileBytes)
			}
			files++
			total += int64(len(item.target))
		case info.Mode().IsRegular():
			item.kind, item.mode, item.size = "file", "100644", info.Size()
			if executableOverrides != nil {
				if executableOverrides[rel] {
					item.mode = "100755"
				}
			} else if info.Mode()&0o111 != 0 {
				item.mode = "100755"
			}
			files++
			if item.size > limits.MaxFileBytes {
				return fmt.Errorf("package file %q exceeds max size %d bytes", rel, limits.MaxFileBytes)
			}
			total += item.size
			if total > limits.MaxTreeBytes {
				return fmt.Errorf("package exceeds max total size %d bytes", limits.MaxTreeBytes)
			}
			if lfs, err := isLFSPointer(candidate, item.size); err != nil {
				return err
			} else if lfs {
				return fmt.Errorf("Git LFS pointer is unsupported at %q", rel)
			}
		default:
			return fmt.Errorf("package contains unsupported special file %q (mode %s)", rel, info.Mode())
		}
		if total > limits.MaxTreeBytes {
			return fmt.Errorf("package exceeds max total size %d bytes", limits.MaxTreeBytes)
		}
		entries = append(entries, item)
		known[rel] = item
		return nil
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("inspect package tree: %w", err)
	}
	for _, item := range entries {
		if item.kind == "symlink" {
			if err := resolveInternalLink(item.rel, item.target, known); err != nil {
				return nil, 0, 0, err
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].rel < entries[j].rel })
	return entries, files, total, nil
}

func validatePath(rel string, maxDepth int, seen map[string]string) error {
	if rel == "" || rel == "." || strings.HasPrefix(rel, "/") || strings.ContainsRune(rel, 0) || !utf8.ValidString(rel) || !norm.NFC.IsNormalString(rel) {
		return fmt.Errorf("package contains non-portable path %q", rel)
	}
	parts := strings.Split(rel, "/")
	if len(parts) > maxDepth {
		return fmt.Errorf("package path %q exceeds max depth %d", rel, maxDepth)
	}
	for _, part := range parts {
		if strings.EqualFold(part, ".git") {
			return fmt.Errorf("package contains reserved Git metadata path %q", rel)
		}
		if err := pathpolicy.ValidatePortablePathSegment(part); err != nil {
			return fmt.Errorf("package contains unsafe path %q: %w", rel, err)
		}
	}
	if strings.EqualFold(rel, icdomain.WorkspaceLockFileName) || strings.EqualFold(rel, icdomain.LegacyWorkspaceLockFileName) {
		return fmt.Errorf("package contains reserved ownership-marker path %q", rel)
	}
	canonical := cases.Fold().String(norm.NFC.String(rel))
	if prior, exists := seen[canonical]; exists && prior != rel {
		return fmt.Errorf("package paths %q and %q collide by case or Unicode normalization", prior, rel)
	}
	seen[canonical] = rel
	return nil
}

func validateLinkText(target string) error {
	if target == "" || !utf8.ValidString(target) || strings.ContainsRune(target, 0) || strings.Contains(target, `\`) || path.IsAbs(target) || filepath.VolumeName(target) != "" {
		return fmt.Errorf("target must be a non-empty relative UTF-8 path using forward slashes")
	}
	return nil
}

func resolveInternalLink(linkRel, target string, known map[string]entry) error {
	// Resolve each link before interpreting the following ".." component.
	// Lexically cleaning first can conceal traversal out of the package through
	// an otherwise valid directory link, or conceal a missing/file ancestor.
	var resolved []string
	if parent := path.Dir(linkRel); parent != "." {
		resolved = strings.Split(parent, "/")
	}
	kind := "directory"
	// Each frame is an unprocessed suffix. Keeping frames instead of building
	// expanded paths bounds allocation even when links repeatedly reference links.
	pending := []string{target}
	links := 0
	for len(pending) > 0 {
		last := len(pending) - 1
		component, rest, more := strings.Cut(pending[last], "/")
		if more {
			pending[last] = rest
		} else {
			pending = pending[:last]
		}
		if kind != "directory" {
			return fmt.Errorf("symlink %q traverses a non-directory package path", linkRel)
		}
		switch component {
		case "", ".":
			continue
		case "..":
			if len(resolved) == 0 {
				return fmt.Errorf("symlink %q escapes the package root", linkRel)
			}
			resolved = resolved[:len(resolved)-1]
			continue
		}
		current := strings.Join(append(resolved, component), "/")
		item, ok := known[current]
		if !ok {
			return fmt.Errorf("symlink %q targets missing package path %q", linkRel, current)
		}
		if item.kind == "symlink" {
			if links >= 128 {
				return fmt.Errorf("symlink %q contains a cycle or excessive chain", linkRel)
			}
			links++
			pending = append(pending, item.target)
			continue
		}
		resolved = append(resolved, component)
		kind = item.kind
	}
	return nil
}

func isLFSPointer(filename string, size int64) (bool, error) {
	if size < 40 || size > 1024 {
		return false, nil
	}
	file, err := os.Open(filename)
	if err != nil {
		return false, fmt.Errorf("inspect possible LFS pointer: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, 1025))
	if !scanner.Scan() {
		return false, scanner.Err()
	}
	return strings.TrimSuffix(scanner.Text(), "\r") == "version https://git-lfs.github.com/spec/v1", scanner.Err()
}

func copyAndHash(item entry, destination string, digest hash.Hash) error {
	source, err := os.Open(item.sourcePath)
	if err != nil {
		return fmt.Errorf("open package file %q: %w", item.rel, err)
	}
	defer source.Close()
	opened, err := source.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(opened, item.info) || opened.Size() != item.size {
		return fmt.Errorf("package file %q changed while snapshotting", item.rel)
	}
	mode := os.FileMode(0o600)
	if item.mode == "100755" {
		mode = 0o700
	}
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("create snapshot file %q: %w", item.rel, err)
	}
	writeEntryHeader(digest, item, item.size)
	written, copyErr := io.Copy(io.MultiWriter(out, digest), io.LimitReader(source, item.size+1))
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("copy package file %q: %w", item.rel, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close snapshot file %q: %w", item.rel, closeErr)
	}
	if written != item.size {
		return fmt.Errorf("package file %q changed size while snapshotting", item.rel)
	}
	return nil
}

func writeEntryHeader(digest hash.Hash, item entry, contentLength int64) {
	frame(digest, []byte("entry"))
	frame(digest, []byte(item.rel))
	frame(digest, []byte(item.kind))
	frame(digest, []byte(item.mode))
	frame(digest, []byte(item.target))
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(contentLength))
	_, _ = digest.Write(length[:])
}

func frame(writer io.Writer, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = writer.Write(value)
}

func seal(root string, entries []entry) error {
	for index := len(entries) - 1; index >= 0; index-- {
		item := entries[index]
		if item.kind == "symlink" {
			continue
		}
		mode := os.FileMode(0o444)
		if item.kind == "directory" || item.mode == "100755" {
			mode = 0o555
		}
		if err := os.Chmod(filepath.Join(root, filepath.FromSlash(item.rel)), mode); err != nil {
			return fmt.Errorf("seal snapshot path %q: %w", item.rel, err)
		}
	}
	if err := os.Chmod(root, 0o555); err != nil {
		return fmt.Errorf("seal snapshot root: %w", err)
	}
	return nil
}

func makeWritable(root string) error {
	return filepath.WalkDir(root, func(candidate string, item os.DirEntry, err error) error {
		if err == nil && item.IsDir() {
			_ = os.Chmod(candidate, 0o700)
		}
		return nil
	})
}
