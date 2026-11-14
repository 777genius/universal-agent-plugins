package discoveryv1

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fixtureArtifacts struct {
	pointer  []byte
	snapshot []byte
	envelope []byte
	search   []byte
	trust    TrustStore
}

func stringPointer(value string) *string { return &value }

func discoveryFixture(t *testing.T, sequence uint64, generated time.Time) fixtureArtifacts {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	stem := strings.Repeat("0", 20-len(string(rune('0'+sequence)))) + string(rune('0'+sequence))
	// Tests use single-digit sequences. Production validation independently
	// requires the exact twenty-digit sequence framing.
	if sequence > 9 {
		t.Fatal("fixture sequence must be one digit")
	}
	record := Record{
		Slug: "discovery:owner/repo//plugins/demo", Name: "demo", Description: "fixture", Owner: "owner",
		Repository: "owner/repo", PackagePath: "plugins/demo", Revision: strings.Repeat("a", 40),
		Version: stringPointer("1.0.0"), License: stringPointer("Apache-2.0"), SchemaVersion: "1.0.0",
		Components: Components{MCP: 1}, MCPTransports: []string{"streamable-http"},
		CompatibleClients: []string{"codex", "cursor", "copilot", "vscode", "kiro"},
		Authentication:    "unknown", Status: "conformant_unreviewed", RuntimeReviewed: false,
		TreeDigest: "sha256:" + strings.Repeat("1", 64), ManifestDigest: "sha256:" + strings.Repeat("2", 64),
		Stars: 42, RepositoryUpdatedAt: generated.Format(time.RFC3339), Availability: "available",
		Author: &Author{Name: "Fixture"}, FirstSeen: generated.Format(time.RFC3339), LastSeen: generated.Format(time.RFC3339),
	}
	compact := record
	compact.Author, compact.FirstSeen, compact.LastSeen = nil, "", ""
	search := Search{SearchSchemaVersion: 1, Sequence: sequence, GeneratedAt: generated.Format(time.RFC3339), Records: []Record{compact}}
	searchBytes, err := json.Marshal(search)
	if err != nil {
		t.Fatal(err)
	}
	searchDigest := sha256.Sum256(searchBytes)
	snapshot := Snapshot{
		DiscoverySchemaVersion: 1, Sequence: sequence, PublicationID: "fixture", SourceCommit: strings.Repeat("b", 40),
		GeneratedAt: generated.Format(time.RFC3339), ExpiresAt: generated.Add(72 * time.Hour).Format(time.RFC3339), Complete: true,
		QueryManifestDigest: "sha256:" + strings.Repeat("3", 64), Partitions: []Partition{},
		SearchProjection: SearchProjection{Path: "search/" + stem + ".json", Digest: "sha256:" + hex.EncodeToString(searchDigest[:]), RecordCount: 1},
		Records:          []Record{record},
	}
	snapshotBytes, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshotDigest := sha256.Sum256(snapshotBytes)
	envelope := Envelope{
		EnvelopeSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: sequence, KeyID: "fixture",
		Algorithm: "Ed25519", SignatureDomain: SignatureDomain, SnapshotDigest: "sha256:" + hex.EncodeToString(snapshotDigest[:]),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(private, SignatureMessage(snapshotBytes))),
	}
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	pointer := Pointer{
		PointerSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: sequence,
		SnapshotPath: "snapshots/" + stem + ".json", EnvelopePath: "snapshots/" + stem + ".envelope.json", SearchPath: "search/" + stem + ".json",
		FetchContract: FetchContract{MaxRedirects: 0, LatestMaxBytes: MaxLatestBytes, SnapshotMaxBytes: MaxSnapshotBytes,
			EnvelopeMaxBytes: MaxEnvelopeBytes, SearchMaxBytes: MaxSearchBytes, RetryAttempts: 1},
	}
	pointerBytes, err := json.Marshal(pointer)
	if err != nil {
		t.Fatal(err)
	}
	return fixtureArtifacts{pointer: pointerBytes, snapshot: snapshotBytes, envelope: envelopeBytes, search: searchBytes,
		trust: TrustStore{Keys: []TrustedKey{{ID: "fixture", PublicKey: public, State: KeyCurrent}}}}
}

func TestVerifyBundleRejectsTamperingAndWrongDomain(t *testing.T) {
	t.Parallel()
	fixture := discoveryFixture(t, 1, time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC))
	if _, err := VerifyBundle(fixture.pointer, fixture.snapshot, fixture.envelope, fixture.search, fixture.trust); err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), fixture.search...)
	tampered[len(tampered)-2] ^= 1
	if _, err := VerifyBundle(fixture.pointer, fixture.snapshot, fixture.envelope, tampered, fixture.trust); err == nil {
		t.Fatal("tampered search projection was accepted")
	}
	var envelope Envelope
	if err := json.Unmarshal(fixture.envelope, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.SignatureDomain = "UAP-DIRECTORY-SNAPSHOT-ED25519-V1"
	wrongDomain, _ := json.Marshal(envelope)
	if _, err := VerifyBundle(fixture.pointer, fixture.snapshot, wrongDomain, fixture.search, fixture.trust); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("wrong signature domain error = %v", err)
	}
}

func TestDiscoveryTrustRotationAndEverySignedArtifactFailClosed(t *testing.T) {
	t.Parallel()
	fixture := discoveryFixture(t, 1, time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC))
	trusted := fixture.trust.Keys[0]

	next := TrustStore{Keys: append([]TrustedKey(nil), fixture.trust.Keys...)}
	next.Keys[0].State = KeyNext
	if _, err := VerifyBundle(fixture.pointer, fixture.snapshot, fixture.envelope, fixture.search, next); err != nil {
		t.Fatalf("next rotation key was not accepted: %v", err)
	}
	retired := TrustStore{Keys: append([]TrustedKey(nil), fixture.trust.Keys...)}
	retired.Keys[0].State = KeyRetired
	if _, err := VerifyBundle(fixture.pointer, fixture.snapshot, fixture.envelope, fixture.search, retired); !errors.Is(err, ErrRetiredKey) {
		t.Fatalf("retired key error = %v", err)
	}
	unknown := TrustStore{Keys: []TrustedKey{{ID: "other", PublicKey: trusted.PublicKey, State: KeyCurrent}}}
	if _, err := VerifyBundle(fixture.pointer, fixture.snapshot, fixture.envelope, fixture.search, unknown); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("unknown key error = %v", err)
	}

	mutate := func(body []byte) []byte {
		result := append([]byte(nil), body...)
		result[len(result)/2] ^= 1
		return result
	}
	for name, artifacts := range map[string]struct {
		pointer, snapshot, envelope, search []byte
	}{
		"pointer":  {mutate(fixture.pointer), fixture.snapshot, fixture.envelope, fixture.search},
		"snapshot": {fixture.pointer, mutate(fixture.snapshot), fixture.envelope, fixture.search},
		"envelope": {fixture.pointer, fixture.snapshot, mutate(fixture.envelope), fixture.search},
		"search":   {fixture.pointer, fixture.snapshot, fixture.envelope, mutate(fixture.search)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := VerifyBundle(artifacts.pointer, artifacts.snapshot, artifacts.envelope, artifacts.search, fixture.trust); err == nil {
				t.Fatalf("tampered %s was accepted", name)
			}
		})
	}
}

func TestUnreviewedDiscoveryRejectsUnprovedChatGPTCompatibility(t *testing.T) {
	generated := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	record := Record{
		Slug: "discovery:owner/repo", Name: "demo", Owner: "owner", Repository: "owner/repo",
		Revision: strings.Repeat("a", 40), SchemaVersion: "1.0.0", Components: Components{MCP: 1},
		MCPTransports: []string{"streamable-http"}, CompatibleClients: []string{"codex", "chatgpt"}, Authentication: "unknown",
		Status: "conformant_unreviewed", TreeDigest: "sha256:" + strings.Repeat("1", 64),
		ManifestDigest: "sha256:" + strings.Repeat("2", 64), RepositoryUpdatedAt: generated.Format(time.RFC3339),
		Availability: "available", FirstSeen: generated.Format(time.RFC3339), LastSeen: generated.Format(time.RFC3339),
	}
	if err := validateRecords([]Record{record}, generated, true); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("unproved ChatGPT compatibility error = %v", err)
	}
}

func TestClientColdCachedOfflineTamperedExpiredAndRollback(t *testing.T) {
	now := time.Date(2026, 8, 27, 1, 0, 0, 0, time.UTC)
	first := discoveryFixture(t, 1, now.Add(-time.Hour))
	current := first
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body []byte
		switch request.URL.Path {
		case "/latest.json":
			body = current.pointer
		default:
			pointer, _ := ParsePointer(current.pointer)
			switch request.URL.Path {
			case "/" + pointer.SnapshotPath:
				body = current.snapshot
			case "/" + pointer.EnvelopePath:
				body = current.envelope
			case "/" + pointer.SearchPath:
				body = current.search
			default:
				http.NotFound(writer, request)
				return
			}
		}
		_, _ = writer.Write(body)
	}))
	cachePath := filepath.Join(t.TempDir(), "discovery.json")
	client := Client{Origin: server.URL + "/", AllowHTTPForTests: true, Trust: first.trust, Cache: Cache{Path: cachePath}, Now: func() time.Time { return now }}
	cold, err := client.Load(context.Background(), 0)
	if err != nil || cold.Source != BundleSourceRemote || cold.Snapshot.Sequence != 1 {
		t.Fatalf("cold load = %+v, %v", cold, err)
	}
	server.Close()
	offline, err := client.Load(context.Background(), 0)
	if err != nil || offline.Snapshot.Sequence != 1 {
		t.Fatalf("offline load = %+v, %v", offline, err)
	}

	second := discoveryFixture(t, 2, now.Add(-30*time.Minute))
	secondBundle, err := VerifyBundle(second.pointer, second.snapshot, second.envelope, second.search, second.trust)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Cache.Store(secondBundle, second.trust); err != nil {
		t.Fatal(err)
	}
	if _, err := client.LoadLocal(3); !errors.Is(err, ErrRollback) {
		t.Fatalf("cache floor error = %v", err)
	}
	rollbackServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		pointer, _ := ParsePointer(first.pointer)
		body := first.pointer
		switch request.URL.Path {
		case "/" + pointer.SnapshotPath:
			body = first.snapshot
		case "/" + pointer.EnvelopePath:
			body = first.envelope
		case "/" + pointer.SearchPath:
			body = first.search
		}
		_, _ = writer.Write(body)
	}))
	client.Origin = rollbackServer.URL + "/"
	rolledBack, err := client.Load(context.Background(), 0)
	rollbackServer.Close()
	if err != nil || rolledBack.Snapshot.Sequence != 2 {
		t.Fatalf("rollback did not retain LKG: sequence=%d err=%v", rolledBack.Snapshot.Sequence, err)
	}

	expired := discoveryFixture(t, 3, now.Add(-96*time.Hour))
	expiredServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		pointer, _ := ParsePointer(expired.pointer)
		body := expired.pointer
		switch request.URL.Path {
		case "/" + pointer.SnapshotPath:
			body = expired.snapshot
		case "/" + pointer.EnvelopePath:
			body = expired.envelope
		case "/" + pointer.SearchPath:
			body = expired.search
		}
		_, _ = writer.Write(body)
	}))
	defer expiredServer.Close()
	expiredClient := Client{Origin: expiredServer.URL + "/", AllowHTTPForTests: true, Trust: expired.trust,
		Cache: Cache{Path: filepath.Join(t.TempDir(), "expired.json")}, Now: func() time.Time { return now }}
	if _, err := expiredClient.Load(context.Background(), 0); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired error = %v", err)
	}
}

func TestInterruptedCacheReplacementRetainsLKG(t *testing.T) {
	t.Parallel()
	generated := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	first := discoveryFixture(t, 1, generated)
	path := filepath.Join(t.TempDir(), "cache.json")
	cache := Cache{Path: path}
	bundle, err := VerifyBundle(first.pointer, first.snapshot, first.envelope, first.search, first.trust)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Store(bundle, first.trust); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cache.BeforeRename = func(string) error { return errors.New("injected") }
	second := discoveryFixture(t, 2, generated.Add(time.Hour))
	secondBundle, err := VerifyBundle(second.pointer, second.snapshot, second.envelope, second.search, second.trust)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Store(secondBundle, second.trust); err == nil || err.Error() != "injected" {
		t.Fatalf("interrupted replacement error = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(original) {
		t.Fatalf("cache changed after interrupted reconciliation: %v", err)
	}
}
