package main

import (
	"context"
	"path/filepath"
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
type kiroProjectJourney struct{ projectJourney }

func newKiroProjectJourney(t *testing.T, transport *projectJourneyTransport) kiroProjectJourney {
	t.Helper()
	f := kiroProjectJourney{projectJourney{home: projectJourneyTemp(t), a: projectJourneyTemp(t), b: projectJourneyTemp(t), transport: transport}}
	f.cache = filepath.Join(f.home, "uap")
	for _, root := range []string{"", f.a, f.b} {
		native := f.kiroRoot(root)
		projectJourneyWrite(t, filepath.Join(native, "settings", "mcp.json"), `{"mcpServers":{"foreign":{"url":"https://foreign.test/mcp","disabled":true}},"foreignPreference":"keep"}`)
		projectJourneyWrite(t, filepath.Join(native, "skills", "foreign", "SKILL.md"), "foreign skill\n")
		projectJourneyWrite(t, filepath.Join(native, "hooks.json"), "foreign hooks\n")
	}
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
