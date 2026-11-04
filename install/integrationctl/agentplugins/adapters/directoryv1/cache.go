package directoryv1

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

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

type cacheVerifier func([]byte, []byte, TrustStore) (VerifiedBundle, error)

func (cache Cache) Load(trust TrustStore) (VerifiedBundle, error) {
	return cache.loadVerified(trust, VerifyBundle, SnapshotSchemaVersion)
}

// LoadV2 reads an independent cache record using only the schema-2 signature
// domain. Callers must pass a separate cache path and enforce its own floor.
func (cache Cache) LoadV2(trust TrustStore) (VerifiedBundle, error) {
	return cache.loadVerified(trust, VerifyV2Bundle, 2)
}

func (cache Cache) loadVerified(trust TrustStore, verifier cacheVerifier, schema int) (VerifiedBundle, error) {
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
	if record.SchemaVersion != schema || record.Sequence < 1 {
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
	bundle, err := verifier(snapshot, envelope, trust)
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

// StoreV2 re-verifies a separately signed v2 candidate before an atomic
// cache update. It does not alter schema-1 cache semantics or file paths.
func (cache Cache) StoreV2(candidate VerifiedBundle, trust TrustStore) error {
	verified, err := VerifyV2Bundle(candidate.SnapshotBytes, candidate.EnvelopeBytes, trust)
	if err != nil {
		return err
	}
	authoritative, err := cache.reconcileVersion(verified, trust, VerifyV2Bundle, 2, true)
	if err == nil && authoritative.Snapshot.Sequence > verified.Snapshot.Sequence {
		return ErrCacheRollback
	}
	return err
}

func (cache Cache) ReconcileV2(candidate VerifiedBundle, trust TrustStore) (VerifiedBundle, error) {
	verified, err := VerifyV2Bundle(candidate.SnapshotBytes, candidate.EnvelopeBytes, trust)
	if err != nil {
		return VerifiedBundle{}, err
	}
	return cache.reconcileVersion(verified, trust, VerifyV2Bundle, 2, true)
}

func (cache Cache) ObserveV2(candidate VerifiedBundle, trust TrustStore) (VerifiedBundle, error) {
	verified, err := VerifyV2Bundle(candidate.SnapshotBytes, candidate.EnvelopeBytes, trust)
	if err != nil {
		return VerifiedBundle{}, err
	}
	verified.Source = candidate.Source
	return cache.reconcileVersion(verified, trust, VerifyV2Bundle, 2, false)
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
	return cache.reconcileVersion(verified, trust, VerifyBundle, SnapshotSchemaVersion, persist)
}
