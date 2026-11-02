package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func TestProjectClineStandaloneCLIPresenceAllowsFileOnlyLocalAddRemove(t *testing.T) {
	t.Setenv("CLINE_MCP_SETTINGS_PATH", "")
	t.Setenv("CLINE_DATA_DIR", "")
	journey := projectJourney{home: projectJourneyTemp(t), a: projectJourneyTemp(t), b: projectJourneyTemp(t)}
	journey.cache = filepath.Join(journey.home, "uap")
	source := projectJourneyTemp(t)
	projectJourneyWrite(t, filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"cline-presence","version":"1.0.0"}`)
	projectJourneyWrite(t, filepath.Join(source, "skills", "docs", "SKILL.md"), projectJourneySkill)
	projectJourneyWrite(t, filepath.Join(source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"node","args":["${PLUGIN_ROOT}/server.js"],"cwd":"${PLUGIN_ROOT}"}}}`)
	projectJourneyWrite(t, filepath.Join(source, "server.js"), "inert source bytes; never executed\n")
	if err := os.Mkdir(filepath.Join(journey.a, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(journey.home, ".cline")
	projectJourneyWrite(t, filepath.Join(user, "data", "settings", "cline_mcp_settings.json"), `{"mcpServers":{"foreign-user":{"command":"/never-run","args":[],"disabled":true}}}`)
	projectJourneyWrite(t, filepath.Join(user, "skills", "foreign-user", "SKILL.md"), "original user skill\n")
	projectJourneyWrite(t, filepath.Join(journey.b, ".cline", "skills", "foreign-b", "SKILL.md"), "other project skill\n")
	projectJourneyWrite(t, filepath.Join(journey.b, ".uap", "keep"), "other project authority\n")
	foreignA := filepath.Join(journey.a, ".cline", "skills", "foreign-a", "SKILL.md")
	projectJourneyWrite(t, foreignA, "foreign A skill\n")
	siblingA := filepath.Join(journey.a, ".gemini", "keep")
	projectJourneyWrite(t, siblingA, "other A client\n")
	beforeUser, beforeB := securityFixtureTree(t, user), securityFixtureTree(t, journey.b)
	schemas, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(journey.home, journey.cache, nil, nil, nil, schemas)
	if err != nil {
		t.Fatal(err)
	}
	runner := &projectClineNoNative{}
	probes, lookups := 0, 0
	var configure func(agentpluginscli.App) agentpluginscli.App
	configure = func(app agentpluginscli.App) agentpluginscli.App {
		detector := clientdetect.NewOS(journey.home)
		detector.Registry, detector.Environment = app.ClientRegistry, map[string]string{}
		detector.LookPath = func(name string) (string, error) {
			if name == "cline" {
				lookups++
				return filepath.Join(journey.home, "inert-bin", "cline"), nil
			}
			return "", exec.ErrNotFound
		}
		detector.ProbeVersionWithEnvironment = func(context.Context, string, []string) (string, error) {
			probes++
			return "", errors.New("presence-only Cline must not execute a version probe")
		}
		app.Detector = detector
		app.SecurityIndex, app.DirectoryClient, app.DiscoveryClient = nil, nil, nil
		app.SecurityEvaluator = projectJourneySecurity{}
		app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return projectJourneySecurity{} }
		helper, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		engine, err := installer.New(installer.Config{StateRoot: filepath.Dir(app.ManagedRoot), ManagedRoot: app.ManagedRoot, ProjectRoot: app.ProjectRoot, Registry: app.ClientRegistry, Runner: runner, EnableNativeObserver: true, HelperExecutable: helper})
		if err != nil {
			t.Fatal(err)
		}
		app.Installer = engine
		activator, ok := app.Lifecycle.Activator.(providers.Activator)
		if !ok {
			t.Fatal("production Cline activator missing")
		}
		activator.Runner = runner
		app.Lifecycle.Activator = activator
		observer, ok := app.Lifecycle.NativeObserver.(providers.NativeIdentityObserver)
		if !ok {
			t.Fatal("production Cline native observer missing")
		}
		observer.Runner = runner
		app.Lifecycle.NativeObserver = observer
		compose := app.ForScope
		app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
			selected, err := compose(scope, root)
			if err != nil {
				return selected, err
			}
			return configure(selected), nil
		}
		return app
	}
	journey.app = configure(app)
	output := journey.run(t, journey.a, "add", source, "--no-save", "--target", "cline")
	if !strings.Contains(output, `"activation":"prepared"`) || !strings.Contains(output, `"verification":"installation_verified"`) {
		t.Fatal("public add did not retain Prepared file-delivery result", output)
	}
	state := journey.state(t, journey.a)
	if len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 {
		t.Fatalf("local no-save route changed ownership: %+v", state)
	}
	installation := state.Installations[0]
	if installation.Source.CanonicalSource != source || len(installation.Clients) != 1 {
		t.Fatal("local source or exact selected binding changed")
	}
	var selected domain.ClientBinding
	for _, binding := range installation.Clients {
		selected = binding
		if binding.ClientID != "cline" || binding.Scope != "project" || binding.NativeProfileRoot != journey.a || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || len(binding.NativeObjects) != 3 {
			t.Fatalf("file-only project authority differs: %+v", binding)
		}
	}
	if projectJourneyRead(t, filepath.Join(journey.a, ".cline", "skills", "docs", "SKILL.md")) != projectJourneySkill || !strings.Contains(projectJourneyRead(t, filepath.Join(journey.a, ".uap", "clients", "cline", "mcp.json")), `"docs"`) {
		t.Fatal("actual project skill/MCP delivery missing")
	}
	journey.run(t, journey.a, "remove", installation.InstallationID, "--target", "cline")
	projectJourneyAbsent(t, selected.TargetLocator, filepath.Join(journey.a, ".cline", "skills", "docs"), filepath.Join(journey.cache, "state-v2.json"), filepath.Join(journey.home, ".vscode"), filepath.Join(journey.home, ".cursor"))
	if !reflect.DeepEqual(beforeUser, securityFixtureTree(t, user)) || !reflect.DeepEqual(beforeB, securityFixtureTree(t, journey.b)) || projectJourneyRead(t, foreignA) != "foreign A skill\n" || projectJourneyRead(t, siblingA) != "other A client\n" {
		t.Fatal("CLI-only project delivery changed original user, sibling project or foreign client facts")
	}
	config, err := os.ReadFile(filepath.Join(journey.a, ".uap", "clients", "cline", "mcp.json"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err == nil {
		var document struct {
			Servers map[string]json.RawMessage `json:"mcpServers"`
		}
		if err := json.Unmarshal(config, &document); err != nil {
			t.Fatal(err)
		}
		if _, exists := document.Servers["docs"]; exists {
			t.Fatal("removed project retained its owned MCP docs member")
		}
	}
	for _, remaining := range journey.state(t, journey.a).Installations {
		if len(remaining.Clients) != 0 {
			t.Fatalf("removed project retained a client binding: %+v", remaining.Clients)
		}
	}
	if lookups == 0 || probes != 0 || runner.calls != 0 {
		t.Fatalf("presence-only native boundary differs: lookups=%d probes=%d calls=%d", lookups, probes, runner.calls)
	}
}
