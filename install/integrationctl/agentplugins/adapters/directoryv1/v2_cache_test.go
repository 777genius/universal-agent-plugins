package directoryv1

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestV2CacheIsIsolatedFromV1AndRetainsSignedBytes(t *testing.T) {
	snapshot, envelope, trust := v2SignedFixture(t)
	valid, err := VerifyV2Bundle(snapshot, envelope, trust)
	if err != nil {
		t.Fatal(err)
	}
	cache := Cache{Path: filepath.Join(t.TempDir(), "directory-v2-cache.json")}
	if err := cache.StoreV2(valid, trust); err != nil {
		t.Fatal(err)
	}
	read, err := cache.LoadV2(trust)
	if err != nil || read.Digest != valid.Digest || read.Source != BundleSourceCache {
		t.Fatalf("v2 cached signature differs: %+v %v", read, err)
	}
	if _, err := cache.Load(trust); err == nil {
		t.Fatal("v1 decoder accepted v2 last-known-good cache")
	}
	before, err := os.ReadFile(cache.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cache.ReconcileV2(read, trust); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(cache.Path)
	if err != nil || string(before) != string(after) {
		t.Fatalf("identical v2 snapshot rewrote cache: %v", err)
	}

	// Equivocation for the same v2 sequence cannot replace authenticated LKG.
	changed := valid.Snapshot
	changed.PublicationID = "another-publication"
	bytes, _ := json.Marshal(changed)
	_, private := fixtureKey("v2-test", KeyCurrent, 18)
	changedBundle := signedV2TestBundle(t, bytes, private, trust)
	if _, err := cache.ReconcileV2(changedBundle, trust); !errors.Is(err, ErrSequenceConflict) {
		t.Fatalf("same-sequence equivocation accepted: %v", err)
	}

	// A corrupt existing record must not be silently replaced by a valid
	// incoming publication (or ignored by read-only observation).
	corrupt := []byte("{}\n")
	if err := os.WriteFile(cache.Path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if err := cache.StoreV2(valid, trust); err == nil {
		t.Fatal("v2 replaced an unreadable signed cache")
	}
	if _, err := cache.ObserveV2(valid, trust); err == nil {
		t.Fatal("v2 read-only observation ignored unreadable signed cache")
	}
	after, err = os.ReadFile(cache.Path)
	if err != nil || string(after) != string(corrupt) {
		t.Fatalf("corrupt authority was overwritten: %v", err)
	}
}

func signedV2TestBundle(t *testing.T, body []byte, private ed25519.PrivateKey, trust TrustStore) VerifiedBundle {
	t.Helper()
	sum := sha256.Sum256(body)
	envelope := Envelope{
		EnvelopeSchemaVersion: 2, SnapshotSchemaVersion: 2,
		Sequence: 1, KeyID: "v2-test", Algorithm: "Ed25519",
		SignatureDomain: SignatureDomainV2, SnapshotDigest: "sha256:" + hex.EncodeToString(sum[:]),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(private, SnapshotSignatureMessageV2(body))),
	}
	envBody, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	b, err := VerifyV2Bundle(body, envBody, trust)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
