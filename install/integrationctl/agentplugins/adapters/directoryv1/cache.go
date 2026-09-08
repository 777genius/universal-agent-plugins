package directoryv1

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

type Cache struct {
	// Path is the single atomic schema-1 last-known-good record.
	Path string
	// BeforeRename is a disposable-fixture fault injection hook. Production
	// callers leave it nil.
	BeforeRename func(temporaryPath string) error
}

type cacheRecord struct {
	SchemaVersion int    `json:"schema_version"`
	Sequence      uint64 `json:"sequence"`
	Snapshot      string `json:"snapshot"`
	Envelope      string `json:"envelope"`
}

var ErrCacheRollback = errors.New("directory cache would roll back last-known-good sequence")

func (cache Cache) Load(trust TrustStore) (VerifiedBundle, error) {
	if cache.Path == "" {
		return VerifiedBundle{}, os.ErrNotExist
	}
	info, err := os.Lstat(cache.Path)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return VerifiedBundle{}, fmt.Errorf("directory cache must be a restrictive regular file")
	}
	file, err := os.Open(cache.Path)
	if err != nil {
		return VerifiedBundle{}, err
	}
	defer file.Close()
	// The cache record base64-encodes both exact artifacts, so its bound must be
	// calculated from their encoded lengths rather than their raw wire limits.
	maxRecordBytes := base64.StdEncoding.EncodedLen(MaxSnapshotBytes) + base64.StdEncoding.EncodedLen(MaxEnvelopeBytes) + 4096
	body, err := io.ReadAll(io.LimitReader(file, int64(maxRecordBytes)+1))
	if err != nil {
		return VerifiedBundle{}, err
	}
	if len(body) > maxRecordBytes {
		return VerifiedBundle{}, fmt.Errorf("invalid directory cache: record is oversized")
	}
	var record cacheRecord
	if err := strictDecode(body, &record, maxRecordBytes); err != nil {
		return VerifiedBundle{}, fmt.Errorf("invalid directory cache: %w", err)
	}
	if err := requireRootFields(body, "schema_version", "sequence", "snapshot", "envelope"); err != nil {
		return VerifiedBundle{}, fmt.Errorf("invalid directory cache: %w", err)
	}
	if record.SchemaVersion != SnapshotSchemaVersion || record.Sequence < 1 {
		return VerifiedBundle{}, fmt.Errorf("invalid directory cache identity")
	}
	snapshot, err := base64.StdEncoding.Strict().DecodeString(record.Snapshot)
	if err != nil {
		return VerifiedBundle{}, fmt.Errorf("invalid cached snapshot encoding: %w", err)
	}
	envelope, err := base64.StdEncoding.Strict().DecodeString(record.Envelope)
	if err != nil {
		return VerifiedBundle{}, fmt.Errorf("invalid cached envelope encoding: %w", err)
	}
	bundle, err := VerifyBundle(snapshot, envelope, trust)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if bundle.Snapshot.Sequence != record.Sequence {
		return VerifiedBundle{}, ErrSequenceMismatch
	}
	bundle.Source = BundleSourceCache
	return bundle, nil
}

// Store re-verifies exact bytes immediately before an atomic replacement.
// Lower records never replace the current valid LKG. An equal sequence is
// idempotent only when it authenticates the same snapshot digest; otherwise it
// is publication equivocation and must fail closed.
func (cache Cache) Store(candidate VerifiedBundle, trust TrustStore) error {
	verified, err := VerifyBundle(candidate.SnapshotBytes, candidate.EnvelopeBytes, trust)
	if err != nil {
		return err
	}
	authoritative, err := cache.reconcileVerified(verified, trust, true)
	if err == nil && authoritative.Snapshot.Sequence > verified.Snapshot.Sequence {
		return ErrCacheRollback
	}
	return err
}

// Reconcile linearizes a verified candidate with the process-shared cache and
// returns the authoritative last-known-good bundle observed while holding the
// OS lock. A caller must use the returned bundle rather than the candidate:
// another process may have advanced the cache after the caller's initial read.
// A same-sequence/different-digest publication is always returned as a
// security error and is never hidden by a fallback.
func (cache Cache) Reconcile(candidate VerifiedBundle, trust TrustStore) (VerifiedBundle, error) {
	verified, err := VerifyBundle(candidate.SnapshotBytes, candidate.EnvelopeBytes, trust)
	if err != nil {
		return VerifiedBundle{}, err
	}
	return cache.reconcileVerified(verified, trust, true)
}

// Observe linearizes a local fallback with the cache without persisting an
// embedded-only candidate. This preserves the bootstrap/cache separation while
// still observing a concurrent higher or conflicting cache publication.
func (cache Cache) Observe(candidate VerifiedBundle, trust TrustStore) (VerifiedBundle, error) {
	verified, err := VerifyBundle(candidate.SnapshotBytes, candidate.EnvelopeBytes, trust)
	if err != nil {
		return VerifiedBundle{}, err
	}
	verified.Source = candidate.Source
	return cache.reconcileVerified(verified, trust, false)
}

func (cache Cache) reconcileVerified(verified VerifiedBundle, trust TrustStore, persist bool) (VerifiedBundle, error) {
	if cache.Path == "" {
		return VerifiedBundle{}, errors.New("directory cache path is empty")
	}
	directory := filepath.Dir(cache.Path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return VerifiedBundle{}, err
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return VerifiedBundle{}, err
	}
	// The lock deliberately covers the read/compare and the complete durable
	// replacement. Without that span, two independently verified writers can
	// both compare against N and then publish N+2 followed by N+1.
	unlock, err := lockCache(cache.Path + ".lock")
	if err != nil {
		return VerifiedBundle{}, fmt.Errorf("lock directory cache: %w", err)
	}
	defer unlock()
	if current, loadErr := cache.Load(trust); loadErr == nil && current.Snapshot.Sequence >= verified.Snapshot.Sequence {
		if current.Snapshot.Sequence > verified.Snapshot.Sequence {
			return current, nil
		}
		if current.Digest != verified.Digest {
			return VerifiedBundle{}, fmt.Errorf("%w: sequence %d", ErrSequenceConflict, verified.Snapshot.Sequence)
		}
		return current, nil
	}
	if !persist {
		return verified, nil
	}
	record := cacheRecord{SchemaVersion: SnapshotSchemaVersion, Sequence: verified.Snapshot.Sequence, Snapshot: base64.StdEncoding.EncodeToString(verified.SnapshotBytes), Envelope: base64.StdEncoding.EncodeToString(verified.EnvelopeBytes)}
	body, err := json.Marshal(record)
	if err != nil {
		return VerifiedBundle{}, err
	}
	body = append(body, '\n')
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return VerifiedBundle{}, err
	}
	temporary := filepath.Join(directory, "."+filepath.Base(cache.Path)+"."+hex.EncodeToString(nonce[:])+".tmp")
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return VerifiedBundle{}, err
	}
	ok := false
	defer func() {
		file.Close()
		if !ok {
			_ = os.Remove(temporary)
		}
	}()
	if _, err = file.Write(body); err != nil {
		return VerifiedBundle{}, err
	}
	if err = file.Sync(); err != nil {
		return VerifiedBundle{}, err
	}
	if err = file.Close(); err != nil {
		return VerifiedBundle{}, err
	}
	if cache.BeforeRename != nil {
		if err := cache.BeforeRename(temporary); err != nil {
			return VerifiedBundle{}, err
		}
	}
	if err = os.Rename(temporary, cache.Path); err != nil {
		return VerifiedBundle{}, err
	}
	ok = true
	if err := atomicfile.SyncDirectory(directory); err != nil {
		return VerifiedBundle{}, err
	}
	verified.Source = BundleSourceCache
	return verified, nil
}
