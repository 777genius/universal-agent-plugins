package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func modernPinnedTrustWithFutureTarget(t *testing.T) pinnedModernDirectoryTrust {
	t.Helper()
	approved := modernPinnedTestTrust(t)
	snapshot, err := directoryv1.ParseSnapshot(approved.Bootstrap.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	policy := &snapshot.Distributions[0].ReleasePolicies[0]
	policy.Targets = append(policy.Targets, domain.DirectoryTarget{
		Client: "future-test-agent", Scopes: []domain.InstallScope{domain.ScopeUser},
		Delivery: "managed", Authentication: domain.AuthenticationRequirementUnknown,
	})
	body, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	body = append(body, '\n')
	envelope, err := directoryv1.ParseEnvelope(approved.Bootstrap.Envelope)
	if err != nil {
		t.Fatal(err)
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{39}, ed25519.SeedSize))
	digest := sha256.Sum256(body)
	envelope.SnapshotDigest = "sha256:" + hex.EncodeToString(digest[:])
	envelope.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, directoryv1.SnapshotSignatureMessage(body)))
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	approved.Bootstrap = directoryv1.EmbeddedBundle{Snapshot: body, Envelope: envelopeBytes}
	return approved
}

func TestPinnedModernDirectoryRetainsFutureIDsThroughBootstrapRemoteAndCache(t *testing.T) {
	clearDirectoryEnvironment(t)
	approved := modernPinnedTrustWithFutureTarget(t)
	if _, err := directoryv1.ParseSnapshot(approved.Bootstrap.Snapshot); !errors.Is(err, directoryv1.ErrStrictJSON) {
		t.Fatalf("historical parser accepted future target: %v", err)
	}
	modern, err := newPinnedModernDirectoryClient(t.TempDir(), approved)
	if err != nil {
		t.Fatalf("pinned modern constructor failed future-ID bootstrap: %v", err)
	}
	local, err := modern.LoadLocal(0)
	if err != nil || local.Source != directoryv1.BundleSourceEmbedded {
		t.Fatalf("modern bootstrap lost its reader contract: %v", err)
	}
	modern.HTTPClient.Transport = conformanceFixtureTransport{
		sequence: 41, snapshot: approved.Bootstrap.Snapshot, envelope: approved.Bootstrap.Envelope,
	}
	modern.Now = func() time.Time { return time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC) }
	remote, err := modern.Load(context.Background(), 41)
	if err != nil || remote.Source != directoryv1.BundleSourceRemote || !bytes.Equal(remote.SnapshotBytes, approved.Bootstrap.Snapshot) {
		t.Fatalf("modern remote load changed reader contract or bytes: %v", err)
	}
	cached, err := modern.Cache.Load(modern.Trust)
	if err != nil || !bytes.Equal(cached.SnapshotBytes, approved.Bootstrap.Snapshot) {
		t.Fatalf("modern cache lost its reader contract: %v", err)
	}
	if _, err := modern.Cache.Load(directoryv1.TrustStore{Keys: modern.Trust.Keys}); !errors.Is(err, directoryv1.ErrStrictJSON) {
		t.Fatalf("cache selected modern decoding without pinned constructor: %v", err)
	}
}
