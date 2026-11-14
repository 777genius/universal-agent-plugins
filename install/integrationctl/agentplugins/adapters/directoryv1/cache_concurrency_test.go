package directoryv1

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func blockedDirectoryServer(t *testing.T, pointer Pointer, snapshot, envelope []byte, reached chan<- struct{}, release <-chan struct{}) *httptest.Server {
	t.Helper()
	pointerBytes, err := json.Marshal(pointer)
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/registry/schemas/1/latest.json":
			_, _ = writer.Write(pointerBytes)
		case "/registry/schemas/1/" + pointer.SnapshotPath:
			_, _ = writer.Write(snapshot)
		case "/registry/schemas/1/" + pointer.EnvelopePath:
			reached <- struct{}{}
			<-release
			_, _ = writer.Write(envelope)
		default:
			http.NotFound(writer, request)
		}
	}))
}

func TestConcurrentLoadReconcilesTenToTwelveAgainstRemoteEleven(t *testing.T) {
	key, private := fixtureKey("race-key", KeyCurrent, 31)
	trust := TrustStore{Keys: []TrustedKey{key}}
	cache := Cache{Path: filepath.Join(t.TempDir(), "lkg.json")}
	storeSignedSequence(t, cache, trust, key, private, 10, "")
	remoteSnapshot, remoteEnvelope, pointer := signedFixture(t, 11, key, private)
	reached, release := make(chan struct{}, 1), make(chan struct{})
	server := blockedDirectoryServer(t, pointer, remoteSnapshot, remoteEnvelope, reached, release)
	defer server.Close()
	client := testClient(server, key, EmbeddedBundle{}, cache.Path)
	type result struct {
		bundle VerifiedBundle
		err    error
	}
	done := make(chan result, 1)
	go func() {
		bundle, err := client.Load(context.Background(), 0)
		done <- result{bundle: bundle, err: err}
	}()
	<-reached
	storeSignedSequence(t, cache, trust, key, private, 12, "")
	close(release)
	loaded := <-done
	if loaded.err != nil || loaded.bundle.Snapshot.Sequence != 12 || loaded.bundle.Source != BundleSourceCache {
		t.Fatalf("concurrent load returned sequence/source %d/%s: %v", loaded.bundle.Snapshot.Sequence, loaded.bundle.Source, loaded.err)
	}
}

func TestConcurrentLoadPropagatesEqualSequenceEquivocation(t *testing.T) {
	key, private := fixtureKey("race-key", KeyCurrent, 32)
	trust := TrustStore{Keys: []TrustedKey{key}}
	cache := Cache{Path: filepath.Join(t.TempDir(), "lkg.json")}
	storeSignedSequence(t, cache, trust, key, private, 10, "")
	remoteSnapshot, remoteEnvelope, pointer := signedFixture(t, 11, key, private)
	reached, release := make(chan struct{}, 1), make(chan struct{})
	server := blockedDirectoryServer(t, pointer, remoteSnapshot, remoteEnvelope, reached, release)
	defer server.Close()
	client := testClient(server, key, EmbeddedBundle{}, cache.Path)
	done := make(chan error, 1)
	go func() {
		_, err := client.Load(context.Background(), 0)
		done <- err
	}()
	<-reached
	storeSignedSequence(t, cache, trust, key, private, 11, "equivocating publication")
	close(release)
	if err := <-done; !errors.Is(err, ErrSequenceConflict) {
		t.Fatalf("equal-sequence equivocation was hidden: %v", err)
	}
}

func TestGeneratedBootstrapBindsReleaseAndEnforcesFreshInstallFloor(t *testing.T) {
	key, private := fixtureKey("bootstrap-key", KeyCurrent, 33)
	trust := TrustStore{Keys: []TrustedKey{key}}
	snapshot, envelope, _ := signedFixture(t, 12, key, private)
	encoded, err := EncodeReleaseBootstrap(snapshot, envelope, trust)
	if err != nil {
		t.Fatal(err)
	}
	again, err := EncodeReleaseBootstrap(snapshot, envelope, trust)
	if err != nil || again != encoded {
		t.Fatalf("bootstrap generation was not deterministic: %+v %+v %v", encoded, again, err)
	}
	generated, err := GenerateReleaseBootstrapSource("main", "generatedProductionDirectoryBootstrap", snapshot, envelope, trust)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := GenerateReleaseBootstrapSource("main", "generatedProductionDirectoryBootstrap", snapshot, envelope, trust)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("bootstrap source generation was not deterministic: %v", err)
	}
	if !strings.Contains(string(generated), "Sequence:       12,") || !strings.Contains(string(generated), encoded.SnapshotBase64) || !strings.Contains(string(generated), encoded.EnvelopeBase64) {
		t.Fatal("generated bootstrap source did not bind the verified release bytes")
	}
	mismatched := encoded
	mismatched.Sequence++
	if _, _, err := DecodeReleaseBootstrap(mismatched, trust); err == nil {
		t.Fatal("generated bootstrap accepted a declared sequence different from its signed release")
	}
	embedded, ready, err := DecodeReleaseBootstrap(encoded, trust)
	if err != nil || !ready {
		t.Fatalf("generated signed bootstrap readiness = %v, %v", ready, err)
	}
	remoteSnapshot, remoteEnvelope, pointer := signedFixture(t, 11, key, private)
	server, _ := directoryServer(t, pointer, remoteSnapshot, remoteEnvelope, 0)
	defer server.Close()
	client := testClient(server, key, embedded, filepath.Join(t.TempDir(), "cache.json"))
	bundle, err := client.Load(context.Background(), 0)
	if err != nil || bundle.Snapshot.Sequence != 12 || bundle.Source != BundleSourceEmbedded {
		t.Fatalf("fresh install escaped generated bootstrap floor: sequence=%d source=%s err=%v", bundle.Snapshot.Sequence, bundle.Source, err)
	}
}

func TestEmptyGeneratedBootstrapCannotSignalReleaseReadiness(t *testing.T) {
	embedded, ready, err := DecodeReleaseBootstrap(ReleaseBootstrapEncoding{}, TrustStore{})
	if err != nil || ready || len(embedded.Snapshot) != 0 || len(embedded.Envelope) != 0 {
		t.Fatalf("empty bootstrap = ready:%v embedded:%+v err:%v", ready, embedded, err)
	}
	if _, _, err := DecodeReleaseBootstrap(ReleaseBootstrapEncoding{Sequence: 1}, TrustStore{}); err == nil {
		t.Fatal("partial generated bootstrap was accepted")
	}
	if _, err := GenerateReleaseBootstrapSource("main", "generatedProductionDirectoryBootstrap", nil, nil, TrustStore{}); err == nil {
		t.Fatal("release generator accepted an empty unsigned bundle")
	}
}

func storeSignedSequence(t *testing.T, cache Cache, trust TrustStore, key TrustedKey, privateKey ed25519.PrivateKey, sequence uint64, publicationSuffix string) {
	t.Helper()
	snapshot := fixtureSnapshot(sequence)
	if publicationSuffix != "" {
		snapshot.Products[0].Description += ": " + publicationSuffix
	}
	snapshotBytes, envelopeBytes, _ := signedSnapshotFixture(t, snapshot, key, privateKey)
	bundle, err := VerifyBundle(snapshotBytes, envelopeBytes, trust)
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Store(bundle, trust); err != nil {
		t.Fatal(err)
	}
}
