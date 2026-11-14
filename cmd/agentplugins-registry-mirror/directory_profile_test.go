package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
)

const modernFixtureFeed = "mirror-modern-test-only"

func modernFixtureAuthority() (modernMirrorAuthority, ed25519.PrivateKey) {
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{72}, ed25519.SeedSize))
	return modernMirrorAuthority{
		FeedID: modernFixtureFeed, Origin: "https://modern-fixture.invalid/registry/modern/schemas/1/",
		KeyID: "mirror-modern-test-only", PublicKeyBase64: base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey)),
	}, private
}

func modernFixtureBytes(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/modern-snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func fixtureJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return append(body, '\n')
}

type mirrorFixtureTransport map[string][]byte

func (transport mirrorFixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body, ok := transport[request.URL.String()]
	if !ok || request.Method != http.MethodGet {
		return nil, fmt.Errorf("unexpected mirror fixture request: %s %s", request.Method, request.URL)
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}, nil
}

func signedMirrorFixture(t *testing.T, body []byte, profile directoryMirrorProfile, keyID string, private ed25519.PrivateKey) (*http.Client, []byte, []byte) {
	t.Helper()
	var identity struct {
		Sequence uint64 `json:"sequence"`
	}
	if err := json.Unmarshal(body, &identity); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	envelope := fixtureJSON(t, directoryv1.Envelope{
		EnvelopeSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: identity.Sequence,
		KeyID: keyID, Algorithm: "Ed25519", SignatureDomain: directoryv1.SignatureDomain,
		SnapshotDigest: fmt.Sprintf("sha256:%x", sum),
		Signature:      base64.StdEncoding.EncodeToString(ed25519.Sign(private, directoryv1.SnapshotSignatureMessage(body))),
	})
	pointer := directoryv1.Pointer{
		PointerSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: identity.Sequence,
		SnapshotPath: fmt.Sprintf("snapshots/%020d.json", identity.Sequence), EnvelopePath: fmt.Sprintf("snapshots/%020d.envelope.json", identity.Sequence),
		FetchContract: directoryv1.FetchContract{HTTPSRequired: true, SameOriginRedirectsOnly: true, MaxRedirects: 2,
			LatestMaxBytes: directoryv1.MaxLatestBytes, SnapshotMaxBytes: directoryv1.MaxSnapshotBytes, EnvelopeMaxBytes: directoryv1.MaxEnvelopeBytes, RetryAttempts: 3},
	}
	pointerBytes := fixtureJSON(t, pointer)
	transport := mirrorFixtureTransport{}
	for path, value := range map[string][]byte{"latest.json": pointerBytes, pointer.SnapshotPath: body, pointer.EnvelopePath: envelope} {
		transport[profile.origin.ResolveReference(&url.URL{Path: path}).String()] = value
	}
	return &http.Client{Transport: transport}, pointerBytes, envelope
}

func TestModernDirectoryMirrorAuthenticatesOriginalBytesAndLegacyStaysStrict(t *testing.T) {
	approved, private := modernFixtureAuthority()
	profile, err := newModernDirectoryProfile(approved)
	if err != nil {
		t.Fatal(err)
	}
	body := modernFixtureBytes(t)
	client, pointer, envelope := signedMirrorFixture(t, body, profile, approved.KeyID, private)
	result, err := fetchDirectoryWithProfile(client, defaultRegistry, profile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result.snapshotBytes, body) || !bytes.Equal(result.pointerBytes, pointer) || !bytes.Equal(result.envelopeBytes, envelope) {
		t.Fatal("mirror changed original authenticated bytes")
	}
	if result.feedID != modernFixtureFeed || result.bundle.Snapshot.Sequence != 1 {
		t.Fatal("modern reader lost its independently selected lineage")
	}
	legacy := directoryMirrorProfile{origin: profile.origin}
	if _, err := fetchDirectoryWithProfile(client, defaultRegistry, legacy); !errors.Is(err, directoryv1.ErrStrictJSON) {
		t.Fatalf("metadata/signing key selected modern decoding for historical mirror: %v", err)
	}
	wrongKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{73}, ed25519.SeedSize))
	badClient, _, _ := signedMirrorFixture(t, body, profile, approved.KeyID, wrongKey)
	if _, err := fetchDirectoryWithProfile(badClient, defaultRegistry, profile); !errors.Is(err, directoryv1.ErrInvalidSignature) {
		t.Fatalf("mirror accepted wrong signer: %v", err)
	}
}

func TestModernDirectoryMirrorValidatesUnknownRecordsBeforeFiltering(t *testing.T) {
	approved, private := modernFixtureAuthority()
	profile, err := newModernDirectoryProfile(approved)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		edit func(map[string]any, map[string]any, map[string]any)
	}{
		{"unsafe id", func(_ map[string]any, target, _ map[string]any) { target["client"] = "../future" }},
		{"duplicate target", func(policy, target, _ map[string]any) { policy["targets"] = append(policy["targets"].([]any), target) }},
		{"delivery", func(_ map[string]any, target, _ map[string]any) { target["delivery"] = "new-protocol" }},
		{"authentication", func(_ map[string]any, target, _ map[string]any) { target["authentication"] = "automatic" }},
		{"scope", func(_ map[string]any, target, _ map[string]any) { target["scopes"] = []string{"project"} }},
		{"app binding", func(_ map[string]any, target, _ map[string]any) {
			target["app_binding"] = map[string]string{"app_key": "tool", "id": "app_tool", "mcp_server": "tool"}
		}},
		{"unknown field", func(_ map[string]any, target, _ map[string]any) { target["permissions"] = "all" }},
		{"unknown evidence id", func(_, _ map[string]any, evidence map[string]any) { evidence["client"] = "../future" }},
		{"modern sequence one trust", func(_, _ map[string]any, evidence map[string]any) { delete(evidence, "trust") }},
		{"wrong evidence scope", func(_, _ map[string]any, evidence map[string]any) { evidence["scope"] = "project" }},
		{"unknown evidence field", func(_, _ map[string]any, evidence map[string]any) { evidence["unexpected"] = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var snapshot map[string]any
			if err := json.Unmarshal(modernFixtureBytes(t), &snapshot); err != nil {
				t.Fatal(err)
			}
			policy := snapshot["distributions"].([]any)[0].(map[string]any)["release_policies"].([]any)[0].(map[string]any)
			tc.edit(policy, policy["targets"].([]any)[2].(map[string]any), snapshot["evidence"].([]any)[3].(map[string]any))
			client, _, _ := signedMirrorFixture(t, fixtureJSON(t, snapshot), profile, approved.KeyID, private)
			if _, err := fetchDirectoryWithProfile(client, defaultRegistry, profile); !errors.Is(err, directoryv1.ErrStrictJSON) {
				t.Fatalf("signed malformed future record reached projection: %v", err)
			}
		})
	}
}

func TestModernDirectoryProfileRequiresIndependentAuthority(t *testing.T) {
	cases := []struct {
		name string
		edit func(*modernMirrorAuthority)
	}{
		{"missing identity", func(a *modernMirrorAuthority) { a.FeedID = "" }},
		{"unsafe identity", func(a *modernMirrorAuthority) { a.FeedID = "../legacy" }},
		{"historical key id", func(a *modernMirrorAuthority) { a.KeyID = directoryKeyID }},
		{"historical key bytes", func(a *modernMirrorAuthority) { a.PublicKeyBase64 = directoryPublicKey }},
		{"malformed key", func(a *modernMirrorAuthority) { a.PublicKeyBase64 = "invalid" }},
		{"historical origin", func(a *modernMirrorAuthority) { a.Origin = defaultPagesOrigin + "registry/modern/schemas/1/" }},
		{"origin spelling", func(a *modernMirrorAuthority) { a.Origin = "https://777GENIUS.github.io.:00443/other/schemas/1/" }},
		{"unsafe origin", func(a *modernMirrorAuthority) { a.Origin = "http://modern.invalid/schemas/1/" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			approved, _ := modernFixtureAuthority()
			tc.edit(&approved)
			if _, err := newModernDirectoryProfile(approved); err == nil {
				t.Fatal("unreviewed or historical authority accepted")
			}
		})
	}
}

func TestModernMirrorCannotBorrowAnotherLineageFloor(t *testing.T) {
	previous := mirrorMetadata{SchemaVersion: 1, RegistryRepository: defaultRegistry, DirectorySequence: 47, DirectoryDigest: "old"}
	candidate := mirrorMetadata{SchemaVersion: 1, RegistryRepository: defaultRegistry, DirectoryFeedID: modernFixtureFeed, DirectorySequence: 1, DirectoryDigest: "new"}
	path := filepath.Join(t.TempDir(), "MIRROR_METADATA.json")
	for _, priorFeed := range []string{"", "other-modern"} {
		previous.DirectoryFeedID = priorFeed
		if err := os.WriteFile(path, fixtureJSON(t, previous), 0600); err != nil {
			t.Fatal(err)
		}
		if err := enforceMonotonic(path, candidate); err == nil || !strings.Contains(err.Error(), "lineage") {
			t.Fatalf("modern sequence one borrowed %q floor: %v", priorFeed, err)
		}
	}
	if err := os.WriteFile(path, fixtureJSON(t, candidate), 0600); err != nil {
		t.Fatal(err)
	}
	wrong := candidate
	wrong.DirectoryFeedID = ""
	if err := enforceMonotonic(path, wrong); err == nil {
		t.Fatal("historical marker borrowed the modern floor")
	}
	if err := enforceMonotonic(path, candidate); err != nil {
		t.Fatalf("same modern publication refused: %v", err)
	}
	wrong = candidate
	wrong.DirectoryDigest = "conflict"
	if err := enforceMonotonic(path, wrong); err == nil {
		t.Fatal("modern same-sequence conflict accepted")
	}
}

func TestModernDirectoryMirrorToActualBuildLoader(t *testing.T) {
	approved, private := modernFixtureAuthority()
	profile, err := newModernDirectoryProfile(approved)
	if err != nil {
		t.Fatal(err)
	}
	client, _, _ := signedMirrorFixture(t, modernFixtureBytes(t), profile, approved.KeyID, private)
	result, err := fetchDirectoryWithProfile(client, defaultRegistry, profile)
	if err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	for _, file := range directoryFiles(result) {
		if err := writeStaged(stage, file.rel, file.body); err != nil {
			t.Fatal(err)
		}
	}
	marker := mirrorMetadata{SchemaVersion: 1, RegistryRepository: defaultRegistry, DirectoryFeedID: result.feedID,
		DirectorySequence: result.bundle.Snapshot.Sequence, DirectoryDigest: result.bundle.Digest, DirectoryCommit: result.bundle.Snapshot.SourceCommit}
	if err := writeStaged(stage, "MIRROR_METADATA.json", fixtureJSON(t, marker)); err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("the signed mirror/build integration requires Node 22.21.1 or newer")
	}
	command := exec.CommandContext(t.Context(), node, "--experimental-strip-types", filepath.Join(repository, "landing", "tests", "helpers", "modern-mirror-projection.mjs"), stage)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("actual TypeScript build projection failed: %v\n%s", err, output)
	}
}
