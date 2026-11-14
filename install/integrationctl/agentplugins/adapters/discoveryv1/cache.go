package discoveryv1

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
	Path         string
	BeforeRename func(string) error
}

type cacheRecord struct {
	SchemaVersion int    `json:"schema_version"`
	Sequence      uint64 `json:"sequence"`
	Pointer       string `json:"pointer"`
	Snapshot      string `json:"snapshot"`
	Envelope      string `json:"envelope"`
	Search        string `json:"search"`
}

var ErrCacheRollback = errors.New("Discovery cache would roll back last-known-good sequence")

func (cache Cache) Load(trust TrustStore) (VerifiedBundle, error) {
	if cache.Path == "" {
		return VerifiedBundle{}, os.ErrNotExist
	}
	info, err := os.Lstat(cache.Path)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return VerifiedBundle{}, errors.New("Discovery cache must be a restrictive regular file")
	}
	file, err := os.Open(cache.Path)
	if err != nil {
		return VerifiedBundle{}, err
	}
	defer file.Close()
	maximum := base64.StdEncoding.EncodedLen(MaxLatestBytes+MaxSnapshotBytes+MaxEnvelopeBytes+MaxSearchBytes) + 4096
	body, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	if err != nil || len(body) > maximum {
		return VerifiedBundle{}, errors.New("invalid Discovery cache: oversized or unreadable")
	}
	var record cacheRecord
	if err := strictDecode(body, &record, maximum); err != nil {
		return VerifiedBundle{}, fmt.Errorf("invalid Discovery cache: %w", err)
	}
	if err := exactObject(body, []string{"schema_version", "sequence", "pointer", "snapshot", "envelope", "search"}); err != nil || record.SchemaVersion != 1 || record.Sequence < 1 {
		return VerifiedBundle{}, errors.New("invalid Discovery cache identity")
	}
	decode := func(label, value string) ([]byte, error) {
		decoded, err := base64.StdEncoding.Strict().DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("invalid cached %s encoding: %w", label, err)
		}
		return decoded, nil
	}
	pointer, err := decode("pointer", record.Pointer)
	if err != nil {
		return VerifiedBundle{}, err
	}
	snapshot, err := decode("snapshot", record.Snapshot)
	if err != nil {
		return VerifiedBundle{}, err
	}
	envelope, err := decode("envelope", record.Envelope)
	if err != nil {
		return VerifiedBundle{}, err
	}
	search, err := decode("search", record.Search)
	if err != nil {
		return VerifiedBundle{}, err
	}
	bundle, err := VerifyBundle(pointer, snapshot, envelope, search, trust)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if bundle.Snapshot.Sequence != record.Sequence {
		return VerifiedBundle{}, ErrSequenceMismatch
	}
	bundle.Source = BundleSourceCache
	return bundle, nil
}

func (cache Cache) Store(candidate VerifiedBundle, trust TrustStore) error {
	verified, err := VerifyBundle(candidate.PointerBytes, candidate.SnapshotBytes, candidate.EnvelopeBytes, candidate.SearchBytes, trust)
	if err != nil {
		return err
	}
	authoritative, err := cache.reconcile(verified, trust, true)
	if err == nil && authoritative.Snapshot.Sequence > verified.Snapshot.Sequence {
		return ErrCacheRollback
	}
	return err
}

func (cache Cache) Reconcile(candidate VerifiedBundle, trust TrustStore) (VerifiedBundle, error) {
	verified, err := VerifyBundle(candidate.PointerBytes, candidate.SnapshotBytes, candidate.EnvelopeBytes, candidate.SearchBytes, trust)
	if err != nil {
		return VerifiedBundle{}, err
	}
	return cache.reconcile(verified, trust, true)
}

func (cache Cache) Observe(candidate VerifiedBundle, trust TrustStore) (VerifiedBundle, error) {
	verified, err := VerifyBundle(candidate.PointerBytes, candidate.SnapshotBytes, candidate.EnvelopeBytes, candidate.SearchBytes, trust)
	if err != nil {
		return VerifiedBundle{}, err
	}
	verified.Source = candidate.Source
	return cache.reconcile(verified, trust, false)
}

func (cache Cache) reconcile(verified VerifiedBundle, trust TrustStore, persist bool) (VerifiedBundle, error) {
	if cache.Path == "" {
		return VerifiedBundle{}, errors.New("Discovery cache path is empty")
	}
	directory := filepath.Dir(cache.Path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return VerifiedBundle{}, err
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return VerifiedBundle{}, err
	}
	unlock, err := lockCache(cache.Path + ".lock")
	if err != nil {
		return VerifiedBundle{}, err
	}
	defer unlock()
	if current, loadErr := cache.Load(trust); loadErr == nil && current.Snapshot.Sequence >= verified.Snapshot.Sequence {
		if current.Snapshot.Sequence == verified.Snapshot.Sequence && current.Digest != verified.Digest {
			return VerifiedBundle{}, fmt.Errorf("%w: sequence %d", ErrSequenceConflict, verified.Snapshot.Sequence)
		}
		return current, nil
	}
	if !persist {
		return verified, nil
	}
	record := cacheRecord{
		SchemaVersion: 1, Sequence: verified.Snapshot.Sequence,
		Pointer: base64.StdEncoding.EncodeToString(verified.PointerBytes), Snapshot: base64.StdEncoding.EncodeToString(verified.SnapshotBytes),
		Envelope: base64.StdEncoding.EncodeToString(verified.EnvelopeBytes), Search: base64.StdEncoding.EncodeToString(verified.SearchBytes),
	}
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
		_ = file.Close()
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
