package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type piSignedProjectDetector struct{ home string }

func (d piSignedProjectDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{
		ClientID: domain.ClientPi, DisplayName: "Pi", Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(d.home, ".pi", "agent"),
		ExecutablePath: filepath.Join(d.home, "pi-test-host-never-launched"),
		Version: "1.5.0",
	}}, nil
}

func piSignedProjectNativeRoot(j projectJourney, root string) string {
	if root == "" {
		return filepath.Join(j.home, ".pi", "agent")
	}
	return filepath.Join(root, ".pi")
}

// This is a signed TEST-ONLY four-package fixture. The production
// Directory and user's Pi state/credentials remain untouched.
func TestPiProjectSignedV1SavedFourPackageLifecycle(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	const repository = "fixture/pi-marketplace"
	const skill = "---\nname: campaign-planning\ndescription: Disposable Pi project fixture\n---\nPi signed-project fixture.\n"
	packages := []struct {
		name, path, mcpName string
	}{
		{"marketing-kit", "kit", ""},
		{"notion", "notion", "notion"},
		{"hubspot-crm", "hubspot-crm", "hubspot"},
		{"playwright-bridge", "playwright-bridge", "playwright"},
	}
	for _, item := range packages {
		root := filepath.Join(transport.repo, item.path)
		projectJourneyWrite(t, filepath.Join(root, "plugin.json"), fmt.Sprintf(
			"{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json\",\"name\":%q,\"version\":\"0.1.0\",\"description\":\"Disposable signed Pi test\"}", item.name))
		if item.mcpName == "" {
			projectJourneyWrite(t, filepath.Join(root, "skills", "campaign-planning", "SKILL.md"), skill)
			projectJourneyWrite(t, filepath.Join(root, "uap.toml"), "schema_version = 1\n[dependencies]\n"+
				"\"fixture/notion\" = \"0.1.0\"\n"+
				"\"fixture/hubspot-crm\" = \"0.1.0\"\n"+
				"\"fixture/playwright-bridge\" = \"0.1.0\"\n")
		} else {
			mcp := ""
			if item.mcpName == "playwright" {
				mcp = fmt.Sprintf("{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json\",\"mcpServers\":{%q:{\"type\":\"stdio\",\"command\":\"go\",\"args\":[\"version\"]}}}", item.mcpName)
			} else {
				mcp = fmt.Sprintf("{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json\",\"mcpServers\":{%q:{\"type\":\"streamable-http\",\"url\":\"https://%s.example.test/mcp\"}}}", item.mcpName, item.mcpName)
			}
			projectJourneyWrite(t, filepath.Join(root, "mcp.json"), mcp)
		}
	}
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "disposable signed Pi saved closure")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	projectJourneyGit(t, transport.remote, "config", "uploadpack.allowTipSHA1InWant", "true")
	git := exec.CommandContext(t.Context(), "git", "-C", transport.repo, "rev-parse", "HEAD")
	rawCommit, err := git.Output()
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(rawCommit))
	if len(commit) != 40 {
		t.Fatalf("fixture revision is not immutable: %q", commit)
	}

	journey := projectJourney{
		home: projectJourneyTemp(t), a: projectJourneyTemp(t),
		b: projectJourneyTemp(t), transport: transport,
	}
	journey.cache = filepath.Join(journey.home, "uap")
	projectJourneyWrite(t, filepath.Join(journey.home, "pi-test-host-never-launched"), "inert Pi host fixture; never execute\n")
	for _, root := range []string{"", journey.a, journey.b} {
		native := piSignedProjectNativeRoot(journey, root)
		projectJourneyWrite(t, filepath.Join(native, "mcp.json"), "{\"mcpServers\":{\"foreign\":{\"type\":\"streamable-http\",\"url\":\"https://foreign.example.test/mcp\"}},\"keep\":true}")
		projectJourneyWrite(t, filepath.Join(native, "skills", "foreign", "SKILL.md"), "foreign Pi skill\n")
		if err := os.MkdirAll(filepath.Join(native, "extensions"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(journey.home, journey.cache, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.November, 12, 1, 0, 0, 0, time.UTC)
	snapshot := domain.DirectorySnapshot{
		SnapshotSchemaVersion: 1, Sequence: 1, PublicationID: "pi-test-only-1",
		SourceCommit: commit, GeneratedAt: now.Add(-time.Hour).Format(time.RFC3339),
		ExpiresAt: now.Add(24*time.Hour).Format(time.RFC3339),
		Products: []domain.DirectoryProduct{}, Distributions: []domain.DirectoryDistribution{},
		Evidence: []domain.DirectoryEvidence{}, Revocations: []domain.DirectoryRevocation{},
	}
	for _, item := range packages {
		captured, captureErr := (packagedigest.Builder{TempRoot: t.TempDir()}).Snapshot(
			t.Context(), filepath.Join(transport.repo, item.path), domain.SourceIdentity{})
		if captureErr != nil {
			t.Fatal(captureErr)
		}
		loaded, loadErr := app.PackageLoader.Load(t.Context(), domain.LoadInput{
			SnapshotRoot: captured.Root, TreeDigest: captured.TreeDigest,
			ExecutableFiles: captured.ExecutableFiles, Source: captured.Source,
		})
		removeErr := packagedigest.Remove(captured)
		if loadErr != nil || removeErr != nil {
			t.Fatalf("fixture source %s: %v / %v", item.name, loadErr, removeErr)
		}
		skills, mcp, components := "optional", "required", []string{"mcp"}
		if item.mcpName == "" {
			skills, mcp, components = "required", "optional", []string{"skills"}
		}
		distribution := "fixture/" + item.name
		snapshot.Products = append(snapshot.Products, domain.DirectoryProduct{
			SchemaVersion: 1, ID: item.name, DisplayName: item.name,
			Description: "Signed test-only Pi project package",
			ManifestName: item.name, Aliases: []string{item.name},
			ReservedAliases: []string{item.name}, Categories: []string{"tools"},
			MinimumCapabilities: domain.DirectoryMinimumCapabilities{Skills: skills, MCP: mcp},
			DefaultDistribution: distribution, Distributions: []string{distribution},
		})
		snapshot.Distributions = append(snapshot.Distributions, domain.DirectoryDistribution{
			SchemaVersion: 1, ID: distribution, ProductID: item.name,
			Kind: domain.DistributionCommunity, Packager: "fixture",
			Status: domain.DistributionActive,
			Releases: []domain.DirectoryRelease{{
				Sequence: 1, PackageVersion: "0.1.0", ManifestName: item.name,
				AgentPluginsSchema: domain.PluginSchemaV1,
				PackageSource: domain.DirectorySource{Repository: repository, Revision: commit, Path: item.path},
				TreeDigestAlgorithm: domain.TreeDigestAlgorithm,
				TreeDigest: loaded.TreeDigest, ManifestDigest: loaded.ManifestDigest,
				Components: components, PublishedAt: now.Add(-2*time.Hour).Format(time.RFC3339),
			}},
			ReleasePolicies: []domain.DirectoryReleasePolicy{{
				ReleaseSequence: 1, Status: domain.ReleaseActive,
				MinimumInstallerVersion: "0.1.0",
				Targets: []domain.DirectoryTarget{{
					Client: domain.ClientPi, Scopes: []domain.InstallScope{domain.ScopeUser},
					Delivery: "managed", Authentication: domain.AuthenticationRequirementUnknown,
				}}, CurrentEvidence: []string{},
			}},
		})
	}
	snapshotBytes, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{84}, ed25519.SeedSize))
	const keyID = "pi-test-project-only"
	trust := directoryv1.TrustStore{Keys: []directoryv1.TrustedKey{{
		ID: keyID, PublicKey: private.Public().(ed25519.PublicKey), State: directoryv1.KeyCurrent,
	}}}
	sum := sha256.Sum256(snapshotBytes)
	envelopeBytes, err := json.Marshal(directoryv1.Envelope{
		EnvelopeSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: 1,
		KeyID: keyID, Algorithm: "Ed25519", SignatureDomain: directoryv1.SignatureDomain,
		SnapshotDigest: "sha256:" + hex.EncodeToString(sum[:]),
		Signature: base64.StdEncoding.EncodeToString(
			ed25519.Sign(private, directoryv1.SnapshotSignatureMessage(snapshotBytes))),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := directoryv1.VerifyBundle(snapshotBytes, envelopeBytes, trust); err != nil {
		t.Fatalf("test-only Pi signed v1 fixture invalid: %v", err)
	}
	ca := os.Getenv("GIT_SSL_CAINFO")
	if ca == "" {
		t.Fatal("missing disposable Git fixture trust root")
	}
	client := &directoryv1.Client{
		Origin: "https://fixture.invalid/registry/pi-test/schemas/1/",
		FeedID: "pi-project-test-only",
		HTTPClient: &http.Client{Transport: conformanceFixtureTransport{
			sequence: 1, snapshot: snapshotBytes, envelope: envelopeBytes,
		}},
		Trust: trust, Embedded: directoryv1.EmbeddedBundle{Snapshot: snapshotBytes, Envelope: envelopeBytes},
		RequireEmbeddedBootstrap: true,
		Cache: directoryv1.Cache{Path: filepath.Join(journey.cache, "pi-project-test-cache.json")},
		Now: func() time.Time { return now },
	}
	acquire := func(root string) agentpluginscli.SourceAcquirer {
		return sourceacquisition.Acquirer{
			TempRoot: root, HTTPClient: transport.server.Client(),
			Runner: signedV1ProjectGitRunner{
				delegate: sourceacquisition.OSRunner{}, caFile: ca,
			},
			URLForRepo: func(value string) string {
				if value == repository {
					return transport.server.URL + "/repo.git"
				}
				return "https://invalid.example.test/blocked"
			},
		}
	}
	var configure func(agentpluginscli.App) agentpluginscli.App
	configure = func(app agentpluginscli.App) agentpluginscli.App {
		app.Version = "1.18.34"
		app.Detector = piSignedProjectDetector{home: journey.home}
		app.DirectoryClient = client
		app.SourceAcquirer = acquire(journey.home)
		app.SourceAcquirerForDryRun = acquire
		app.DirectoryClientForDryRun = func(string) (agentpluginscli.DirectoryClient, error) { return client, nil }
		app.SecurityIndex, app.DiscoveryClient = nil, nil
		app.SecurityEvaluator = projectJourneySecurity{}
		app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return projectJourneySecurity{} }
		parent := app.ForScope
		app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
			selected, err := parent(scope, root)
			if err != nil {
				return selected, err
			}
			return configure(selected), nil
		}
		return app
	}
	journey.app = configure(app)
	originalForeign := projectJourneyRead(t, filepath.Join(piSignedProjectNativeRoot(journey, journey.b), "mcp.json"))
	for _, root := range []string{"", journey.a} {
		journey.run(t, root, "install", "marketing-kit", "--target", "pi", "--yes")
		state := journey.state(t, root)
		if len(state.Installations) != 4 || len(state.EnvironmentReceipts) != 4 {
			t.Fatalf("Pi signed test-only saved closure in %q omitted packages: %+v", root, state)
		}
		for _, installed := range state.Installations {
			if installed.Directory == nil || installed.Directory.FeedID != client.FeedID {
				t.Fatalf("Pi installation lost signed fixture lineage in %q: %+v", root, installed.Directory)
			}
		}
		native := piSignedProjectNativeRoot(journey, root)
		if got := projectJourneyRead(t, filepath.Join(native, "skills", "campaign-planning", "SKILL.md")); got != skill {
			t.Fatalf("Pi native Skill in %q differs from the signed source: %q", root, got)
		}
		var config struct {
			MCPServers map[string]json.RawMessage `json:"mcpServers"`
			Keep bool `json:"keep"`
		}
		if err := json.Unmarshal([]byte(projectJourneyRead(t, filepath.Join(native, "mcp.json"))), &config); err != nil {
			t.Fatal(err)
		}
		if !config.Keep || len(config.MCPServers) != 4 {
			t.Fatalf("Pi signed test fixture lost an MCP child or foreign ownership in %q: %+v", root, config)
		}
		journey.run(t, root, "deps", "sync", "--locked", "--yes")
		if got := len(journey.state(t, root).EnvironmentReceipts); got != 4 {
			t.Fatalf("replay duplicated or removed Pi receipts in %q: %d", root, got)
		}
	}
	if got := projectJourneyRead(t, filepath.Join(piSignedProjectNativeRoot(journey, journey.b), "mcp.json")); got != originalForeign {
		t.Fatal("Project A installation changed unselected Project B Pi native config")
	}
	journey.run(t, journey.a, "remove", "marketing-kit", "--target", "pi", "--yes")
	if got := len(journey.state(t, journey.a).EnvironmentReceipts); got != 0 {
		t.Fatalf("Project Pi removal left signed test-only owned graph: %d", got)
	}
	if got := len(journey.state(t, "").EnvironmentReceipts); got != 4 {
		t.Fatalf("Project Pi removal changed User receipts: %d", got)
	}
	if got := projectJourneyRead(t, filepath.Join(piSignedProjectNativeRoot(journey, journey.b), "mcp.json")); got != originalForeign {
		t.Fatal("Project Pi removal changed Project B foreign native configuration")
	}
}
