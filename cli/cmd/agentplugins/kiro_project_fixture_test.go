package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type kiroProjectDetector struct{ root string }

func (detector kiroProjectDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientKiro, ConfigRoot: detector.root, Status: domain.DetectionDetected}}, nil
}

// Reuse the core journey's production CLI/state helpers and real TLS Git/ZIP
// transports. Kiro supplies its own detected profile and native layout.
type kiroProjectJourney struct {
	projectJourney
	protected map[string]kiroProtectedTree
}

type kiroProtectedTree struct {
	entries    map[string]string
	identities map[string]os.FileInfo
}

func newKiroProjectJourney(t *testing.T, transport *projectJourneyTransport) kiroProjectJourney {
	t.Helper()
	f := kiroProjectJourney{projectJourney: projectJourney{home: projectJourneyTemp(t), a: projectJourneyTemp(t), b: projectJourneyTemp(t), transport: transport}, protected: map[string]kiroProtectedTree{}}
	f.cache = filepath.Join(f.home, "uap")
	for index, root := range []string{"", f.a, f.b} {
		native := f.kiroRoot(root)
		projectJourneyWrite(t, filepath.Join(native, "settings", "mcp.json"), `{"mcpServers":{"foreign":{"url":"https://foreign.test/mcp","disabled":true}},"foreignPreference":"keep"}`)
		projectJourneyWrite(t, filepath.Join(native, "skills", "foreign", "SKILL.md"), "foreign skill\n")
		projectJourneyWrite(t, filepath.Join(native, "hooks.json"), "foreign hooks\n")
		label := []string{"user", "project-a", "project-b"}[index]
		projectJourneyWrite(t, filepath.Join(native, "agents", "preserved.json"), `{"name":"preserved","includeMcpJson":false,"resources":["file://`+label+`.md"],"mcpServers":{"docs":{"url":"https://`+label+`.invalid/mcp","disabled":true}}}`)
	}
	user := f.kiroRoot("")
	projectJourneyWrite(t, filepath.Join(user, "settings", "mcp.json"), `{"mcpServers":{"docs":{"url":"https://user-peer.invalid/mcp","disabled":true},"foreign":{"url":"https://foreign.test/mcp","disabled":true}},"foreignPreference":"keep"}`)
	projectJourneyWrite(t, filepath.Join(user, "skills", "docs", "SKILL.md"), "Original same-name User skill.\n")
	f.preserve(t, user, filepath.Join(f.kiroRoot(f.a), "agents"), filepath.Join(f.kiroRoot(f.b), "agents"))
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(f.home, f.cache, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	f.app = configureKiroProjectJourney(app, f.home, transport)
	return f
}

// These are preserved opaque policy files, never evidence of active agent selection.
func (f kiroProjectJourney) preserve(t *testing.T, roots ...string) {
	t.Helper()
	f.assertProtected(t)
	for _, root := range roots {
		if _, exists := f.protected[root]; exists {
			t.Fatal("cannot rebaseline original Kiro policy or peer")
		}
		snapshot := kiroProtectedTree{entries: securityFixtureTree(t, root), identities: map[string]os.FileInfo{}}
		for relative := range snapshot.entries {
			held, err := os.Open(filepath.Join(root, relative))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := held.Close(); err != nil {
					t.Error(err)
				}
			})
			original, err := held.Stat()
			if err != nil {
				t.Fatal(err)
			}
			snapshot.identities[relative] = original
		}
		f.protected[root] = snapshot
	}
	f.assertProtected(t)
}

func (f kiroProjectJourney) assertProtected(t *testing.T) {
	t.Helper()
	for root, snapshot := range f.protected {
		if !reflect.DeepEqual(snapshot.entries, securityFixtureTree(t, root)) {
			t.Fatalf("Kiro changed original policy or peer bytes/membership: %s", root)
		}
		for relative, original := range snapshot.identities {
			current, err := os.Lstat(filepath.Join(root, relative))
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(original, current) || original.Mode() != current.Mode() {
				t.Fatalf("Kiro replaced original policy or peer identity: %s", root)
			}
		}
	}
}

func (f kiroProjectJourney) run(t *testing.T, project string, args ...string) {
	t.Helper()
	f.assertProtected(t)
	f.projectJourney.run(t, project, args...)
	f.assertProtected(t)
	for _, installation := range f.state(t, project).Installations {
		for _, binding := range installation.Clients {
			if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
				t.Fatal("Kiro policy/peer fixture upgraded prepared file verification")
			}
		}
	}
}

// Preserve the production scoped composer, installer, ownership kernel, store,
// registry and solver. Only host discovery and security assessment are inert.
func configureKiroProjectJourney(app agentpluginscli.App, home string, transport *projectJourneyTransport) agentpluginscli.App {
	app.Detector = kiroProjectDetector{root: filepath.Join(home, ".kiro")}
	app.SecurityIndex, app.DirectoryClient, app.DiscoveryClient = nil, nil, nil
	app.SecurityEvaluator = projectJourneySecurity{}
	app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return projectJourneySecurity{} }
	app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: home, HTTPClient: transport.server.Client()}
	app.SourceAcquirerForDryRun = func(root string) agentpluginscli.SourceAcquirer {
		return sourceacquisition.Acquirer{TempRoot: root, HTTPClient: transport.server.Client()}
	}
	app.DirectoryClientForDryRun, app.CommitDirectoryPreview = nil, nil
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureKiroProjectJourney(selected, home, transport), nil
	}
	return app
}

func (f kiroProjectJourney) kiroRoot(project string) string {
	if project == "" {
		return filepath.Join(f.home, ".kiro")
	}
	return filepath.Join(project, ".kiro")
}

func (f kiroProjectJourney) assertKiroOwned(t *testing.T, project, skill, endpoint string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := f.owned(t, project)
	if binding.Scope != string(domain.ScopeProject) || binding.NativeProfileRoot != f.kiroRoot(project) || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("Kiro project lifecycle lost its exact scope or conservative observation: %+v", binding)
	}
	if err := pathpolicy.RequireContainedChild(filepath.Join(project, ".uap"), binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{binding.TargetLocator, f.kiroRoot(project)} {
		if projectJourneyRead(t, filepath.Join(root, "skills", "docs", "SKILL.md")) != skill {
			t.Fatal("Kiro managed/native skill bytes differ from the selected pin")
		}
	}
	settings := projectJourneyRead(t, filepath.Join(f.kiroRoot(project), "settings", "mcp.json"))
	for _, text := range []string{endpoint, "foreign.test", "foreignPreference", `"disabled":true`} {
		if !strings.Contains(strings.ReplaceAll(settings, " ", ""), text) {
			t.Fatalf("Kiro settings lost selected or foreign member %q: %s", text, settings)
		}
	}
	return installation, binding
}

// Select inert remote configuration so the composed proof never executes an
// MCP command, copies a test binary as a launcher, or starts a native client.
func kiroProjectCommitSource(t *testing.T, transport *projectJourneyTransport, skill, endpoint string) {
	t.Helper()
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), skill)
	projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"`+endpoint+`"}}}`)
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "select inert Kiro project components")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.archive = projectJourneyArchive(t, transport.repo)
}
