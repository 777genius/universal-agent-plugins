//go:build ignore

// Test source-backed policy against the exact frozen producer, independent of go test's cwd.
package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func frozenProducerRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("UAP_FROZEN_PRODUCER_ROOT")
	if root == "" {
		t.Fatal("frozen producer root is required")
	}
	if err := requireProducer(root); err != nil {
		t.Fatalf("frozen producer checkout is invalid: %v", err)
	}
	return root
}

func TestRuntimeCacheAuthenticationAndIdentity(t *testing.T) {
	base := filepath.Join(frozenProducerRoot(t), "cli", "cmd", "agentplugins", "directory_bootstrap_inputs")
	// Real signed producer bootstrap bytes, never a test key admitted to
	// production trust. This test must run in exact producer checkout/cli.
	snapshot, err := os.ReadFile(filepath.Join(base, "snapshot.json"))
	if err != nil { t.Fatal(err) }
	envelope, err := os.ReadFile(filepath.Join(base, "envelope.json"))
	if err != nil { t.Fatal(err) }
	trust, err := producerTrust()
	if err != nil { t.Fatal(err) }
	bundle, err := directoryv1.VerifyBundle(snapshot, envelope, trust)
	if err != nil { t.Fatal(err) }
	at, err := time.Parse(time.RFC3339, bundle.Snapshot.GeneratedAt)
	if err != nil { t.Fatal(err) }
	expires, err := time.Parse(time.RFC3339, bundle.Snapshot.ExpiresAt)
	if err != nil { t.Fatal(err) }
	path := filepath.Join(t.TempDir(), "cache.json")
	write := func(s, e []byte) {
		t.Helper()
		body, marshalErr := json.Marshal(map[string]any{"schema_version": 1, "sequence": bundle.Snapshot.Sequence,
			"snapshot": base64.StdEncoding.EncodeToString(s), "envelope": base64.StdEncoding.EncodeToString(e)})
		if marshalErr != nil { t.Fatal(marshalErr) }
		if writeErr := os.WriteFile(path, body, 0600); writeErr != nil { t.Fatal(writeErr) }
	}
	write(snapshot, envelope)
	if _, err := authenticateCache(path, trust, bundle.Snapshot.Sequence, bundle.Digest, at); err != nil { t.Fatal(err) }
	for _, test := range []struct{name string; seq uint64; digest string; at time.Time}{
		{"wrong-sequence", bundle.Snapshot.Sequence + 1, bundle.Digest, at},
		{"wrong-digest", bundle.Snapshot.Sequence, "sha256:wrong", at},
		{"not-yet-valid", bundle.Snapshot.Sequence, bundle.Digest, at.Add(-time.Second)},
		{"expired", bundle.Snapshot.Sequence, bundle.Digest, expires},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := authenticateCache(path, trust, test.seq, test.digest, test.at); err == nil { t.Fatal("unverified cache accepted") }
		})
	}
	write(append(append([]byte{}, snapshot...), ' '), envelope)
	if _, err := authenticateCache(path, trust, bundle.Snapshot.Sequence, bundle.Digest, at); err == nil { t.Fatal("changed signed bytes accepted") }
	var wire map[string]any
	if err := json.Unmarshal(envelope, &wire); err != nil { t.Fatal(err) }
	wire["signature"] = base64.StdEncoding.EncodeToString(make([]byte, 64))
	badEnvelope, err := json.Marshal(wire)
	if err != nil { t.Fatal(err) }
	write(snapshot, badEnvelope)
	if _, err := authenticateCache(path, trust, bundle.Snapshot.Sequence, bundle.Digest, at); err == nil { t.Fatal("forged signature accepted") }
	write(snapshot, envelope)
	if _, err := authenticateCache(path, directoryv1.TrustStore{}, bundle.Snapshot.Sequence, bundle.Digest, at); err == nil { t.Fatal("untrusted key accepted") }
}

func TestProducerPolicyNeverPromotesV1GrantToSignedProjectGrant(t *testing.T) {
	// Wrong schema and unsupported Project backends must fail through the
	// production function, rather than through an audit-side allowlist.
	request := domain.DirectoryResolveRequest{Scope: domain.ScopeProject, Targets: []domain.ClientID{"hermes"}}
	for _, schema := range []int{1, 2} {
		if _, err := domain.ResolveDirectoryV1ProjectSource(domain.DirectorySnapshot{SnapshotSchemaVersion: schema}, request); err == nil {
			t.Fatal("unsupported project tuple accepted")
		}
	}
}

func TestProducerBootstrapFloorAndEquivocation(t *testing.T) {
	trust, err := producerTrust()
	if err != nil { t.Fatal(err) }
	root := frozenProducerRoot(t)
	embedded, err := producerBootstrap(root, trust)
	if err != nil { t.Fatal(err) }
	if err := checkFloor(embedded, embedded); err != nil { t.Fatal(err) }
	below := embedded
	below.Snapshot.Sequence--
	if err := checkFloor(below, embedded); err == nil { t.Fatal("producer bootstrap rollback accepted") }
	conflict := embedded
	conflict.Digest = "sha256:different"
	if err := checkFloor(conflict, embedded); err == nil { t.Fatal("producer bootstrap equivocation accepted") }
}
