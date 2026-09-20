package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/cmd/agentplugins/internal/bootstrapio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var forbiddenProductionDirectoryVariables = []string{
	"AGENTPLUGINS_DIRECTORY_CONFORMANCE_ONLY",
	"AGENTPLUGINS_DIRECTORY_CONFORMANCE_SNAPSHOT",
	"AGENTPLUGINS_DIRECTORY_CONFORMANCE_ENVELOPE",
	"AGENTPLUGINS_DIRECTORY_CONFORMANCE_TRUST",
	"AGENTPLUGINS_DIRECTORY_CONFORMANCE_CACHE",
	"AGENTPLUGINS_DIRECTORY_SNAPSHOT",
	"AGENTPLUGINS_DIRECTORY_ENVELOPE",
	"AGENTPLUGINS_DIRECTORY_TRUST",
	"AGENTPLUGINS_DIRECTORY_CACHE",
}

func TestProductionDirectoryIgnoresCallerSuppliedConformanceData(t *testing.T) {
	clearDirectoryEnvironment(t)
	root := t.TempDir()
	fixture := writeConformanceFixture(t, root)
	for _, name := range forbiddenProductionDirectoryVariables {
		t.Setenv(name, fixtureValue(name, fixture))
	}
	productionRoot := filepath.Join(root, "production")
	client, err := newDirectoryClient(productionRoot)
	if err != nil {
		t.Fatal(err)
	}
	if client.Origin != defaultDirectoryOrigin || client.Cache.Path != filepath.Join(productionRoot, "directory-v1-cache.json") || len(client.Trust.Keys) != 1 || client.Trust.Keys[0].ID != defaultDirectoryKeyID {
		t.Fatalf("conformance environment changed production Directory configuration: %+v", client)
	}
	if _, err := os.Lstat(fixture["AGENTPLUGINS_DIRECTORY_CACHE"]); !os.IsNotExist(err) {
		t.Fatalf("production configuration touched caller cache: %v", err)
	}
}

func TestProductionDefaultsUseTheStandaloneRegistry(t *testing.T) {
	if want := "https://777genius.github.io/universal-agent-plugins-registry/"; !strings.HasPrefix(defaultDirectoryOrigin, want) || !strings.HasPrefix(defaultDiscoveryOrigin, want) || !strings.HasPrefix(defaultSecurityOrigin, want) {
		t.Fatalf("production feed defaults must use the standalone registry: directory=%q discovery=%q security=%q", defaultDirectoryOrigin, defaultDiscoveryOrigin, defaultSecurityOrigin)
	}
}

func TestProductionSecurityIndexUsesIndependentOriginAndPinnedTrust(t *testing.T) {
	clearDirectoryEnvironment(t)
	client, err := newSecurityClient(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if client.Origin != defaultSecurityOrigin || client.Trust.KeyID != defaultSecurityKeyID || len(client.Trust.PublicKey) != ed25519.PublicKeySize {
		t.Fatalf("production Security Index configuration = %+v", client)
	}
	t.Setenv("AGENTPLUGINS_SECURITY_ORIGIN", "https://mirror.example/security/")
	client, err = newSecurityClient(t.TempDir())
	if err != nil || client.Origin != "https://mirror.example/security/" {
		t.Fatalf("custom Security Index origin = %q err=%v", client.Origin, err)
	}
	for _, value := range []string{"http://mirror.example/security/", "https://mirror.example/security", "https://mirror.example/security/../trust/"} {
		if _, err := productionFeedOrigin(value, defaultSecurityOrigin, "AGENTPLUGINS_SECURITY_ORIGIN"); err == nil {
			t.Fatalf("unsafe Security Index origin accepted: %q", value)
		}
	}
	_ = securityv1.SchemaVersion
}

func TestTestOnlyDependencyInjectionPreservesExactLaunchConformance(t *testing.T) {
	clearDirectoryEnvironment(t)
	root := t.TempDir()
	fixture := writeConformanceFixture(t, root)
	client, err := newConformanceDirectoryClient(fixture)
	if err != nil {
		t.Fatal(err)
	}
	client.Now = func() time.Time { return time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC) }
	bundle, err := client.Load(context.Background(), 0)
	if err != nil || bundle.Source != directoryv1.BundleSourceRemote || bundle.Snapshot.Sequence != 41 {
		t.Fatalf("verified conformance fixture load = sequence %d source %q err %v", bundle.Snapshot.Sequence, bundle.Source, err)
	}
	if _, err := os.Stat(fixture["AGENTPLUGINS_DIRECTORY_CACHE"]); err != nil {
		t.Fatalf("verified fixture was not reconciled to its test-only cache: %v", err)
	}

	originalFactory, originalArgs := directoryClientFactory, os.Args
	directoryClientFactory = func(string) (*directoryv1.Client, error) { return client, nil }
	os.Args = []string{"agentplugins", "version"}
	t.Cleanup(func() { directoryClientFactory, os.Args = originalFactory, originalArgs })
	if err := run(); err != nil {
		t.Fatalf("exact CLI launch with test-only injected Directory client: %v", err)
	}
}

func TestOrdinaryDirectoryOriginPreservesProductionTrustAndBootstrap(t *testing.T) {
	clearDirectoryEnvironment(t)
	t.Setenv("AGENTPLUGINS_DIRECTORY_ORIGIN", "https://mirror.example/registry/schemas/1/")
	client, err := newDirectoryClient(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if client.Origin != "https://mirror.example/registry/schemas/1/" || len(client.Trust.Keys) != 1 || client.Trust.Keys[0].ID != defaultDirectoryKeyID || !client.RequireEmbeddedBootstrap {
		t.Fatalf("ordinary origin changed production configuration: origin=%q keys=%d require_bootstrap=%v", client.Origin, len(client.Trust.Keys), client.RequireEmbeddedBootstrap)
	}
	bundle, err := client.Embedded.Verify(client.Trust)
	if err != nil || bundle.Snapshot.Sequence != 2 || bundle.Digest != "sha256:fe6422853423f447d797a54c5c2af0b0eda6f89c23815f8945f5b6f48d50a460" {
		t.Fatalf("ordinary origin changed production bootstrap identity: sequence=%d digest=%q err=%v", bundle.Snapshot.Sequence, bundle.Digest, err)
	}
}

func TestProductionDiscoveryUsesIndependentOriginAndCacheWithPinnedTrust(t *testing.T) {
	clearDirectoryEnvironment(t)
	root := t.TempDir()
	client, err := newDiscoveryClient(root)
	if err != nil {
		t.Fatal(err)
	}
	if client.Origin != defaultDiscoveryOrigin || client.Cache.Path != filepath.Join(root, "discovery-v1-cache.json") ||
		len(client.Trust.Keys) != 1 || client.Trust.Keys[0].ID != defaultDiscoveryKeyID {
		t.Fatalf("production Discovery configuration = %+v", client)
	}
	t.Setenv("AGENTPLUGINS_DISCOVERY_ORIGIN", "https://mirror.example/discovery/")
	client, err = newDiscoveryClient(root)
	if err != nil || client.Origin != "https://mirror.example/discovery/" {
		t.Fatalf("custom Discovery origin = %q err=%v", client.Origin, err)
	}
	for _, value := range []string{"http://mirror.example/discovery/", "https://mirror.example/discovery", "https://mirror.example/discovery/../trust/"} {
		if _, err := productionFeedOrigin(value, defaultDiscoveryOrigin, "AGENTPLUGINS_DISCOVERY_ORIGIN"); err == nil {
			t.Fatalf("unsafe Discovery origin accepted: %q", value)
		}
	}
}

func TestMalformedDirectoryEnvironmentCausesZeroMutation(t *testing.T) {
	clearDirectoryEnvironment(t)
	t.Setenv("HOME", t.TempDir())
	dataRoot := filepath.Join(t.TempDir(), "must-not-exist")
	t.Setenv("AGENTPLUGINS_HOME", dataRoot)
	originalArgs := os.Args
	os.Args = []string{"agentplugins", "version"}
	t.Cleanup(func() { os.Args = originalArgs })
	for _, name := range forbiddenProductionDirectoryVariables {
		t.Setenv(name, "malformed-caller-value")
	}
	if err := run(); err != nil {
		t.Fatalf("production binary interpreted forbidden conformance variables: %v", err)
	}
	if _, err := os.Lstat(dataRoot); !os.IsNotExist(err) {
		t.Fatalf("ignored malformed conformance environment mutated agentplugins home: %v", err)
	}
	t.Setenv("AGENTPLUGINS_DIRECTORY_ORIGIN", "https://caller.example/registry/../trust/")
	if err := run(); err == nil || !strings.Contains(err.Error(), "AGENTPLUGINS_DIRECTORY_ORIGIN") {
		t.Fatalf("malformed production origin error = %v", err)
	}
	if _, err := os.Lstat(dataRoot); !os.IsNotExist(err) {
		t.Fatalf("malformed production origin mutated agentplugins home: %v", err)
	}
}

func TestProductionDirectoryOriginValidation(t *testing.T) {
	for _, value := range []string{
		"http://mirror.example/registry/", "https://user@mirror.example/registry/", "https://mirror.example/registry",
		"https://mirror.example/registry/../trust/", "https://mirror.example/registry/%2e%2e/trust/", "https://mirror.example/registry/?source=caller", "//mirror.example/registry/",
	} {
		t.Run(value, func(t *testing.T) {
			if _, err := productionDirectoryOrigin(value); err == nil {
				t.Fatalf("unsafe Directory origin accepted: %q", value)
			}
		})
	}
}

func clearDirectoryEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range append(append([]string{}, forbiddenProductionDirectoryVariables...), "AGENTPLUGINS_DIRECTORY_ORIGIN", "AGENTPLUGINS_DISCOVERY_ORIGIN", "AGENTPLUGINS_SECURITY_ORIGIN") {
		value, existed := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		restoreName, restoreValue, restoreExisted := name, value, existed
		t.Cleanup(func() {
			if restoreExisted {
				_ = os.Setenv(restoreName, restoreValue)
			} else {
				_ = os.Unsetenv(restoreName)
			}
		})
	}
}

func fixtureValue(name string, fixture map[string]string) string {
	if value := fixture[name]; value != "" {
		return value
	}
	return "malformed-caller-value"
}

func newConformanceDirectoryClient(configuration map[string]string) (*directoryv1.Client, error) {
	bundle, trust, err := bootstrapio.LoadVerifiedBundle(configuration["AGENTPLUGINS_DIRECTORY_SNAPSHOT"], configuration["AGENTPLUGINS_DIRECTORY_ENVELOPE"], configuration["AGENTPLUGINS_DIRECTORY_TRUST"])
	if err != nil {
		return nil, fmt.Errorf("load test-only Directory conformance tuple: %w", err)
	}
	return &directoryv1.Client{
		Origin: configuration["AGENTPLUGINS_DIRECTORY_ORIGIN"],
		HTTPClient: &http.Client{Timeout: 20 * time.Second, Transport: conformanceFixtureTransport{
			sequence: bundle.Snapshot.Sequence, snapshot: bundle.SnapshotBytes, envelope: bundle.EnvelopeBytes,
		}},
		Trust: trust, Embedded: directoryv1.EmbeddedBundle{Snapshot: bundle.SnapshotBytes, Envelope: bundle.EnvelopeBytes},
		RequireEmbeddedBootstrap: true, Cache: directoryv1.Cache{Path: configuration["AGENTPLUGINS_DIRECTORY_CACHE"]},
	}, nil
}

type conformanceFixtureTransport struct {
	sequence           uint64
	snapshot, envelope []byte
}

func (transport conformanceFixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	stem := fmt.Sprintf("%020d", transport.sequence)
	pointer, err := json.Marshal(directoryv1.Pointer{
		PointerSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: transport.sequence,
		SnapshotPath: "snapshots/" + stem + ".json", EnvelopePath: "snapshots/" + stem + ".envelope.json",
		FetchContract: directoryv1.FetchContract{HTTPSRequired: true, SameOriginRedirectsOnly: true, MaxRedirects: 2, LatestMaxBytes: directoryv1.MaxLatestBytes, SnapshotMaxBytes: directoryv1.MaxSnapshotBytes, EnvelopeMaxBytes: directoryv1.MaxEnvelopeBytes, RetryAttempts: 1},
	})
	if err != nil {
		return nil, err
	}
	status, body := http.StatusOK, pointer
	switch {
	case request.Method != http.MethodGet:
		status, body = http.StatusMethodNotAllowed, nil
	case strings.HasSuffix(request.URL.Path, "/latest.json"):
	case strings.HasSuffix(request.URL.Path, "/snapshots/"+stem+".json"):
		body = transport.snapshot
	case strings.HasSuffix(request.URL.Path, "/snapshots/"+stem+".envelope.json"):
		body = transport.envelope
	default:
		status, body = http.StatusNotFound, nil
	}
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(body))), Request: request}, nil
}

func writeConformanceFixture(t *testing.T, root string) map[string]string {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = 17
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	release := domain.DirectoryRelease{Sequence: 1, PackageVersion: "1.0.0", ManifestName: "tool", AgentPluginsSchema: "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", PackageSource: domain.DirectorySource{Repository: "owner/repo", Revision: "0123456789012345678901234567890123456789", Path: "plugin"}, TreeDigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ManifestDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Components: []string{"mcp"}, PublishedAt: "2026-08-20T10:00:00Z"}
	policy := domain.DirectoryReleasePolicy{ReleaseSequence: 1, Status: domain.ReleaseActive, MinimumInstallerVersion: "1.0.0", Targets: []domain.DirectoryTarget{{Client: domain.ClientCursor, Scopes: []domain.InstallScope{domain.ScopeUser}, Delivery: "managed", Authentication: domain.AuthenticationRequirementNotRequired}}, CurrentEvidence: []string{}}
	product := domain.DirectoryProduct{SchemaVersion: 1, ID: "tool", DisplayName: "Tool", Description: "Tool description", ManifestName: "tool", Aliases: []string{"tool"}, ReservedAliases: []string{"tool"}, Categories: []string{"tools"}, MinimumCapabilities: domain.DirectoryMinimumCapabilities{Skills: "optional", MCP: "required"}, DefaultDistribution: "owner/tool", Distributions: []string{"owner/tool"}}
	distribution := domain.DirectoryDistribution{SchemaVersion: 1, ID: "owner/tool", ProductID: "tool", Kind: domain.DistributionUpstream, Status: domain.DistributionActive, Packager: "owner", Releases: []domain.DirectoryRelease{release}, ReleasePolicies: []domain.DirectoryReleasePolicy{policy}}
	snapshot := domain.DirectorySnapshot{SnapshotSchemaVersion: 1, Sequence: 41, PublicationID: "launch-conformance-41", SourceCommit: "abcdefabcdefabcdefabcdefabcdefabcdefabcd", GeneratedAt: "2026-08-20T11:00:00Z", ExpiresAt: "2026-09-19T11:00:00Z", Products: []domain.DirectoryProduct{product}, Distributions: []domain.DirectoryDistribution{distribution}, Evidence: []domain.DirectoryEvidence{}, Revocations: []domain.DirectoryRevocation{}}
	snapshotBytes, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshotBytes = append(snapshotBytes, '\n')
	digest := sha256.Sum256(snapshotBytes)
	signature := ed25519.Sign(privateKey, directoryv1.SnapshotSignatureMessage(snapshotBytes))
	envelope := directoryv1.Envelope{EnvelopeSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: 41, KeyID: "launch-conformance-test", Algorithm: "Ed25519", SignatureDomain: directoryv1.SignatureDomain, SnapshotDigest: "sha256:" + hex.EncodeToString(digest[:]), Signature: base64.StdEncoding.EncodeToString(signature)}
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{
		"AGENTPLUGINS_DIRECTORY_SNAPSHOT":         filepath.Join(root, "snapshot.json"),
		"AGENTPLUGINS_DIRECTORY_ENVELOPE":         filepath.Join(root, "envelope.json"),
		"AGENTPLUGINS_DIRECTORY_TRUST":            filepath.Join(root, "trusted-keys.json"),
		"AGENTPLUGINS_DIRECTORY_CACHE":            filepath.Join(root, "directory-cache"),
		"AGENTPLUGINS_DIRECTORY_ORIGIN":           "https://conformance.invalid/registry/schemas/1/",
		"AGENTPLUGINS_DIRECTORY_CONFORMANCE_ONLY": "1",
	}
	trustBytes := []byte(fmt.Sprintf(`{"schema_version":1,"keys":[{"key_id":"launch-conformance-test","public_key":%q}]}`, base64.StdEncoding.EncodeToString(publicKey)))
	for path, body := range map[string][]byte{paths["AGENTPLUGINS_DIRECTORY_SNAPSHOT"]: snapshotBytes, paths["AGENTPLUGINS_DIRECTORY_ENVELOPE"]: envelopeBytes, paths["AGENTPLUGINS_DIRECTORY_TRUST"]: trustBytes} {
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}
