// Package environmentstore publishes immutable verified package snapshots for
// dependency environments. Store bytes are reused only after digest and
// executable inventory verification.
package environmentstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var digestHex = regexp.MustCompile(`^[0-9a-f]{64}$`)

var (
	ErrMissing  = errors.New("environment store entry is missing")
	ErrMismatch = errors.New("environment store entry does not match its digest")
)

type Store struct {
	Root     string
	TempRoot string
	LockPath string
}

type Metadata struct {
	SchemaVersion   int                   `json:"schema_version"`
	DigestAlgorithm string                `json:"digest_algorithm"`
	TreeDigest      string                `json:"tree_digest"`
	ExecutableFiles []string              `json:"executable_files,omitempty"`
	Source          domain.SourceIdentity `json:"source"`
}

type Package struct {
	Root     string
	Metadata Metadata
}

func (store Store) Put(ctx context.Context, snapshot domain.PackageSnapshot) (Package, error) {
	if err := ctx.Err(); err != nil {
		return Package{}, err
	}
	if err := store.validate(); err != nil {
		return Package{}, err
	}
	metadata, err := metadataForSnapshot(snapshot)
	if err != nil {
		return Package{}, err
	}
	// The writer uses the reader's exact envelope before any persistent work.
	if _, err := encodeMetadata(metadata); err != nil {
		return Package{}, err
	}
	snapshot.ExecutableFiles = metadata.ExecutableFiles
	sealed, err := (packagedigest.Builder{TempRoot: store.tempRoot()}).SnapshotWithExecutables(ctx, snapshot.Root, snapshot.Source, snapshot.ExecutableFiles)
	if err != nil {
		return Package{}, snapshotVerificationError(err)
	}
	sealedInventory := append([]string{}, sealed.ExecutableFiles...)
	sealedDigest, sealedAlgorithm := sealed.TreeDigest, sealed.DigestAlgorithm
	if err := packagedigest.Remove(sealed); err != nil {
		return Package{}, err
	}
	if sealedDigest != snapshot.TreeDigest || sealedAlgorithm != snapshot.DigestAlgorithm || !equalStrings(sealedInventory, snapshot.ExecutableFiles) {
		return Package{}, fmt.Errorf("%w: acquired snapshot changed before publication", ErrMismatch)
	}
	lock := processlock.Lock{Path: store.lockPath()}
	release, err := lock.Acquire(ctx)
	if err != nil {
		return Package{}, err
	}
	defer func() { _ = release() }()
	destination, err := store.entryPath(snapshot.DigestAlgorithm, snapshot.TreeDigest)
	if err != nil {
		return Package{}, err
	}
	replaceCorrupt := false
	if existing, ok, err := store.inspect(ctx, destination); err != nil {
		if !errors.Is(err, ErrMismatch) {
			return Package{}, err
		}
		replaceCorrupt = true
	} else if ok {
		if existing.Metadata.TreeDigest != snapshot.TreeDigest || existing.Metadata.DigestAlgorithm != snapshot.DigestAlgorithm ||
			!equalStrings(existing.Metadata.ExecutableFiles, snapshot.ExecutableFiles) {
			return Package{}, fmt.Errorf("%w: existing entry has different immutable metadata", ErrMismatch)
		}
		return existing, nil
	}
	return store.publish(ctx, snapshot, destination, replaceCorrupt)
}

func (store Store) validate() error {
	if filepath.IsAbs(store.Root) && filepath.Clean(store.Root) == store.Root && strings.TrimSpace(store.Root) != "" {
		return nil
	}
	return fmt.Errorf("environment store root must be an explicit absolute clean path")
}

func (store Store) lockPath() string {
	if store.LockPath != "" {
		return store.LockPath
	}
	return filepath.Join(store.Root, "store.lock")
}

func (store Store) tempRoot() string {
	if store.TempRoot != "" {
		if info, err := os.Stat(store.TempRoot); err == nil && info.IsDir() {
			return store.TempRoot
		}
		return ""
	}
	return ""
}

func (store Store) entryPath(algorithm, digest string) (string, error) {
	if algorithm != domain.TreeDigestAlgorithm {
		return "", fmt.Errorf("unsupported package digest algorithm %q", algorithm)
	}
	value := strings.TrimPrefix(digest, "sha256:")
	if value == digest || !digestHex.MatchString(value) {
		return "", fmt.Errorf("invalid package tree digest %q", digest)
	}
	return filepath.Join(store.Root, "packages", algorithm, value), nil
}

func copyTree(sourceRoot, destinationRoot string, executables []string) error {
	if err := os.MkdirAll(destinationRoot, 0o700); err != nil {
		return err
	}
	source, err := os.OpenRoot(sourceRoot)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	destination, err := os.OpenRoot(destinationRoot)
	if err != nil {
		return err
	}
	defer func() { _ = destination.Close() }()
	executable := map[string]bool{}
	for _, item := range executables {
		executable[item] = true
	}
	return fs.WalkDir(source.FS(), ".", func(relative string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if relative == "." {
			return nil
		}
		return copyRootedEntry(source, destination, relative, executable[relative])
	})
}

func copyRootedEntry(source, destination *os.Root, relative string, executable bool) error {
	info, err := source.Lstat(relative)
	if err != nil {
		return err
	}
	switch {
	case info.IsDir():
		return destination.Mkdir(relative, 0o700)
	case info.Mode()&os.ModeSymlink != 0:
		link, err := source.Readlink(relative)
		if err != nil {
			return err
		}
		return destination.Symlink(link, relative)
	case info.Mode().IsRegular():
		mode := os.FileMode(0o600)
		if executable {
			mode = 0o700
		}
		return copyRootedFile(source, destination, relative, mode)
	default:
		return fmt.Errorf("unsupported package file type %q", relative)
	}
}

func copyRootedFile(source, destination *os.Root, relative string, mode os.FileMode) error {
	input, err := source.Open(relative)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	output, err := destination.OpenFile(relative, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
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

func (store Store) publish(ctx context.Context, snapshot domain.PackageSnapshot, destination string, replaceCorrupt bool) (Package, error) {
	if err := os.MkdirAll(filepath.Join(store.Root, ".staging"), 0o700); err != nil {
		return Package{}, err
	}
	stage, err := os.MkdirTemp(filepath.Join(store.Root, ".staging"), "entry-")
	if err != nil {
		return Package{}, err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	metadata, err := store.stageSnapshot(ctx, snapshot, stage)
	if err != nil {
		return Package{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return Package{}, err
	}
	quarantine := ""
	if replaceCorrupt {
		quarantine = stage + "-corrupt"
		if err := os.Rename(destination, quarantine); err != nil {
			return Package{}, err
		}
	}
	if err := os.Rename(stage, destination); err != nil {
		if quarantine != "" {
			_ = os.Rename(quarantine, destination)
		}
		if existing, ok, inspectErr := store.inspect(ctx, destination); inspectErr == nil && ok {
			return existing, nil
		}
		return Package{}, fmt.Errorf("publish environment store entry: %w", err)
	}
	if err := atomicfile.SyncDirectory(filepath.Dir(destination)); err != nil {
		return Package{}, err
	}
	// The replacement is durable now. Remove only this publication's unique quarantine;
	// earlier failure paths retain it for diagnosis or recovery.
	if quarantine != "" {
		if err := os.RemoveAll(quarantine); err != nil {
			return Package{}, fmt.Errorf("remove repaired store quarantine: %w", err)
		}
	}
	return Package{Root: filepath.Join(destination, "package"), Metadata: metadata}, nil
}

func (store Store) stageSnapshot(ctx context.Context, snapshot domain.PackageSnapshot, stage string) (Metadata, error) {
	if err := copyTree(snapshot.Root, filepath.Join(stage, "package"), snapshot.ExecutableFiles); err != nil {
		return Metadata{}, err
	}
	metadata, err := metadataForSnapshot(snapshot)
	if err != nil {
		return Metadata{}, err
	}
	body, err := encodeMetadata(metadata)
	if err != nil {
		return Metadata{}, err
	}
	if err := atomicfile.Write(filepath.Join(stage, "metadata.json"), body, 0o600); err != nil {
		return Metadata{}, err
	}
	verified, _, err := store.inspect(ctx, stage)
	if err != nil {
		return Metadata{}, err
	}
	if verified.Metadata.TreeDigest != snapshot.TreeDigest {
		return Metadata{}, fmt.Errorf("%w: staged bytes changed", ErrMismatch)
	}
	return metadata, nil
}
