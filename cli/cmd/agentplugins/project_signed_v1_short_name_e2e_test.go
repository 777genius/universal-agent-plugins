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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// signedV1ProjectGitRunner trusts only the disposable fixture CA for the
// disposable Git repository. The production runner deliberately removes Git
// environment overrides; keep its TLS verification and transport hardening.
type signedV1ProjectGitRunner struct {
	delegate sourceacquisition.Runner
	caFile   string
}

func (runner signedV1ProjectGitRunner) Run(ctx context.Context, command sourceacquisition.Command) ([]byte, error) {
	if len(command.Args) > 2 && command.Args[0] == "-C" && command.Args[2] == "fetch" {
		_, err := (sourceacquisition.OSRunner{}).Run(ctx, sourceacquisition.Command{
			Dir: command.Dir, Args: []string{"-C", command.Args[1], "config", "http.sslCAInfo", runner.caFile},
		})
		if err != nil {
			return nil, err
		}
	}
	return runner.delegate.Run(ctx, command)
}

// Real v1 Ed25519 reader, signed short names, Git acquisition, Cobra, receipts,
// native Gemini fixture and project isolation. No model-driven MCP tools/call
// or production Marketing Kit qualification is implied by this test.
func TestProjectSignedV1ShortNameMarketingClosureAndColdRestore(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	caFile := os.Getenv("GIT_SSL_CAINFO")
	if caFile == "" {
		t.Fatal("disposable signed-v1 Git fixture CA is missing")
	}
	const repository = "fixture/marketplace"
	paths := []struct {
		name, distribution, path, server string
	}{
		{"marketing-kit", "fixture/marketing-kit", "kit", ""},
		{"notion", "fixture/notion", "notion", "notion"},
		{"hubspot-crm", "fixture/hubspot-crm", "hubspot-crm", "hubspot"},
		{"playwright-bridge", "fixture/playwright-bridge", "playwright-bridge", "playwright"},
	}
	for _, item := range paths {
		root := filepath.Join(transport.repo, item.path)
		projectJourneyWrite(t, filepath.Join(root, "plugin.json"), fmt.Sprintf(
			"{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json\",\"name\":%q,\"version\":\"0.1.0\"}", item.name))
		if item.server == "" {
			projectJourneyWrite(t, filepath.Join(root, "skills", "campaign-planning", "SKILL.md"),
				"---\nname: campaign-planning\ndescription: Test-only campaign plan\n---\nLocal test fixture\n")
			projectJourneyWrite(t, filepath.Join(root, "uap.toml"), "schema_version = 1\n[dependencies]\n"+
				"\"fixture/notion\" = \"0.1.0\"\n"+
				"\"fixture/hubspot-crm\" = \"0.1.0\"\n"+
				"\"fixture/playwright-bridge\" = \"0.1.0\"\n")
		} else {
			var mcp string
			if item.server == "playwright" {
				mcp = fmt.Sprintf("{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json\",\"mcpServers\":{%q:{\"type\":\"stdio\",\"command\":\"go\",\"args\":[\"version\"]}}}", item.server)
			} else {
				mcp = fmt.Sprintf("{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json\",\"mcpServers\":{%q:{\"type\":\"streamable-http\",\"url\":%q}}}", item.server, "https://"+item.server+".example.test/mcp")
			}
			projectJourneyWrite(t, filepath.Join(root, "mcp.json"), mcp)
		}
	}
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "signed v1 scoped closure fixture")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	// The signed Directory pins a raw commit SHA rather than a branch name.
	// Match the published Git transport: permit fetching an advertised tip by SHA
	// from this disposable smart-HTTP bare repository (Git defaults to denying it).
	projectJourneyGit(t, transport.remote, "config", "uploadpack.allowTipSHA1InWant", "true")
	gitSHA := exec.CommandContext(t.Context(), "git", "-C", transport.repo, "rev-parse", "HEAD")
	commitBytes, err := gitSHA.Output()
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(commitBytes))
	if len(commit) != 40 {
		t.Fatalf("fixture is not pinned to a Git SHA: %q", commit)
	}

	journey := newProjectJourney(t, transport)
	now := time.Date(2026, time.November, 5, 20, 0, 0, 0, time.UTC)
	delivery, ok := domain.ExpectedDirectoryDelivery(domain.ClientGemini)
	if !ok {
		t.Fatal("Gemini has no existing Directory delivery")
	}
	snapshot := domain.DirectorySnapshot{
		SnapshotSchemaVersion: 1, Sequence: 44, PublicationID: "project-v1-44",
		SourceCommit: strings.Repeat("a", 40),
		GeneratedAt:  now.Add(-time.Hour).Format(time.RFC3339),
		ExpiresAt:    now.Add(24 * time.Hour).Format(time.RFC3339),
		Products:     []domain.DirectoryProduct{}, Distributions: []domain.DirectoryDistribution{},
		Evidence: []domain.DirectoryEvidence{}, Revocations: []domain.DirectoryRevocation{},
	}
	for _, item := range paths {
		captured, err := (packagedigest.Builder{TempRoot: t.TempDir()}).Snapshot(t.Context(),
			filepath.Join(transport.repo, item.path), domain.SourceIdentity{})
		if err != nil {
			t.Fatal(err)
		}
		envelope, loadErr := journey.app.PackageLoader.Load(t.Context(), domain.LoadInput{
			SnapshotRoot: captured.Root, TreeDigest: captured.TreeDigest,
			ExecutableFiles: captured.ExecutableFiles, Source: captured.Source,
		})
		removeErr := packagedigest.Remove(captured)
		if loadErr != nil || removeErr != nil {
			t.Fatalf("load or remove signed fixture source %s: %v %v", item.name, loadErr, removeErr)
		}
		skills, mcp := "optional", "required"
		components := []string{"mcp"}
		if item.server == "" {
			skills, mcp, components = "required", "optional", []string{"skills"}
		}
		snapshot.Products = append(snapshot.Products, domain.DirectoryProduct{
			SchemaVersion: 1, ID: item.name, DisplayName: item.name,
			Description: "Disposable signed fixture " + item.name, ManifestName: item.name,
			Aliases: []string{item.name}, ReservedAliases: []string{item.name},
			Categories:          []string{"agent-plugins"},
			MinimumCapabilities: domain.DirectoryMinimumCapabilities{Skills: skills, MCP: mcp},
			DefaultDistribution: item.distribution, Distributions: []string{item.distribution},
		})
		snapshot.Distributions = append(snapshot.Distributions, domain.DirectoryDistribution{
			SchemaVersion: 1, ID: item.distribution, ProductID: item.name,
			Kind: domain.DistributionCommunity, Status: domain.DistributionActive, Packager: "fixture",
			Releases: []domain.DirectoryRelease{{
				Sequence: 1, PackageVersion: "0.1.0", ManifestName: item.name,
				AgentPluginsSchema:  "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
				PackageSource:       domain.DirectorySource{Repository: repository, Revision: commit, Path: item.path},
				TreeDigestAlgorithm: domain.TreeDigestAlgorithm,
				TreeDigest:          envelope.TreeDigest, ManifestDigest: envelope.ManifestDigest,
				Components: components, PublishedAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
			}},
			ReleasePolicies: []domain.DirectoryReleasePolicy{{
				ReleaseSequence: 1, Status: domain.ReleaseActive, MinimumInstallerVersion: "0.1.0",
				Targets: []domain.DirectoryTarget{{
					Client: domain.ClientGemini, Scopes: []domain.InstallScope{domain.ScopeUser},
					Delivery: delivery, Authentication: domain.AuthenticationRequirementUnknown,
				}},
				CurrentEvidence: []string{},
			}},
		})
	}
	snapshotBytes, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshotBytes = append(snapshotBytes, '\n')
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{23}, ed25519.SeedSize))
	trust := directoryv1.TrustStore{Keys: []directoryv1.TrustedKey{{
		ID: "fixture-project-v1", PublicKey: key.Public().(ed25519.PublicKey), State: directoryv1.KeyCurrent,
	}}}
	digest := sha256.Sum256(snapshotBytes)
	signed := ed25519.Sign(key, directoryv1.SnapshotSignatureMessage(snapshotBytes))
	signedEnvelope := directoryv1.Envelope{
		EnvelopeSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: snapshot.Sequence,
		KeyID: trust.Keys[0].ID, Algorithm: "Ed25519", SignatureDomain: directoryv1.SignatureDomain,
		SnapshotDigest: "sha256:" + hex.EncodeToString(digest[:]),
		Signature:      base64.StdEncoding.EncodeToString(signed),
	}
	envelopeBytes, err := json.Marshal(signedEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := directoryv1.VerifyBundle(snapshotBytes, envelopeBytes, trust); err != nil {
		t.Fatalf("signed fixture was not authenticated by production verifier: %v", err)
	}
	tampered := append([]byte(nil), snapshotBytes...)
	tampered[0] ^= 1
	if _, err := directoryv1.VerifyBundle(tampered, envelopeBytes, trust); err == nil {
		t.Fatal("tampered signed Directory was accepted")
	}

	configure := func(j projectJourney) projectJourney {
		client := &directoryv1.Client{
			Origin: "https://fixture.invalid/registry/schemas/1/",
			HTTPClient: &http.Client{Transport: conformanceFixtureTransport{
				sequence: snapshot.Sequence, snapshot: snapshotBytes, envelope: envelopeBytes,
			}},
			Trust: trust, Embedded: directoryv1.EmbeddedBundle{Snapshot: snapshotBytes, Envelope: envelopeBytes},
			RequireEmbeddedBootstrap: true, Cache: directoryv1.Cache{Path: filepath.Join(j.cache, "directory-v1-cache.json")},
			Now: func() time.Time { return now },
		}
		source := func(temp string) agentpluginscli.SourceAcquirer {
			return sourceacquisition.Acquirer{TempRoot: temp, HTTPClient: transport.server.Client(),
				Runner: signedV1ProjectGitRunner{delegate: projectGitDiagnosticRunner{recorder: transport.diagnostic, root: temp}, caFile: caFile},
				URLForRepo: func(repo string) string {
					if repo == repository {
						return transport.endpoint
					}
					return "https://invalid.example.test/refuse-unknown-repository"
				},
			}
		}
		wire := func(app agentpluginscli.App) agentpluginscli.App {
			app.Version = "1.18.34"
			app.DirectoryClient = client
			app.SourceAcquirer = source(j.home)
			app.SourceAcquirerForDryRun = source
			app.DirectoryClientForDryRun = func(string) (agentpluginscli.DirectoryClient, error) { return client, nil }
			return app
		}
		j.app = wire(j.app)
		parent := j.app.ForScope
		j.app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
			selected, err := parent(scope, root)
			if err != nil {
				return selected, err
			}
			return wire(selected), nil
		}
		return j
	}
	journey = configure(journey)
	for _, root := range []string{"", journey.a, journey.b} {
		journey.run(t, root, "install", "marketing-kit", "--target", "gemini")
		state := journey.state(t, root)
		if len(state.Installations) != 4 || len(state.EnvironmentReceipts) != 4 {
			t.Fatalf("signed short-name install in %q omitted mandatory MCP children: %+v", root, state)
		}
		for _, installation := range state.Installations {
			if installation.Directory == nil || installation.Directory.SnapshotSchema != 1 || installation.Directory.SnapshotSequence != 44 {
				t.Fatalf("installation lost actual signed v1 authority: %+v", installation.Directory)
			}
		}
		native := projectJourneyRead(t, filepath.Join(journey.nativeRoot(root), "settings.json"))
		for _, server := range []string{"notion", "hubspot", "playwright", "foreign.test"} {
			if !strings.Contains(native, server) {
				t.Fatalf("%q native settings lost %s: %s", root, server, native)
			}
		}
		journey.run(t, root, "install", "marketing-kit", "--target", "gemini")
		if len(journey.state(t, root).EnvironmentReceipts) != 4 {
			t.Fatal("replay duplicated signed Directory installation")
		}
	}
	fresh := configure(newProjectJourney(t, transport))
	for _, name := range []string{"uap.toml", "uap.lock"} {
		projectJourneyWrite(t, filepath.Join(fresh.a, name),
			projectJourneyRead(t, filepath.Join(journey.a, name)))
	}
	lockBefore := projectJourneyRead(t, filepath.Join(fresh.a, "uap.lock"))
	fresh.run(t, fresh.a, "deps", "sync", "--locked", "--yes")
	if got := len(fresh.state(t, fresh.a).EnvironmentReceipts); got != 4 {
		t.Fatalf("cold signed v1 Project lock restore omitted children: %d", got)
	}
	if got := projectJourneyRead(t, filepath.Join(fresh.a, "uap.lock")); got != lockBefore {
		t.Fatal("cold locked restore rewrote the exact signed dependency lock")
	}
	coldNative := projectJourneyRead(t, filepath.Join(fresh.nativeRoot(fresh.a), "settings.json"))
	for _, server := range []string{"notion", "hubspot", "playwright", "foreign.test"} {
		if !strings.Contains(coldNative, server) {
			t.Fatalf("cold signed v1 restore omitted native MCP server %s: %s", server, coldNative)
		}
	}
	userStateBefore := projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json"))
	bStateBefore := projectJourneyRead(t, filepath.Join(journey.namespace(journey.b), "state-v2.json"))
	userNativeBefore := projectJourneyRead(t, filepath.Join(journey.nativeRoot(""), "settings.json"))
	bNativeBefore := projectJourneyRead(t, filepath.Join(journey.nativeRoot(journey.b), "settings.json"))
	journey.run(t, journey.a, "remove", "marketing-kit", "--target", "gemini")
	if got := len(journey.state(t, journey.a).EnvironmentReceipts); got != 0 {
		t.Fatalf("Project A still owns signed root/children after removal: %d", got)
	}
	if got := len(journey.state(t, "").EnvironmentReceipts); got != len(userBefore.EnvironmentReceipts) {
		t.Fatal("Project A removal changed user installation")
	}
	if got := projectJourneyRead(t, filepath.Join(journey.cache, "state-v2.json")); got != userStateBefore {
		t.Fatal("Project A removal changed user state bytes")
	}
	if got := projectJourneyRead(t, filepath.Join(journey.namespace(journey.b), "state-v2.json")); got != bStateBefore {
		t.Fatal("Project A removal changed Project B state bytes")
	}
	if got := projectJourneyRead(t, filepath.Join(journey.nativeRoot(""), "settings.json")); got != userNativeBefore {
		t.Fatal("Project A removal changed user native settings")
	}
	if got := projectJourneyRead(t, filepath.Join(journey.nativeRoot(journey.b), "settings.json")); got != bNativeBefore {
		t.Fatal("Project A removal changed Project B native settings")
	}
	removedNative := projectJourneyRead(t, filepath.Join(journey.nativeRoot(journey.a), "settings.json"))
	if !strings.Contains(removedNative, "foreign.test") || !strings.Contains(removedNative, "retained foreign settings") {
		t.Fatalf("signed Project removal damaged foreign native settings: %s", removedNative)
	}
	for _, server := range []string{"notion", "hubspot", "playwright"} {
		if strings.Contains(removedNative, "\"" + server + "\"") {
			t.Fatalf("signed Project removal left owned MCP server %s: %s", server, removedNative)
		}
	}
	if got := projectJourneyRead(t, filepath.Join(journey.nativeRoot(journey.a), "skills", "foreign", "SKILL.md")); got != "foreign skill\n" {
		t.Fatal("signed Project removal changed a foreign native Skill")
	}
}
