package directoryv1

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// V2Client has a separate authenticated bootstrap and last-known-good cache.
// Its transport reuses the existing bounded same-origin HTTP client; the
// production schema-1 client is not changed or automatically downgraded.
type V2Client struct {
	Transport Client
	Cache     Cache
	Embedded  EmbeddedBundle
}

func (V2Client) SnapshotSchema() int { return 2 }

// validateConfig keeps read-only inspection on the same signed trust and
// isolated cache authority as install-time loading.
func (client V2Client) validateConfig() error {
	if len(client.Transport.Trust.Keys) == 0 {
		return ErrNoTrustedKeys
	}
	if client.Cache.Path == "" {
		return fmt.Errorf("%w: v2 cache path is required", ErrUnavailable)
	}
	if client.Transport.Cache.Path != "" {
		v1Path, err := canonicalCachePath(client.Transport.Cache.Path)
		if err != nil {
			return fmt.Errorf("%w: resolve v1 cache path: %w", ErrUnavailable, err)
		}
		v2Path, err := canonicalCachePath(client.Cache.Path)
		if err != nil {
			return fmt.Errorf("%w: resolve v2 cache path: %w", ErrUnavailable, err)
		}
		if v1Path == v2Path {
			return fmt.Errorf("%w: v1/v2 cache path collision", ErrUnavailable)
		}
		// Distinct paths can still name the same cache inode through hard links.
		v1Info, v1Err := os.Stat(client.Transport.Cache.Path)
		if v1Err != nil && !errors.Is(v1Err, os.ErrNotExist) {
			return fmt.Errorf("%w: inspect v1 cache: %w", ErrUnavailable, v1Err)
		}
		v2Info, v2Err := os.Stat(client.Cache.Path)
		if v2Err != nil && !errors.Is(v2Err, os.ErrNotExist) {
			return fmt.Errorf("%w: inspect v2 cache: %w", ErrUnavailable, v2Err)
		}
		if v1Err == nil && v2Err == nil && os.SameFile(v1Info, v2Info) {
			return fmt.Errorf("%w: v1/v2 cache path collision", ErrUnavailable)
		}
	}
	return nil
}

// canonicalCachePath resolves existing parent aliases even when the cache file
// does not exist yet. Keep v1 and v2 from writing the same physical LKG path.
func canonicalCachePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	// A symlink at the cache filename itself must also resolve to its target.
	// Reject dangling filename symlinks instead of following them on write.
	if info, err := os.Lstat(absolute); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return filepath.EvalSymlinks(absolute)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	parent, suffix := filepath.Dir(absolute), filepath.Base(absolute)
	for {
		resolved, err := filepath.EvalSymlinks(parent)
		if err == nil {
			return filepath.Join(resolved, suffix), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		ancestor := filepath.Dir(parent)
		if ancestor == parent {
			return "", err
		}
		suffix = filepath.Join(filepath.Base(parent), suffix)
		parent = ancestor
	}
}

// LoadLocal returns signed v2 provenance for read-only inspection without
// network access or writes. As for v1, expired data may be displayed but must
// never authorize installation or locked restore.
func (client V2Client) LoadLocal(installedFloor uint64) (VerifiedBundle, error) {
	if err := client.validateConfig(); err != nil {
		return VerifiedBundle{}, err
	}
	embedded, cached, floor, err := client.loadLocal(installedFloor)
	if err != nil {
		return VerifiedBundle{}, err
	}
	best := VerifiedBundle{}
	for _, candidate := range []VerifiedBundle{cached, embedded} {
		if candidate.Snapshot.Sequence >= floor && candidate.Snapshot.Sequence > best.Snapshot.Sequence {
			best = candidate
		}
	}
	if best.Snapshot.Sequence > 0 {
		return best, nil
	}
	if floor > 0 {
		return VerifiedBundle{}, fmt.Errorf("%w: local floor %d", ErrRollback, floor)
	}
	return VerifiedBundle{}, ErrUnavailable
}

func (client V2Client) loadLocal(floor uint64) (VerifiedBundle, VerifiedBundle, uint64, error) {
	trust := client.Transport.Trust
	var embedded VerifiedBundle
	if len(client.Embedded.Snapshot) > 0 || len(client.Embedded.Envelope) > 0 ||
		client.Transport.RequireEmbeddedBootstrap {
		bundle, err := VerifyV2Bundle(client.Embedded.Snapshot, client.Embedded.Envelope, trust)
		if err != nil {
			if client.Transport.RequireEmbeddedBootstrap {
				return embedded, VerifiedBundle{}, 0, fmt.Errorf("%w: %w", ErrBootstrapNotReady, err)
			}
		} else {
			embedded = bundle
			embedded.Source = BundleSourceEmbedded
		}
	}
	cached, err := client.Cache.LoadV2(trust)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return embedded, VerifiedBundle{}, 0, fmt.Errorf("invalid scoped directory cache: %w", err)
	}
	if sameSequenceDifferentDigest(embedded, cached) {
		return embedded, cached, 0, fmt.Errorf("%w: embedded/cache sequence %d", ErrSequenceConflict, embedded.Snapshot.Sequence)
	}
	for _, seq := range []uint64{embedded.Snapshot.Sequence, cached.Snapshot.Sequence} {
		if seq > floor {
			floor = seq
		}
	}
	return embedded, cached, floor, nil
}

func (client V2Client) reconcileRemote(remote, embedded, cached VerifiedBundle, floor uint64, now time.Time) (VerifiedBundle, error) {
	if remote.Snapshot.Sequence < floor {
		return VerifiedBundle{}, fmt.Errorf("%w: v2 remote %d, local floor %d", ErrRollback, remote.Snapshot.Sequence, floor)
	}
	if sameSequenceDifferentDigest(remote, cached) || sameSequenceDifferentDigest(remote, embedded) {
		return VerifiedBundle{}, fmt.Errorf("%w: v2 remote conflicts with local authority", ErrSequenceConflict)
	}
	if err := validityErr(remote.Snapshot, now); err != nil {
		return VerifiedBundle{}, err
	}
	authoritative, err := client.Cache.ReconcileV2(remote, client.Transport.Trust)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if authoritative.Snapshot.Sequence < floor || sameSequenceDifferentDigest(authoritative, embedded) {
		return VerifiedBundle{}, ErrRollback
	}
	if err := validityErr(authoritative.Snapshot, now); err != nil {
		return VerifiedBundle{}, err
	}
	if authoritative.Snapshot.Sequence == remote.Snapshot.Sequence && authoritative.Digest == remote.Digest {
		authoritative.Source = BundleSourceRemote
	}
	return authoritative, nil
}

func (client V2Client) fallback(embedded, cached VerifiedBundle, floor uint64, now time.Time, remoteErr error) (VerifiedBundle, error) {
	best := VerifiedBundle{}
	for _, candidate := range []VerifiedBundle{cached, embedded} {
		if candidate.Snapshot.Sequence >= floor && candidate.Snapshot.Sequence > 0 &&
			validityErr(candidate.Snapshot, now) == nil && candidate.Snapshot.Sequence > best.Snapshot.Sequence {
			best = candidate
		}
	}
	if best.Snapshot.Sequence > 0 {
		result, err := client.Cache.ObserveV2(best, client.Transport.Trust)
		if err != nil {
			return VerifiedBundle{}, err
		}
		if result.Snapshot.Sequence < floor || sameSequenceDifferentDigest(result, embedded) {
			return VerifiedBundle{}, ErrRollback
		}
		if err := validityErr(result.Snapshot, now); err != nil {
			return VerifiedBundle{}, err
		}
		return result, nil
	}
	for _, candidate := range []VerifiedBundle{cached, embedded} {
		if candidate.Snapshot.Sequence >= floor && candidate.Snapshot.Sequence > 0 {
			if err := validityErr(candidate.Snapshot, now); err != nil {
				return VerifiedBundle{}, err
			}
		}
	}
	if floor > 0 && embedded.Snapshot.Sequence < floor && cached.Snapshot.Sequence < floor {
		return VerifiedBundle{}, ErrRollback
	}
	if remoteErr != nil {
		return VerifiedBundle{}, remoteErr
	}
	return VerifiedBundle{}, ErrUnavailable
}

// Load requires a current v2 signature, independent monotonic sequence and
// the same expiry/redirect/digest safeguards as v1. No implicit v1 fallback.
func (client V2Client) Load(ctx context.Context, installedFloor uint64) (VerifiedBundle, error) {
	if err := client.validateConfig(); err != nil {
		return VerifiedBundle{}, err
	}
	embedded, cached, floor, err := client.loadLocal(installedFloor)
	if err != nil {
		return VerifiedBundle{}, err
	}
	now := time.Now().UTC()
	if client.Transport.Now != nil {
		now = client.Transport.Now().UTC()
	}
	remote, remoteErr := client.Transport.fetchV2(ctx)
	if remoteErr == nil {
		accepted, err := client.reconcileRemote(remote, embedded, cached, floor, now)
		if err == nil {
			return accepted, nil
		}
		if errors.Is(err, ErrSequenceConflict) {
			return VerifiedBundle{}, err
		}
		remoteErr = err
	}
	return client.fallback(embedded, cached, floor, now, remoteErr)
}
