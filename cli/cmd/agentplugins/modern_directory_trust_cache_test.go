package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
)

// An independent modern-client v1 feed must not authenticate with the legacy
// feed's Ed25519 public key, nor reuse its local high-water cache. Same schema,
// release sequence, key ID and even exact snapshot body cannot supply trust.
func TestDistinctSignedV1FeedsCannotBorrowEachOthersTrustOrCache(t *testing.T) {
	clearDirectoryEnvironment(t)
	root := t.TempDir()
	legacyRoot, modernRoot := filepath.Join(root, "legacy"), filepath.Join(root, "modern")
	for _, path := range []string{legacyRoot, modernRoot} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	legacyFixture := writeConformanceFixtureWithSeed(t, legacyRoot, 17)
	modernFixture := writeConformanceFixtureWithSeed(t, modernRoot, 39)
	legacy, err := newConformanceDirectoryClient(legacyFixture)
	if err != nil {
		t.Fatal(err)
	}
	modern, err := newConformanceDirectoryClient(modernFixture)
	if err != nil {
		t.Fatal(err)
	}
	modern.FeedID = "pi-modern-v1-test-only"
	modern.Origin = "https://modern-conformance.invalid/registry/schemas/1/"
	if legacy.SignedFeedID() != "" || modern.SignedFeedID() == "" ||
		legacy.Origin == modern.Origin || legacy.Cache.Path == modern.Cache.Path {
		t.Fatal("signed v1 lineages share an origin, cache or feed identity")
	}
	if len(legacy.Trust.Keys) != 1 || len(modern.Trust.Keys) != 1 ||
		bytes.Equal(legacy.Trust.Keys[0].PublicKey, modern.Trust.Keys[0].PublicKey) {
		t.Fatal("independent test-only signed feeds must use distinct Ed25519 keys")
	}
	if _, err := legacy.Embedded.Verify(legacy.Trust); err != nil {
		t.Fatalf("legacy bootstrap failed its own trust: %v", err)
	}
	if _, err := modern.Embedded.Verify(modern.Trust); err != nil {
		t.Fatalf("modern bootstrap failed its own trust: %v", err)
	}
	for _, test := range []struct {
		name   string
		client *directoryv1.Client
		trust  directoryv1.TrustStore
	}{
		{"modern_accepts_legacy_key", modern, legacy.Trust},
		{"legacy_accepts_modern_key", legacy, modern.Trust},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.client.Embedded.Verify(test.trust); err == nil {
				t.Fatal("cross-feed Ed25519 bootstrap was accepted")
			}
		})
	}
	clock := func() time.Time { return time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC) }
	legacy.Now, modern.Now = clock, clock
	first, err := legacy.Load(context.Background(), 0)
	if err != nil {
		t.Fatalf("legacy signed remote load: %v", err)
	}
	if first.Snapshot.Sequence != 41 || first.Source != directoryv1.BundleSourceRemote {
		t.Fatalf("legacy signed load identity: %+v", first)
	}
	oldCache, err := os.ReadFile(legacy.Cache.Path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := modern.Load(context.Background(), 0)
	if err != nil {
		t.Fatalf("modern signed remote load: %v", err)
	}
	if second.Snapshot.Sequence != 41 || second.Source != directoryv1.BundleSourceRemote {
		t.Fatalf("modern signed load identity: %+v", second)
	}
	oldAfter, err := os.ReadFile(legacy.Cache.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(oldCache, oldAfter) {
		t.Fatal("loading the independent modern feed modified the legacy cache")
	}
	if _, err := legacy.Cache.Load(legacy.Trust); err != nil {
		t.Fatalf("legacy authenticated cache invalid: %v", err)
	}
	if _, err := modern.Cache.Load(modern.Trust); err != nil {
		t.Fatalf("modern authenticated cache invalid: %v", err)
	}
	if _, err := legacy.Cache.Load(modern.Trust); err == nil {
		t.Fatal("modern trust authenticated the legacy cache")
	}
	if _, err := modern.Cache.Load(legacy.Trust); err == nil {
		t.Fatal("legacy trust authenticated the modern cache")
	}
	if err := os.WriteFile(modern.Cache.Path, oldCache, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := modern.Cache.Load(modern.Trust); err == nil {
		t.Fatal("legacy cache bytes were accepted as modern signed cache")
	}
	if _, err := legacy.Cache.Load(legacy.Trust); err != nil {
		t.Fatalf("cross-feed negative test changed legacy cache: %v", err)
	}
}
