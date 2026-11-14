package directoryv1

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestModernReaderProfileSurvivesAllCacheVerificationPaths(t *testing.T) {
	verified, trust := verifyModernSnapshot(t, futureClientSnapshot(41))
	cache := Cache{Path: filepath.Join(t.TempDir(), "modern-cache.json")}
	if err := cache.Store(verified, trust); err != nil {
		t.Fatalf("store future records: %v", err)
	}
	loaded, err := cache.Load(trust)
	if err != nil || !bytes.Equal(loaded.SnapshotBytes, verified.SnapshotBytes) || !bytes.Equal(loaded.EnvelopeBytes, verified.EnvelopeBytes) {
		t.Fatalf("cache failed byte-exact modern round trip: %v", err)
	}
	if _, err := cache.Load(TrustStore{Keys: trust.Keys}); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("cache metadata selected the modern reader for historical trust: %v", err)
	}
	wrongKey, _ := fixtureKey("other-modern-issuer", KeyCurrent, 35)
	if _, err := cache.Load(NewModernTrustStore([]TrustedKey{wrongKey})); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("another modern issuer authenticated cache: %v", err)
	}
	lower, _ := verifyModernSnapshot(t, futureClientSnapshot(40))
	if err := cache.Store(lower, trust); !errors.Is(err, ErrCacheRollback) {
		t.Fatalf("modern cache accepted rollback: %v", err)
	}
	// A caller's parsed view has no authority over the reverified signed bytes.
	lower.Snapshot.Sequence = 999
	for _, check := range []struct {
		name string
		load func(VerifiedBundle, TrustStore) (VerifiedBundle, error)
	}{{"reconcile", cache.Reconcile}, {"observe", cache.Observe}} {
		t.Run(check.name, func(t *testing.T) {
			got, loadErr := check.load(lower, trust)
			if loadErr != nil || got.Snapshot.Sequence != 41 || !bytes.Equal(got.SnapshotBytes, verified.SnapshotBytes) {
				t.Fatalf("modern cache did not retain authoritative signed sequence: %v", loadErr)
			}
		})
	}
	conflicting := futureClientSnapshot(41)
	conflicting.PublicationID += "-equivocation"
	conflict, _ := verifyModernSnapshot(t, conflicting)
	if err := cache.Store(conflict, trust); !errors.Is(err, ErrSequenceConflict) {
		t.Fatalf("modern store accepted equivocation: %v", err)
	}
	if _, err := cache.Reconcile(conflict, trust); !errors.Is(err, ErrSequenceConflict) {
		t.Fatalf("modern reconcile accepted equivocation: %v", err)
	}
	if _, err := cache.Observe(conflict, trust); !errors.Is(err, ErrSequenceConflict) {
		t.Fatalf("modern observe accepted equivocation: %v", err)
	}
}

func TestModernReaderProfileSurvivesBootstrapRemoteAndOfflineLoads(t *testing.T) {
	bootstrap, trust := verifyModernSnapshot(t, futureClientSnapshot(41))
	key, private := fixtureKey("modern-reader-test-only", KeyCurrent, 34)
	body, envelope, pointer := signedSnapshotFixture(t, futureClientSnapshot(42), key, private)
	server, _ := directoryServer(t, pointer, body, envelope, 0)
	defer server.Close()
	client := testClient(server, key, EmbeddedBundle{Snapshot: bootstrap.SnapshotBytes, Envelope: bootstrap.EnvelopeBytes}, filepath.Join(t.TempDir(), "modern-cache.json"))
	client.Trust, client.RequireEmbeddedBootstrap = trust, true
	local, err := client.LoadLocal(0)
	if err != nil || local.Source != BundleSourceEmbedded || local.Snapshot.Sequence != 41 {
		t.Fatalf("modern embedded bootstrap: %v", err)
	}
	if _, err := os.Stat(client.Cache.Path); !os.IsNotExist(err) {
		t.Fatalf("embedded inspection wrote cache: %v", err)
	}
	remote, err := client.Load(context.Background(), 41)
	if err != nil || remote.Source != BundleSourceRemote || remote.Snapshot.Sequence != 42 || !bytes.Equal(remote.SnapshotBytes, body) {
		t.Fatalf("modern remote load: %v", err)
	}
	if _, err := client.Load(context.Background(), 43); !errors.Is(err, ErrRollback) {
		t.Fatalf("modern installed floor ignored: %v", err)
	}
	server.Close()
	offline, err := client.Load(context.Background(), 42)
	if err != nil || offline.Source != BundleSourceCache || !bytes.Equal(offline.SnapshotBytes, body) {
		t.Fatalf("modern offline fallback: %v", err)
	}
	if _, err := client.LoadLocal(43); !errors.Is(err, ErrRollback) {
		t.Fatalf("modern offline installed floor ignored: %v", err)
	}
	client.Now = func() time.Time { return time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC) }
	if _, err := client.Load(context.Background(), 42); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired modern cache authorized installation: %v", err)
	}
	if local, err := client.LoadLocal(42); err != nil || local.Snapshot.Sequence != 42 {
		t.Fatalf("read-only local provenance lost modern records: %v", err)
	}
}
