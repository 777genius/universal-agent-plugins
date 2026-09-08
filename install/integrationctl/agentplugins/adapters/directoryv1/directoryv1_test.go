package directoryv1

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var fixtureNow = time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

func fixtureKey(id string, state KeyState, fill byte) (TrustedKey, ed25519.PrivateKey) {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = fill
	}
	private := ed25519.NewKeyFromSeed(seed)
	return TrustedKey{ID: id, PublicKey: append(ed25519.PublicKey(nil), private.Public().(ed25519.PublicKey)...), State: state}, private
}

func fixtureSnapshot(sequence uint64) domain.DirectorySnapshot {
	release := domain.DirectoryRelease{Sequence: 9, PackageVersion: "not-semver", ManifestName: "tool", AgentPluginsSchema: supportedPluginSchema, PackageSource: domain.DirectorySource{Repository: "owner/repo", Revision: "0123456789012345678901234567890123456789", Path: "plugin"}, TreeDigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ManifestDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Components: []string{"mcp", "skills"}, PublishedAt: "2026-08-20T10:00:00Z"}
	policy := domain.DirectoryReleasePolicy{ReleaseSequence: 9, Status: domain.ReleaseActive, MinimumInstallerVersion: "1.0.0", Targets: []domain.DirectoryTarget{{Client: domain.ClientCodex, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "managed", Authentication: domain.AuthenticationRequirementUnknown}, {Client: domain.ClientCursor, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "managed", Authentication: domain.AuthenticationRequirementUnknown}}, CurrentEvidence: []string{"schema-pass"}}
	evidence := domain.DirectoryEvidence{SchemaVersion: 1, ID: "schema-pass", DistributionID: "owner/tool", ReleaseSequence: 9, PackageTreeDigest: release.TreeDigest, Level: "schema", Outcome: "passed", Artifact: domain.DirectoryEvidenceArtifact{Repository: "owner/evidence", Revision: "abcdefabcdefabcdefabcdefabcdefabcdefabcd", Path: "evidence/schema.json", Digest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}, Trust: &domain.DirectoryEvidenceTrust{Kind: "github_actions", Workflow: "owner/evidence/.github/workflows/directory.yml", SourceRef: "refs/heads/main", SourceDigest: "abcdefabcdefabcdefabcdefabcdefabcdefabcd"}}
	if sequence < domain.DirectoryEvidenceTrustCutoverSequence {
		evidence.ProductID = "tool"
		evidence.ManifestDigest = release.ManifestDigest
		evidence.SourceRepository = release.PackageSource.Repository
		evidence.SourceRevision = release.PackageSource.Revision
		evidence.SourcePath = release.PackageSource.Path
		evidence.AdapterVersion = "0.1.13"
		evidence.Trust = nil
	}
	product := domain.DirectoryProduct{SchemaVersion: 1, ID: "tool", DisplayName: "Tool", Description: "Tool description", ManifestName: "tool", Aliases: []string{"tool"}, ReservedAliases: []string{"tool"}, Categories: []string{"tools"}, MinimumCapabilities: domain.DirectoryMinimumCapabilities{Skills: "optional", MCP: "required"}, DefaultDistribution: "owner/tool", Distributions: []string{"owner/tool"}}
	distribution := domain.DirectoryDistribution{SchemaVersion: 1, ID: "owner/tool", ProductID: "tool", Kind: domain.DistributionUpstream, Status: domain.DistributionActive, Packager: "owner", Releases: []domain.DirectoryRelease{release}, ReleasePolicies: []domain.DirectoryReleasePolicy{policy}}
	return domain.DirectorySnapshot{SnapshotSchemaVersion: 1, Sequence: sequence, PublicationID: fmt.Sprintf("publication-%d", sequence), SourceCommit: "abcdefabcdefabcdefabcdefabcdefabcdefabcd", GeneratedAt: "2026-08-20T11:00:00Z", ExpiresAt: "2026-09-19T11:00:00Z", Products: []domain.DirectoryProduct{product}, Distributions: []domain.DirectoryDistribution{distribution}, Evidence: []domain.DirectoryEvidence{evidence}, Revocations: []domain.DirectoryRevocation{}}
}

func TestSnapshotEvidenceCutoverAcceptsBothStrictShapes(t *testing.T) {
	legacy := fixtureSnapshot(13)
	legacyBody, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSnapshot(legacyBody); err != nil {
		t.Fatalf("legacy production evidence rejected: %v", err)
	}

	legacy.Evidence[0].SourceRevision = strings.Repeat("f", 40)
	legacyBody, _ = json.Marshal(legacy)
	if _, err := ParseSnapshot(legacyBody); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("legacy source rebind accepted: %v", err)
	}

	wire := fixtureSnapshot(15)
	wireBody, _ := json.Marshal(wire)
	if _, err := ParseSnapshot(wireBody); err != nil {
		t.Fatalf("wire evidence rejected: %v", err)
	}
	wire.Evidence[0].AdapterVersion = "0.1.13"
	wireBody, _ = json.Marshal(wire)
	if _, err := ParseSnapshot(wireBody); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("legacy field accepted after cutover: %v", err)
	}

	wire = fixtureSnapshot(15)
	var raw map[string]any
	if err := json.Unmarshal(encodeSnapshot(t, wire), &raw); err != nil {
		t.Fatal(err)
	}
	raw["evidence"].([]any)[0].(map[string]any)["product_id"] = ""
	wireBody, _ = json.Marshal(raw)
	if _, err := ParseSnapshot(wireBody); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("empty legacy field accepted after cutover: %v", err)
	}
}

func encodeSnapshot(t *testing.T, value domain.DirectorySnapshot) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func signedFixture(t *testing.T, sequence uint64, key TrustedKey, private ed25519.PrivateKey) ([]byte, []byte, Pointer) {
	t.Helper()
	return signedSnapshotFixture(t, fixtureSnapshot(sequence), key, private)
}

func signedSnapshotFixture(t *testing.T, value domain.DirectorySnapshot, key TrustedKey, private ed25519.PrivateKey) ([]byte, []byte, Pointer) {
	t.Helper()
	sequence := value.Sequence
	snapshot, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	snapshot = append(snapshot, '\n')
	sum := sha256.Sum256(snapshot)
	message := SnapshotSignatureMessage(snapshot)
	signature := ed25519.Sign(private, message)
	envelope := Envelope{EnvelopeSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: sequence, KeyID: key.ID, Algorithm: "Ed25519", SignatureDomain: SignatureDomain, SnapshotDigest: "sha256:" + hex.EncodeToString(sum[:]), Signature: base64.StdEncoding.EncodeToString(signature)}
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	pointer := Pointer{PointerSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: sequence, SnapshotPath: fmt.Sprintf("snapshots/%020d.json", sequence), EnvelopePath: fmt.Sprintf("snapshots/%020d.envelope.json", sequence), FetchContract: FetchContract{HTTPSRequired: true, SameOriginRedirectsOnly: true, ForwardCredentialsOnRedirect: false, MaxRedirects: 2, LatestMaxBytes: MaxLatestBytes, SnapshotMaxBytes: MaxSnapshotBytes, EnvelopeMaxBytes: MaxEnvelopeBytes, RetryAttempts: 3}}
	return snapshot, envelopeBytes, pointer
}

func TestVerifyCurrentNextUnknownRetiredAndTamper(t *testing.T) {
	current, currentPrivate := fixtureKey("current-2026", KeyCurrent, 1)
	next, nextPrivate := fixtureKey("next-2026", KeyNext, 2)
	for _, item := range []struct {
		name    string
		key     TrustedKey
		private ed25519.PrivateKey
	}{{"current", current, currentPrivate}, {"next", next, nextPrivate}} {
		t.Run(item.name, func(t *testing.T) {
			snapshot, envelope, _ := signedFixture(t, 7, item.key, item.private)
			if _, err := VerifyBundle(snapshot, envelope, TrustStore{Keys: []TrustedKey{current, next}}); err != nil {
				t.Fatal(err)
			}
		})
	}
	snapshot, envelope, _ := signedFixture(t, 7, current, currentPrivate)
	if _, err := VerifyBundle(snapshot, envelope, TrustStore{}); !errors.Is(err, ErrNoTrustedKeys) {
		t.Fatalf("empty production trust must fail closed: %v", err)
	}
	if _, err := VerifyBundle(snapshot, envelope, TrustStore{Keys: []TrustedKey{next}}); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("unknown key: %v", err)
	}
	retired := current
	retired.State = KeyRetired
	if _, err := VerifyBundle(snapshot, envelope, TrustStore{Keys: []TrustedKey{retired}}); !errors.Is(err, ErrRetiredKey) {
		t.Fatalf("retired key: %v", err)
	}
	tampered := append([]byte(nil), snapshot...)
	tampered[len(tampered)-2] ^= 1
	if _, err := VerifyBundle(tampered, envelope, TrustStore{Keys: []TrustedKey{current}}); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("tamper: %v", err)
	}
	var parsed Envelope
	if err := json.Unmarshal(envelope, &parsed); err != nil {
		t.Fatal(err)
	}
	parsed.Signature = base64.StdEncoding.EncodeToString(make([]byte, 64))
	badSignature, _ := json.Marshal(parsed)
	if _, err := VerifyBundle(snapshot, badSignature, TrustStore{Keys: []TrustedKey{current}}); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("signature: %v", err)
	}
	parsed.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(currentPrivate, append([]byte("wrong\x00"), snapshot...)))
	badSignature, _ = json.Marshal(parsed)
	if _, err := VerifyBundle(snapshot, badSignature, TrustStore{Keys: []TrustedKey{current}}); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("domain separation: %v", err)
	}
	legacyUnprefixed := append(append([]byte(nil), signaturePrefix...), snapshot...)
	parsed.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(currentPrivate, legacyUnprefixed))
	badSignature, _ = json.Marshal(parsed)
	if _, err := VerifyBundle(snapshot, badSignature, TrustStore{Keys: []TrustedKey{current}}); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("legacy unprefixed message: %v", err)
	}
}

func TestStrictJSONSchemaDigestAndSequenceFailures(t *testing.T) {
	key, private := fixtureKey("fixture-key", KeyCurrent, 3)
	snapshot, envelope, pointer := signedFixture(t, 11, key, private)
	trust := TrustStore{Keys: []TrustedKey{key}}
	badBodies := [][]byte{
		[]byte(`{"pointer_schema_version":1,"pointer_schema_version":1}`),
		[]byte(`{"pointer_schema_version":1} {}`),
		[]byte(`{"pointer_schema_version":1,"snapshot_schema_version":1,"sequence":1,"snapshot_path":"snapshots/00000000000000000001.json","envelope_path":"snapshots/00000000000000000001.envelope.json","fetch_contract":{"https_required":true,"same_origin_redirects_only":true,"forward_credentials_on_redirect":false,"max_redirects":0,"latest_max_bytes":1,"snapshot_max_bytes":1,"envelope_max_bytes":1,"retry_attempts":1},"unknown":1}`),
	}
	for _, body := range badBodies {
		if _, err := ParsePointer(body); !errors.Is(err, ErrStrictJSON) {
			t.Fatalf("strict pointer accepted %s: %v", body, err)
		}
	}
	var raw map[string]any
	if err := json.Unmarshal(snapshot, &raw); err != nil {
		t.Fatal(err)
	}
	raw["snapshot_schema_version"] = 2
	badSchema, _ := json.Marshal(raw)
	if _, err := ParseSnapshot(badSchema); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("schema: %v", err)
	}
	delete(raw, "source_commit")
	missing, _ := json.Marshal(raw)
	if _, err := ParseSnapshot(missing); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("missing required: %v", err)
	}
	pointer.Sequence++
	if _, err := VerifyPointerBundle(pointer, snapshot, envelope, trust); !errors.Is(err, ErrSequenceMismatch) {
		t.Fatalf("sequence mismatch: %v", err)
	}
	var env Envelope
	_ = json.Unmarshal(envelope, &env)
	env.Sequence++
	sequenceEnvelope, _ := json.Marshal(env)
	if _, err := VerifyBundle(snapshot, sequenceEnvelope, trust); !errors.Is(err, ErrSequenceMismatch) {
		t.Fatalf("envelope/snapshot sequence mismatch: %v", err)
	}
	env.Sequence--
	env.SnapshotDigest = "sha256:" + strings.Repeat("0", 64)
	digestEnvelope, _ := json.Marshal(env)
	if _, err := VerifyBundle(snapshot, digestEnvelope, trust); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("digest mismatch: %v", err)
	}
}

func TestSnapshotStrictSchemaOneSemantics(t *testing.T) {
	base := fixtureSnapshot(15)
	encode := func(value domain.DirectorySnapshot) []byte {
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	if _, err := ParseSnapshot(encode(base)); err != nil {
		t.Fatalf("authoritative top-level schema fixture: %v", err)
	}
	// A top-level revocation remains authoritative even during an emergency
	// publication whose release policy has not yet changed.
	revoked := base
	revoked.Revocations = []domain.DirectoryRevocation{{DistributionID: "owner/tool", ReleaseSequence: 9}}
	if _, err := ParseSnapshot(encode(revoked)); err != nil {
		t.Fatalf("authoritative top-level revocation rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*domain.DirectorySnapshot)
	}{
		{"malformed product ID", func(v *domain.DirectorySnapshot) { v.Products[0].ID = "Bad_ID" }},
		{"malformed source SHA", func(v *domain.DirectorySnapshot) { v.Distributions[0].Releases[0].PackageSource.Revision = "main" }},
		{"noncanonical tree digest algorithm", func(v *domain.DirectorySnapshot) {
			v.Distributions[0].Releases[0].TreeDigestAlgorithm = "uap-tree-sha256-v1"
		}},
		{"malformed time", func(v *domain.DirectorySnapshot) { v.GeneratedAt = "2026-08-20T11:00:00+00:00" }},
		{"overlong lifetime", func(v *domain.DirectorySnapshot) { v.ExpiresAt = "2026-09-19T11:00:01Z" }},
		{"unknown enum", func(v *domain.DirectorySnapshot) { v.Distributions[0].Kind = domain.DistributionKind("mirror") }},
		{"duplicate product", func(v *domain.DirectorySnapshot) { v.Products = append(v.Products, v.Products[0]) }},
		{"reserved alias takeover", func(v *domain.DirectorySnapshot) {
			other := v.Products[0]
			other.ID = "other"
			other.ManifestName = "other"
			other.Aliases = []string{"other", "tool"}
			other.ReservedAliases = []string{"other"}
			other.DefaultDistribution = "other/tool"
			other.Distributions = []string{"other/tool"}
			v.Products = append(v.Products, other)
		}},
		{"duplicate release", func(v *domain.DirectorySnapshot) {
			v.Distributions[0].Releases = append(v.Distributions[0].Releases, v.Distributions[0].Releases[0])
		}},
		{"policy sequence mismatch", func(v *domain.DirectorySnapshot) { v.Distributions[0].ReleasePolicies[0].ReleaseSequence = 10 }},
		{"evidence digest mismatch", func(v *domain.DirectorySnapshot) {
			v.Evidence[0].PackageTreeDigest = "sha256:" + strings.Repeat("d", 64)
		}},
		{"unsafe source path", func(v *domain.DirectorySnapshot) { v.Distributions[0].Releases[0].PackageSource.Path = "../plugin" }},
		{"unsafe app binding alias", func(v *domain.DirectorySnapshot) {
			v.Distributions[0].ReleasePolicies[0].Targets[0] = domain.DirectoryTarget{Client: domain.ClientChatGPT, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "prepared", Authentication: domain.AuthenticationRequirementUnknown,
				AppBinding: &domain.DirectoryAppBinding{AppKey: "../docs", ID: "asdk_app_docs_123", MCPServer: "../docs"}}
		}},
		{"mismatched app binding aliases", func(v *domain.DirectorySnapshot) {
			v.Distributions[0].ReleasePolicies[0].Targets[0] = domain.DirectoryTarget{Client: domain.ClientChatGPT, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "prepared", Authentication: domain.AuthenticationRequirementUnknown,
				AppBinding: &domain.DirectoryAppBinding{AppKey: "docs", ID: "asdk_app_docs_123", MCPServer: "other"}}
		}},
		{"unsafe app binding id", func(v *domain.DirectorySnapshot) {
			v.Distributions[0].ReleasePolicies[0].Targets[0] = domain.DirectoryTarget{Client: domain.ClientChatGPT, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "prepared", Authentication: domain.AuthenticationRequirementUnknown,
				AppBinding: &domain.DirectoryAppBinding{AppKey: "docs", ID: "not/an/id", MCPServer: "docs"}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value := fixtureSnapshot(15)
			tc.mutate(&value)
			if _, err := ParseSnapshot(encode(value)); !errors.Is(err, ErrStrictJSON) {
				t.Fatalf("invalid snapshot accepted: %v", err)
			}
		})
	}

	var raw map[string]any
	if err := json.Unmarshal(encode(base), &raw); err != nil {
		t.Fatal(err)
	}
	distributions := raw["distributions"].([]any)
	release := distributions[0].(map[string]any)["releases"].([]any)[0].(map[string]any)
	delete(release, "package_version")
	missingNested, _ := json.Marshal(raw)
	if _, err := ParseSnapshot(missingNested); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("missing nested required field accepted: %v", err)
	}
	raw["unknown"] = true
	unknown, _ := json.Marshal(raw)
	if _, err := ParseSnapshot(unknown); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("unknown snapshot field accepted: %v", err)
	}
	trailing := append(encode(base), []byte(` {}`)...)
	if _, err := ParseSnapshot(trailing); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("trailing snapshot JSON accepted: %v", err)
	}
}

func TestEvidenceEligibilityTrustContract(t *testing.T) {
	encode := func(t *testing.T, value domain.DirectorySnapshot) []byte {
		t.Helper()
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	reviewed := &domain.DirectoryEvidenceTrust{Kind: "reviewed_external"}
	tests := []struct {
		name    string
		mutate  func(*domain.DirectorySnapshot)
		wantErr bool
	}{
		{name: "trusted pass"},
		{name: "trusted fail", mutate: func(v *domain.DirectorySnapshot) { v.Evidence[0].Outcome = "failed" }},
		{name: "missing trust cannot be current", mutate: func(v *domain.DirectorySnapshot) { v.Evidence[0].Trust = nil }, wantErr: true},
		{name: "unknown trust", mutate: func(v *domain.DirectorySnapshot) { v.Evidence[0].Trust.Kind = "contributor_asserted" }, wantErr: true},
		{name: "forged workflow repository", mutate: func(v *domain.DirectorySnapshot) {
			v.Evidence[0].Trust.Workflow = "contributor/evidence/.github/workflows/directory.yml"
		}, wantErr: true},
		{name: "forged source digest", mutate: func(v *domain.DirectorySnapshot) {
			v.Evidence[0].Trust.SourceDigest = "dddddddddddddddddddddddddddddddddddddddd"
		}, wantErr: true},
		{name: "incompatible reviewed gate", mutate: func(v *domain.DirectorySnapshot) {
			v.Evidence[0].Level = "materialization"
			v.Evidence[0].Client = domain.ClientCodex
			v.Evidence[0].ClientVersion = "1.0.0"
			v.Evidence[0].InstallerVersion = "1.0.0"
			v.Evidence[0].OS = "linux"
			v.Evidence[0].Architecture = "amd64"
			v.Evidence[0].ObservedAt = "2026-08-20T10:00:00Z"
			v.Evidence[0].Trust = reviewed
		}},
		{name: "reviewed runtime fail accepted", mutate: func(v *domain.DirectorySnapshot) {
			v.Evidence[0].Level = "runtime"
			v.Evidence[0].Outcome = "failed"
			v.Evidence[0].Trust = reviewed
		}},
		{name: "reviewed schema fail incompatible", mutate: func(v *domain.DirectorySnapshot) {
			v.Evidence[0].Level = "schema"
			v.Evidence[0].Outcome = "failed"
			v.Evidence[0].Client = ""
			v.Evidence[0].ClientVersion = ""
			v.Evidence[0].InstallerVersion = ""
			v.Evidence[0].OS = ""
			v.Evidence[0].Architecture = ""
			v.Evidence[0].ObservedAt = ""
			v.Evidence[0].Trust = reviewed
		}},
		{name: "non-current evidence", mutate: func(v *domain.DirectorySnapshot) { v.Distributions[0].ReleasePolicies[0].CurrentEvidence = []string{} }, wantErr: true},
		{name: "informational without trust cannot be current", mutate: func(v *domain.DirectorySnapshot) { v.Evidence[0].Outcome = "inconclusive"; v.Evidence[0].Trust = nil }, wantErr: true},
		{name: "failure without trust cannot be current", mutate: func(v *domain.DirectorySnapshot) { v.Evidence[0].Outcome = "failed"; v.Evidence[0].Trust = nil }, wantErr: true},
		{name: "reviewed informational accepted", mutate: func(v *domain.DirectorySnapshot) {
			v.Evidence[0].Outcome = "inconclusive"
			v.Evidence[0].Trust = reviewed
		}},
		{name: "github actions informational accepted", mutate: func(v *domain.DirectorySnapshot) { v.Evidence[0].Outcome = "not_tested" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			value := fixtureSnapshot(16)
			value.Evidence[0].Level = "materialization"
			value.Evidence[0].Client = domain.ClientCodex
			value.Evidence[0].ClientVersion = "1.0.0"
			value.Evidence[0].InstallerVersion = "1.0.0"
			value.Evidence[0].OS = "linux"
			value.Evidence[0].Architecture = "amd64"
			value.Evidence[0].ObservedAt = "2026-08-20T10:00:00Z"
			if tc.mutate != nil {
				tc.mutate(&value)
			}
			_, err := ParseSnapshot(encode(t, value))
			if tc.wantErr && !errors.Is(err, ErrStrictJSON) {
				t.Fatalf("untrusted evidence accepted: %v", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("valid evidence rejected: %v", err)
			}
		})
	}
}

func directoryServer(t *testing.T, pointer Pointer, snapshot, envelope []byte, failArtifacts int32) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	pointerBytes, _ := json.Marshal(pointer)
	attempts := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry/schemas/1/latest.json":
			w.Write(pointerBytes)
		case "/registry/schemas/1/" + pointer.SnapshotPath, "/registry/schemas/1/" + pointer.EnvelopePath:
			if attempts.Add(1) <= failArtifacts {
				http.Error(w, "partial", http.StatusNotFound)
				return
			}
			if strings.HasSuffix(r.URL.Path, ".envelope.json") {
				w.Write(envelope)
			} else {
				w.Write(snapshot)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	return server, attempts
}

func testClient(server *httptest.Server, key TrustedKey, embedded EmbeddedBundle, cachePath string) Client {
	return Client{Origin: server.URL + "/registry/schemas/1/", HTTPClient: server.Client(), Trust: TrustStore{Keys: []TrustedKey{key}}, Embedded: embedded, Cache: Cache{Path: cachePath}, Now: func() time.Time { return fixtureNow }, AllowHTTPForTests: true}
}

func TestSafeFetchRetryPointerRedirectAndSize(t *testing.T) {
	key, private := fixtureKey("fixture-key", KeyCurrent, 4)
	snapshot, envelope, pointer := signedFixture(t, 12, key, private)
	server, attempts := directoryServer(t, pointer, snapshot, envelope, 2)
	defer server.Close()
	client := testClient(server, key, EmbeddedBundle{}, filepath.Join(t.TempDir(), "cache.json"))
	if _, err := client.Load(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() < 4 {
		t.Fatalf("artifact retry did not occur: %d", attempts.Load())
	}
	unsafe := pointer
	unsafe.SnapshotPath = "https://evil.invalid/snapshot.json"
	unsafeBytes, _ := json.Marshal(unsafe)
	if _, err := ParsePointer(unsafeBytes); err == nil {
		t.Fatal("absolute pointer accepted")
	}
	oversizeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(make([]byte, MaxLatestBytes+1)) }))
	defer oversizeServer.Close()
	oversizeClient := testClient(oversizeServer, key, EmbeddedBundle{}, filepath.Join(t.TempDir(), "cache.json"))
	oversizeClient.Origin = oversizeServer.URL + "/"
	if _, err := oversizeClient.Load(context.Background(), 0); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("oversize: %v", err)
	}
	cross := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.invalid/registry/schemas/1/latest.json", http.StatusFound)
	}))
	defer cross.Close()
	redirectClient := testClient(cross, key, EmbeddedBundle{}, filepath.Join(t.TempDir(), "cache.json"))
	redirectClient.Origin = cross.URL + "/"
	if _, err := redirectClient.Load(context.Background(), 0); !errors.Is(err, ErrUnsafeRedirect) {
		t.Fatalf("cross-origin redirect: %v", err)
	}
	production := client
	production.Cache = Cache{Path: filepath.Join(t.TempDir(), "empty.json")}
	production.AllowHTTPForTests = false
	if _, err := production.Load(context.Background(), 0); !errors.Is(err, ErrUnsafeOrigin) {
		t.Fatalf("production HTTP accepted: %v", err)
	}
}

func TestSameOriginRedirectDoesNotForwardCredentials(t *testing.T) {
	key, private := fixtureKey("fixture-key", KeyCurrent, 9)
	snapshot, envelope, pointer := signedFixture(t, 13, key, private)
	pointerBytes, _ := json.Marshal(pointer)
	var leaked atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			leaked.Store(true)
		}
		switch r.URL.Path {
		case "/registry/schemas/1/latest.json":
			http.Redirect(w, r, "/registry/schemas/1/redirected.json", http.StatusFound)
		case "/registry/schemas/1/redirected.json":
			_, _ = w.Write(pointerBytes)
		case "/registry/schemas/1/" + pointer.SnapshotPath:
			_, _ = w.Write(snapshot)
		case "/registry/schemas/1/" + pointer.EnvelopePath:
			_, _ = w.Write(envelope)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	serverURL, _ := url.Parse(server.URL)
	jar.SetCookies(serverURL, []*http.Cookie{{Name: "secret", Value: "must-not-be-sent"}})
	httpClient := server.Client()
	httpClient.Jar = jar
	httpClient.CheckRedirect = func(request *http.Request, _ []*http.Request) error {
		request.Header.Set("Authorization", "must-not-be-sent")
		return nil
	}
	client := testClient(server, key, EmbeddedBundle{}, filepath.Join(t.TempDir(), "cache.json"))
	client.HTTPClient = httpClient
	if bundle, err := client.Load(context.Background(), 0); err != nil || bundle.Source != BundleSourceRemote {
		t.Fatalf("same-origin redirect: %+v %v", bundle, err)
	}
	if leaked.Load() {
		t.Fatal("ambient credentials reached Directory request or redirect")
	}
}

func TestCacheAtomicPermissionsPreservationAndOfflineExpiry(t *testing.T) {
	key, private := fixtureKey("fixture-key", KeyCurrent, 5)
	trust := TrustStore{Keys: []TrustedKey{key}}
	snapshot, envelope, _ := signedFixture(t, 20, key, private)
	bundle, err := VerifyBundle(snapshot, envelope, trust)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	cache := Cache{Path: filepath.Join(directory, "lkg.json")}
	if err := cache.Store(bundle, trust); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(cache.Path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("cache mode %o", info.Mode().Perm())
	}
	lowerSnapshot, lowerEnvelope, _ := signedFixture(t, 19, key, private)
	lower, _ := VerifyBundle(lowerSnapshot, lowerEnvelope, trust)
	if err := cache.Store(lower, trust); !errors.Is(err, ErrCacheRollback) {
		t.Fatalf("lower cache replacement: %v", err)
	}
	loaded, err := cache.Load(trust)
	if err != nil || loaded.Snapshot.Sequence != 20 {
		t.Fatalf("LKG changed: %d %v", loaded.Snapshot.Sequence, err)
	}
	if string(loaded.SnapshotBytes) != string(snapshot) || string(loaded.EnvelopeBytes) != string(envelope) {
		t.Fatal("cache did not preserve exact verified bytes")
	}
	higherSnapshot, higherEnvelope, _ := signedFixture(t, 21, key, private)
	higher, _ := VerifyBundle(higherSnapshot, higherEnvelope, trust)
	interrupted := cache
	interrupted.BeforeRename = func(string) error { return errors.New("interrupted") }
	if err := interrupted.Store(higher, trust); err == nil {
		t.Fatal("interruption not injected")
	}
	loaded, _ = cache.Load(trust)
	if loaded.Snapshot.Sequence != 20 {
		t.Fatal("interrupted write replaced LKG")
	}
	client := Client{Origin: "https://127.0.0.1:1/registry/schemas/1/", HTTPClient: &http.Client{Timeout: time.Millisecond}, Trust: trust, Embedded: EmbeddedBundle{Snapshot: snapshot, Envelope: envelope}, Cache: cache, Now: func() time.Time { return fixtureNow }}
	if got, err := client.Load(context.Background(), 0); err != nil || got.Snapshot.Sequence != 20 {
		t.Fatalf("offline LKG: %v %v", got.Snapshot.Sequence, err)
	}
	client.Now = func() time.Time { return time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC) }
	if _, err := client.Load(context.Background(), 0); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired offline: %v", err)
	}
	client.Now = func() time.Time { return time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC) }
	if _, err := client.Load(context.Background(), 0); !errors.Is(err, ErrClockSkew) {
		t.Fatalf("clock skew: %v", err)
	}
}

func TestCacheConcurrentStoresCannotPublishLowerSequenceLast(t *testing.T) {
	key, private := fixtureKey("fixture-key", KeyCurrent, 12)
	trust := TrustStore{Keys: []TrustedKey{key}}
	bundle := func(sequence uint64) VerifiedBundle {
		snapshot, envelope, _ := signedFixture(t, sequence, key, private)
		verified, err := VerifyBundle(snapshot, envelope, trust)
		if err != nil {
			t.Fatal(err)
		}
		return verified
	}
	cache := Cache{Path: filepath.Join(t.TempDir(), "lkg.json")}
	initial, lowerBundle, higherBundle := bundle(40), bundle(41), bundle(42)
	if err := cache.Store(initial, trust); err != nil {
		t.Fatal(err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	lower := cache
	lower.BeforeRename = func(string) error {
		close(entered)
		<-release
		return nil
	}
	var wait sync.WaitGroup
	wait.Add(2)
	errorsByWriter := make(chan error, 2)
	go func() {
		defer wait.Done()
		errorsByWriter <- lower.Store(lowerBundle, trust)
	}()
	<-entered
	go func() {
		defer wait.Done()
		errorsByWriter <- cache.Store(higherBundle, trust)
	}()
	close(release)
	wait.Wait()
	close(errorsByWriter)
	for err := range errorsByWriter {
		if err != nil {
			t.Fatalf("concurrent store: %v", err)
		}
	}
	loaded, err := cache.Load(trust)
	if err != nil || loaded.Snapshot.Sequence != 42 {
		t.Fatalf("final cache sequence = %d, err = %v", loaded.Snapshot.Sequence, err)
	}
}

func TestCacheConcurrentLowerStoreRechecksAfterHigherCommit(t *testing.T) {
	key, private := fixtureKey("fixture-key", KeyCurrent, 13)
	trust := TrustStore{Keys: []TrustedKey{key}}
	bundle := func(sequence uint64) VerifiedBundle {
		snapshot, envelope, _ := signedFixture(t, sequence, key, private)
		verified, err := VerifyBundle(snapshot, envelope, trust)
		if err != nil {
			t.Fatal(err)
		}
		return verified
	}
	cache := Cache{Path: filepath.Join(t.TempDir(), "lkg.json")}
	initial, lowerBundle, higherBundle := bundle(50), bundle(51), bundle(52)
	if err := cache.Store(initial, trust); err != nil {
		t.Fatal(err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	higher := cache
	higher.BeforeRename = func(string) error {
		close(entered)
		<-release
		return nil
	}
	higherErr := make(chan error, 1)
	go func() { higherErr <- higher.Store(higherBundle, trust) }()
	<-entered
	lowerErr := make(chan error, 1)
	go func() { lowerErr <- cache.Store(lowerBundle, trust) }()
	close(release)
	if err := <-higherErr; err != nil {
		t.Fatalf("higher store: %v", err)
	}
	if err := <-lowerErr; !errors.Is(err, ErrCacheRollback) {
		t.Fatalf("lower store after higher commit = %v", err)
	}
	loaded, err := cache.Load(trust)
	if err != nil || loaded.Snapshot.Sequence != 52 {
		t.Fatalf("final cache sequence = %d, err = %v", loaded.Snapshot.Sequence, err)
	}
}

func TestCacheAcceptsBase64ExpansionNearSnapshotLimit(t *testing.T) {
	key, private := fixtureKey("fixture-key", KeyCurrent, 11)
	trust := TrustStore{Keys: []TrustedKey{key}}
	snapshot, _, _ := signedFixture(t, 22, key, private)
	// JSON permits trailing whitespace and the signature authenticates it. This
	// keeps the semantic fixture small while exercising a cache record whose
	// base64 representation is larger than the raw snapshot-size bound.
	snapshot = append(snapshot, bytes.Repeat([]byte{' '}, 3<<20)...)
	sum := sha256.Sum256(snapshot)
	signature := ed25519.Sign(private, SnapshotSignatureMessage(snapshot))
	envelope, err := json.Marshal(Envelope{EnvelopeSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: 22, KeyID: key.ID, Algorithm: "Ed25519", SignatureDomain: SignatureDomain, SnapshotDigest: "sha256:" + hex.EncodeToString(sum[:]), Signature: base64.StdEncoding.EncodeToString(signature)})
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := VerifyBundle(snapshot, envelope, trust)
	if err != nil {
		t.Fatal(err)
	}
	cache := Cache{Path: filepath.Join(t.TempDir(), "lkg.json")}
	if err := cache.Store(bundle, trust); err != nil {
		t.Fatal(err)
	}
	loaded, err := cache.Load(trust)
	if err != nil || loaded.Digest != bundle.Digest || !bytes.Equal(loaded.SnapshotBytes, snapshot) {
		t.Fatalf("expanded cache record: digest=%s err=%v", loaded.Digest, err)
	}
}

func TestRollbackFloorsAndInvalidNewerPreservesLKG(t *testing.T) {
	key, private := fixtureKey("fixture-key", KeyCurrent, 6)
	trust := TrustStore{Keys: []TrustedKey{key}}
	s1, e1, _ := signedFixture(t, 1, key, private)
	embedded := EmbeddedBundle{Snapshot: s1, Envelope: e1}
	s3, e3, _ := signedFixture(t, 3, key, private)
	b3, _ := VerifyBundle(s3, e3, trust)
	cachePath := filepath.Join(t.TempDir(), "lkg.json")
	cache := Cache{Path: cachePath}
	if err := cache.Store(b3, trust); err != nil {
		t.Fatal(err)
	}
	s2, e2, p2 := signedFixture(t, 2, key, private)
	server, _ := directoryServer(t, p2, s2, e2, 0)
	client := testClient(server, key, embedded, cachePath)
	defer server.Close()
	if got, err := client.Load(context.Background(), 0); err != nil || got.Snapshot.Sequence != 3 {
		t.Fatalf("cached floor fallback: %d %v", got.Snapshot.Sequence, err)
	}
	_ = os.Remove(cachePath)
	if _, err := client.Load(context.Background(), 4); !errors.Is(err, ErrRollback) {
		t.Fatalf("installed floor after cache loss: %v", err)
	}
	// Embedded floor independently rejects a remote rollback.
	s5, e5, _ := signedFixture(t, 5, key, private)
	embedded5 := EmbeddedBundle{Snapshot: s5, Envelope: e5}
	client.Embedded = embedded5
	if _, err := client.Load(context.Background(), 0); err != nil {
		t.Fatalf("embedded fallback failed: %v", err)
	}
	// A cryptographically invalid newer publication never replaces sequence 3.
	if err := cache.Store(b3, trust); err != nil {
		t.Fatal(err)
	}
	s4, e4, p4 := signedFixture(t, 4, key, private)
	s4[5] ^= 1
	badServer, _ := directoryServer(t, p4, s4, e4, 0)
	defer badServer.Close()
	badClient := testClient(badServer, key, EmbeddedBundle{}, cachePath)
	if got, err := badClient.Load(context.Background(), 0); err != nil || got.Snapshot.Sequence != 3 {
		t.Fatalf("invalid newer displaced LKG: %d %v", got.Snapshot.Sequence, err)
	}
	loaded, _ := cache.Load(trust)
	if loaded.Snapshot.Sequence != 3 {
		t.Fatal("cache replaced by invalid newer")
	}
}

func TestAuthenticatedSameSequenceEquivocationFailsClosed(t *testing.T) {
	key, private := fixtureKey("fixture-key", KeyCurrent, 10)
	trust := TrustStore{Keys: []TrustedKey{key}}
	firstSnapshot, firstEnvelope, _ := signedFixture(t, 30, key, private)
	changed := fixtureSnapshot(30)
	changed.PublicationID = "publication-30-conflict"
	secondSnapshot, secondEnvelope, secondPointer := signedSnapshotFixture(t, changed, key, private)
	first, err := VerifyBundle(firstSnapshot, firstEnvelope, trust)
	if err != nil {
		t.Fatal(err)
	}
	second, err := VerifyBundle(secondSnapshot, secondEnvelope, trust)
	if err != nil {
		t.Fatal(err)
	}
	if first.Digest == second.Digest {
		t.Fatal("equivocation fixture did not change authenticated bytes")
	}

	t.Run("cache store", func(t *testing.T) {
		cache := Cache{Path: filepath.Join(t.TempDir(), "lkg.json")}
		if err := cache.Store(first, trust); err != nil {
			t.Fatal(err)
		}
		if err := cache.Store(second, trust); !errors.Is(err, ErrSequenceConflict) {
			t.Fatalf("equal-sequence replacement: %v", err)
		}
		got, err := cache.Load(trust)
		if err != nil || got.Digest != first.Digest {
			t.Fatalf("conflict changed LKG: %+v %v", got, err)
		}
	})

	t.Run("cache versus embedded offline", func(t *testing.T) {
		cache := Cache{Path: filepath.Join(t.TempDir(), "lkg.json")}
		if err := cache.Store(first, trust); err != nil {
			t.Fatal(err)
		}
		client := Client{
			Origin:     "https://127.0.0.1:1/registry/schemas/1/",
			HTTPClient: &http.Client{Timeout: time.Millisecond},
			Trust:      trust,
			Embedded:   EmbeddedBundle{Snapshot: secondSnapshot, Envelope: secondEnvelope},
			Cache:      cache,
			Now:        func() time.Time { return fixtureNow },
		}
		if _, err := client.Load(context.Background(), 0); !errors.Is(err, ErrSequenceConflict) {
			t.Fatalf("offline cache/embed conflict: %v", err)
		}
	})

	t.Run("remote versus cache", func(t *testing.T) {
		cachePath := filepath.Join(t.TempDir(), "lkg.json")
		cache := Cache{Path: cachePath}
		if err := cache.Store(first, trust); err != nil {
			t.Fatal(err)
		}
		server, _ := directoryServer(t, secondPointer, secondSnapshot, secondEnvelope, 0)
		defer server.Close()
		client := testClient(server, key, EmbeddedBundle{}, cachePath)
		if _, err := client.Load(context.Background(), 0); !errors.Is(err, ErrSequenceConflict) {
			t.Fatalf("remote/cache conflict: %v", err)
		}
	})

	t.Run("remote versus embedded", func(t *testing.T) {
		_, _, firstPointer := signedFixture(t, 30, key, private)
		server, _ := directoryServer(t, firstPointer, firstSnapshot, firstEnvelope, 0)
		defer server.Close()
		client := testClient(server, key, EmbeddedBundle{Snapshot: secondSnapshot, Envelope: secondEnvelope}, filepath.Join(t.TempDir(), "empty.json"))
		if _, err := client.Load(context.Background(), 0); !errors.Is(err, ErrSequenceConflict) {
			t.Fatalf("remote/embed conflict: %v", err)
		}
	})
}

func TestDirectExactRequiresNoDirectory(t *testing.T) {
	revision := "0123456789012345678901234567890123456789"
	for _, canonical := range []string{
		"owner/repo@" + revision + "//plugin",
		"github:owner/repo@" + revision + "//plugin",
		"https://github.com/owner/repo@" + revision + "//plugin",
	} {
		t.Run(canonical, func(t *testing.T) {
			selection, err := ResolveDirectExact(domain.SourceIdentity{
				RequestedSource: "owner/repo@" + revision + "//plugin",
				CanonicalSource: canonical,
				Repository:      "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision,
			})
			if err != nil || selection.Label != "direct source" {
				t.Fatalf("exact source: %+v %v", selection, err)
			}
		})
	}
	for _, canonical := range []string{
		"owner/repo@" + revision,
		"github:owner/repo@" + revision,
		"https://github.com/owner/repo@" + revision,
	} {
		t.Run("repository root "+canonical, func(t *testing.T) {
			selection, err := ResolveDirectExact(domain.SourceIdentity{
				RequestedSource: canonical,
				CanonicalSource: canonical,
				Repository:      "owner/repo", ResolvedRevision: revision,
			})
			if err != nil || selection.Label != "direct source" {
				t.Fatalf("exact repository-root source: %+v %v", selection, err)
			}
		})
	}

	invalid := map[string]domain.SourceIdentity{
		"branch":              {CanonicalSource: "owner/repo@main//plugin", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: "main"},
		"tag":                 {CanonicalSource: "owner/repo@v1.0.0//plugin", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision},
		"unpinned":            {CanonicalSource: "owner/repo//plugin", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision},
		"repository mismatch": {CanonicalSource: "other/repo@" + revision + "//plugin", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision},
		"SHA mismatch":        {CanonicalSource: "owner/repo@" + strings.Repeat("a", 40) + "//plugin", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision},
		"subpath escape":      {CanonicalSource: "owner/repo@" + revision + "//plugin/../other", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision},
		"subpath mismatch":    {CanonicalSource: "owner/repo@" + revision + "//other", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision},
		"alternate host":      {CanonicalSource: "https://example.com/owner/repo@" + revision + "//plugin", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision},
		"query":               {CanonicalSource: "https://github.com/owner/repo@" + revision + "//plugin?ref=main", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision},
		"fragment":            {CanonicalSource: "https://github.com/owner/repo@" + revision + "//plugin#main", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision},
		"empty query":         {CanonicalSource: "https://github.com/owner/repo@" + revision + "//plugin?", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision},
		"empty fragment":      {CanonicalSource: "https://github.com/owner/repo@" + revision + "//plugin#", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision},
		"malformed URL":       {CanonicalSource: "https://github.com//owner/repo@" + revision + "//plugin", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision},
		"requested mismatch":  {RequestedSource: "owner/repo@" + revision + "//other", CanonicalSource: "https://github.com/owner/repo@" + revision + "//plugin", Repository: "owner/repo", PackageSubpath: "plugin", ResolvedRevision: revision},
	}
	for name, source := range invalid {
		t.Run(name, func(t *testing.T) {
			if _, err := ResolveDirectExact(source); err == nil {
				t.Fatal("unsafe or mismatched direct source accepted")
			}
		})
	}
	if _, err := ResolveDirectExact(domain.SourceIdentity{CanonicalSource: "./plugin"}); err != nil {
		t.Fatalf("local direct source rejected: %v", err)
	}
}

func TestDirectExactPreservesCaseSensitiveGitHubRepositoryIdentity(t *testing.T) {
	revision := strings.Repeat("a", 40)
	for _, prefix := range []string{"", "github:", "https://github.com/"} {
		value := prefix + "Azure/PluginRepo@" + revision + "//Plugin/Path"
		selection, err := ResolveDirectExact(domain.SourceIdentity{
			RequestedSource: value, CanonicalSource: value, Repository: "Azure/PluginRepo",
			PackageSubpath: "Plugin/Path", ResolvedRevision: revision,
		})
		if err != nil || selection.Source.Repository != "Azure/PluginRepo" {
			t.Fatalf("case-preserving exact source %q: %+v %v", value, selection, err)
		}
	}
}
