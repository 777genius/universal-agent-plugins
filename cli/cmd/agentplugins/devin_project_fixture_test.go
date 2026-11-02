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

type devinProjectDetector struct{ root string }

func (detector devinProjectDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	return []domain.DetectedClient{{ClientID: domain.ClientWindsurf, ConfigRoot: "", ExecutablePath: filepath.Join(detector.root, "devin"), Version: "3000.11.205", Status: domain.DetectionDetected}}, nil
}

// Reuse the core journey's production CLI/state helpers and real TLS Git/ZIP
// transports. Devin supplies its own detected profile and native layout.
type devinProjectJourney struct{ projectJourney }

func newDevinProjectJourney(t *testing.T, transport *projectJourneyTransport) devinProjectJourney {
	t.Helper()
	f := devinProjectJourney{projectJourney{home: devinJourneyTemp(t), a: devinJourneyTemp(t), b: devinJourneyTemp(t), transport: transport}}
	f.cache = filepath.Join(f.home, "uap")
	for _, root := range []string{"", f.a, f.b} {
		native := f.devinRoot(root)
		projectJourneyWrite(t, filepath.Join(native, "mcp_config.json"), `{"mcpServers":{"foreign":{"url":"https://foreign.test/mcp","disabled":true}},"foreignPreference":"keep"}`)
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
	f.app = configureDevinProjectJourney(app, f.home, transport)
	return f
}

// Preserve the production scoped composer, installer, ownership kernel, store,
// registry and solver. Only host discovery and security assessment are inert.
func configureDevinProjectJourney(app agentpluginscli.App, home string, transport *projectJourneyTransport) agentpluginscli.App {
	app.Detector = devinProjectDetector{root: filepath.Join(home, ".devin")}
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
		return configureDevinProjectJourney(selected, home, transport), nil
	}
	return app
}

func (f devinProjectJourney) devinRoot(project string) string {
	if project == "" {
		return filepath.Join(f.home, ".devin")
	}
	return filepath.Join(project, ".devin")
}

func (f devinProjectJourney) assertDevinOwned(t *testing.T, project, skill, endpoint string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	installation, binding := f.owned(t, project)
	if binding.Scope != string(domain.ScopeProject) || binding.NativeProfileRoot != f.devinRoot(project) || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
		t.Fatalf("Devin project lifecycle lost its exact scope or conservative observation: %+v", binding)
	}
	if err := pathpolicy.RequireContainedChild(filepath.Join(project, ".uap"), binding.TargetLocator); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{binding.TargetLocator, f.devinRoot(project)} {
		if projectJourneyRead(t, filepath.Join(root, "skills", "docs", "SKILL.md")) != skill {
			t.Fatal("Devin managed/native skill bytes differ from the selected pin")
		}
	}
	settings := projectJourneyRead(t, filepath.Join(f.devinRoot(project), "mcp_config.json"))
	for _, text := range []string{endpoint, "foreign.test", "foreignPreference", `"disabled":true`} {
		if !strings.Contains(strings.ReplaceAll(settings, " ", ""), text) {
			t.Fatalf("Devin settings lost selected or foreign member %q: %s", text, settings)
		}
	}
	return installation, binding
}

// Select inert remote configuration so the composed proof never executes an
// MCP command, copies a test binary as a launcher, or starts a native client.
func devinProjectCommitSource(t *testing.T, transport *projectJourneyTransport, skill, endpoint string) {
	t.Helper()
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), skill)
	projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"`+endpoint+`"}}}`)
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "select inert Devin project components")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.archive = projectJourneyArchive(t, transport.repo)
}

func devinJourneyTemp(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
