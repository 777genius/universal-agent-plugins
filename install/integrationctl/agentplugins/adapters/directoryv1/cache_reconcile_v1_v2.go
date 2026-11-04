package directoryv1

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

// v1 and v2 use the same atomic cache reconciliation with distinct
// authenticated verifiers, schemas and caller-selected storage paths.
func (cache Cache) reconcileVersion(verified VerifiedBundle, trust TrustStore, verifier cacheVerifier, schema int, persist bool) (VerifiedBundle, error) {
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
	current, loadErr := cache.loadVerified(trust, verifier, schema)
	// V2 must never overwrite a schema-1 cache accidentally reused at the
	// same path (or replace corrupted signed authority with a fresh snapshot).
	if schema == 2 && loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
		return VerifiedBundle{}, fmt.Errorf("refuse replacement of unreadable v2 cache: %w", loadErr)
	}
	if loadErr == nil && current.Snapshot.Sequence >= verified.Snapshot.Sequence {
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
	// The process-shared lock remains held through the durable replacement.
	return cache.writeVersionedRecord(verified, schema, directory)
}

func (cache Cache) writeVersionedRecord(verified VerifiedBundle, schema int, directory string) (VerifiedBundle, error) {
	record := cacheRecord{SchemaVersion: schema, Sequence: verified.Snapshot.Sequence, Snapshot: base64.StdEncoding.EncodeToString(verified.SnapshotBytes), Envelope: base64.StdEncoding.EncodeToString(verified.EnvelopeBytes)}
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
