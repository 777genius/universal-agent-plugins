package environmentstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (store Store) Get(ctx context.Context, algorithm, digest string) (Package, error) {
	pkg, snapshot, err := store.load(ctx, algorithm, digest)
	if err != nil {
		return Package{}, err
	}
	if err := packagedigest.Remove(snapshot); err != nil {
		return Package{}, err
	}
	return pkg, nil
}

// Snapshot returns disposable verified bytes for a single local observation.
// The caller must release it with packagedigest.Remove. Source is advisory
// provenance, not authority for the current lock or permission to acquire data.
func (store Store) Snapshot(ctx context.Context, algorithm, digest string) (domain.PackageSnapshot, error) {
	_, snapshot, err := store.load(ctx, algorithm, digest)
	return snapshot, err
}

func (store Store) load(ctx context.Context, algorithm, digest string) (Package, domain.PackageSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return Package{}, domain.PackageSnapshot{}, err
	}
	if err := store.validate(); err != nil {
		return Package{}, domain.PackageSnapshot{}, err
	}
	destination, err := store.entryPath(algorithm, digest)
	if err != nil {
		return Package{}, domain.PackageSnapshot{}, err
	}
	pkg, snapshot, exists, err := store.inspectSnapshot(ctx, destination)
	if err != nil {
		return Package{}, domain.PackageSnapshot{}, err
	}
	if !exists {
		return Package{}, domain.PackageSnapshot{}, ErrMissing
	}
	if pkg.Metadata.DigestAlgorithm != algorithm || pkg.Metadata.TreeDigest != digest {
		return Package{}, domain.PackageSnapshot{}, discardSnapshot(snapshot, ErrMismatch)
	}
	return pkg, snapshot, nil
}

func (store Store) inspect(ctx context.Context, entryRoot string) (Package, bool, error) {
	pkg, snapshot, exists, err := store.inspectSnapshot(ctx, entryRoot)
	if err != nil || !exists {
		return pkg, exists, err
	}
	if err := packagedigest.Remove(snapshot); err != nil {
		return Package{}, false, err
	}
	return pkg, true, nil
}

func (store Store) inspectSnapshot(ctx context.Context, entryRoot string) (Package, domain.PackageSnapshot, bool, error) {
	if err := ctx.Err(); err != nil {
		return Package{}, domain.PackageSnapshot{}, false, err
	}
	body, err := atomicfile.ReadBoundedRegularFile(filepath.Join(entryRoot, "metadata.json"), MaxMetadataBytes)
	if errors.Is(err, os.ErrNotExist) {
		if _, statErr := os.Lstat(entryRoot); statErr == nil {
			return Package{}, domain.PackageSnapshot{}, false, fmt.Errorf("%w: metadata is missing from an existing entry", ErrMismatch)
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return Package{}, domain.PackageSnapshot{}, false, statErr
		}
		// Windows can report PATH_NOT_FOUND when an intermediate component is
		// a file. Do not describe that broken storage route as a missing entry.
		return Package{}, domain.PackageSnapshot{}, false, checkMissingEntryParents(entryRoot)
	}
	if err != nil {
		if errors.Is(err, atomicfile.ErrUnsafeFile) || errors.Is(err, atomicfile.ErrFileTooLarge) || errors.Is(err, atomicfile.ErrFileChanged) {
			err = fmt.Errorf("%w: unsafe environment store metadata: %w", ErrMismatch, err)
		}
		return Package{}, domain.PackageSnapshot{}, false, err
	}
	metadata, err := decodeMetadata(body)
	if err != nil {
		return Package{}, domain.PackageSnapshot{}, false, err
	}
	if err := ctx.Err(); err != nil {
		return Package{}, domain.PackageSnapshot{}, false, err
	}
	packageRoot := filepath.Join(entryRoot, "package")
	snapshot, err := (packagedigest.Builder{TempRoot: store.tempRoot()}).SnapshotWithExecutables(ctx, packageRoot, metadata.Source, metadata.ExecutableFiles)
	if err != nil {
		return Package{}, domain.PackageSnapshot{}, false, snapshotVerificationError(err)
	}
	// Overrides are only requests: the builder ignores names not encountered as
	// actual regular files. Compare its returned members before trusting metadata.
	if snapshot.TreeDigest != metadata.TreeDigest || snapshot.DigestAlgorithm != metadata.DigestAlgorithm || !equalStrings(metadata.ExecutableFiles, snapshot.ExecutableFiles) {
		return Package{}, domain.PackageSnapshot{}, false, discardSnapshot(snapshot, ErrMismatch)
	}
	if err := ctx.Err(); err != nil {
		return Package{}, domain.PackageSnapshot{}, false, discardSnapshot(snapshot, err)
	}
	return Package{Root: packageRoot, Metadata: metadata}, snapshot, true, nil
}

// Only corrupt contents authorize replacement. Permissions, temporary-storage
// failures and cancellation must not be disguised as repairable corruption.
func snapshotVerificationError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var pathError *os.PathError
	if errors.As(err, &pathError) && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrMismatch, err)
}

func discardSnapshot(snapshot domain.PackageSnapshot, cause error) error {
	if err := packagedigest.Remove(snapshot); err != nil {
		// A cleanup I/O failure must not authorize quarantine through ErrMismatch.
		return fmt.Errorf("discard verification snapshot after %v: %w", cause, err)
	}
	return cause
}

func checkMissingEntryParents(entryRoot string) error {
	for parent := filepath.Dir(entryRoot); ; parent = filepath.Dir(parent) {
		info, err := os.Stat(parent)
		if err == nil {
			if !info.IsDir() {
				return &os.PathError{Op: "inspect", Path: parent, Err: errors.New("store ancestor is not a directory")}
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) || parent == filepath.Dir(parent) {
			return err
		}
	}
}
