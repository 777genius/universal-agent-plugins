package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// This proves the public source's first/repeated imperative add branch with the
// production composer, filesystem, planner, native kernel and persisted state.
// Only --version is an inert compiled TEST process; no vendor/MCP/model runs.
func TestOpenCodeProjectPublicRepeatProgressSource(t *testing.T) {
	base := projectJourneyTemp(t)
	home, project := filepath.Join(base, "home"), filepath.Join(base, "A")
	for _, dir := range []string{home, project} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	pkg := filepath.Join(base, "source-A")
	openCodeProgressPackage(t, pkg)
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(home, filepath.Join(home, "uap"), nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(home, "opencode-user")
	projectJourneyWrite(t, filepath.Join(user, "opencode.jsonc"), "{// foreign user\n\"mcp\":{}}\n")
	projectJourneyWrite(t, filepath.Join(project, "opencode.jsonc"), "{// foreign project\n\"mcp\":{},\"default_agent\":\"uap-inert-project\"}\n")
	app = configureOpenCodeProgressApp(app, projectOpenCodeDetector{root: user, executable: projectOpenCodeProbeFixture(t)})
	var installedProject, installedHome map[string]string
	for _, label := range []string{"first-add", "repeat-add"} {
		var output, stderr bytes.Buffer
		selected := app
		selected.Input, selected.Output, selected.ErrorOutput, selected.Terminal = strings.NewReader(""), &output, &stderr, false
		command := agentpluginscli.NewRoot(selected)
		command.SetArgs([]string{"add", pkg, "--no-save", "--target", "opencode", "--scope", "project", "--project-root", project, "--format", "json"})
		err := command.ExecuteContext(context.Background())
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		expected := "Resolving and validating one Agent Plugin package for every selected target...\nInstalling uap-commands-fixture for 1 selected clients...\n"
		if label == "repeat-add" {
			expected = "Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"
		}
		if stderr.String() != expected {
			t.Fatalf("%s progress: got %q, want %q", label, stderr.String(), expected)
		}
		var envelope struct {
			Result string `json:"result"`
			Data   struct {
				Result struct {
					Mutated *bool `json:"mutated"`
				} `json:"result"`
			} `json:"data"`
		}
		if err := json.Unmarshal(output.Bytes(), &envelope); err != nil || envelope.Result != "success" {
			t.Fatalf("%s public result: %v %s", label, err, output.Bytes())
		}
		if label == "repeat-add" {
			if envelope.Data.Result.Mutated == nil || *envelope.Data.Result.Mutated {
				t.Fatal("repeat add reported actual mutation or omitted the required flag")
			}
			for label, pair := range map[string][2]map[string]string{"project": {installedProject, openCodeProgressProjectSnapshot(t, project)}, "home": {installedHome, securityFixtureTree(t, home)}} {
				if !reflect.DeepEqual(pair[0], pair[1]) {
					for path, before := range pair[0] {
						if pair[1][path] != before {
							t.Logf("%s changed %s: %s -> %s", label, path, before, pair[1][path])
						}
					}
					for path, after := range pair[1] {
						if _, exists := pair[0][path]; !exists {
							t.Logf("%s added %s: %s", label, path, after)
						}
					}
					t.Fatal("repeat add changed project/state/skill/native configuration or original user bytes")
				}
			}
		}
		t.Logf("%s exact_stderr=%q result=success", label, stderr.String())
		scoped, err := app.ForScope(domain.ScopeProject, project)
		if err != nil {
			t.Fatal(err)
		}
		state, err := scoped.StateStore.Load()
		if err != nil || len(state.Installations) != 1 {
			t.Fatalf("real saved state: %+v %v", state, err)
		}
		for _, binding := range state.Installations[0].Clients {
			if binding.Scope != "project" || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled {
				t.Fatalf("real Prepared binding differs: %+v", binding)
			}
		}
		installedProject, installedHome = openCodeProgressProjectSnapshot(t, project), securityFixtureTree(t, home)
	}
}

func openCodeProgressProjectSnapshot(t *testing.T, project string) map[string]string {
	t.Helper()
	entries := securityFixtureTree(t, project)
	// Every mutation admission refreshes this process-lock payload, even a
	// read-only replay. Preserve its presence/type/mode; compare every other
	// project file, including installed state and native configuration, exactly.
	key := filepath.Join(".uap", "mutation.lock")
	mode, _, ok := strings.Cut(entries[key], "/")
	if !ok {
		t.Fatal("expected exact project mutation lock is missing")
	}
	entries[key] = mode + "/ephemeral-process-lock"
	return entries
}

func configureOpenCodeProgressApp(app agentpluginscli.App, detector projectOpenCodeDetector) agentpluginscli.App {
	app.Detector = detector
	app.SecurityIndex, app.DirectoryClient, app.DiscoveryClient = nil, nil, nil
	app.SecurityEvaluator = projectJourneySecurity{}
	app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return projectJourneySecurity{} }
	app.DirectoryClientForDryRun, app.CommitDirectoryPreview = nil, nil
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureOpenCodeProgressApp(selected, detector), nil
	}
	return app
}

func openCodeProgressPackage(t *testing.T, target string) {
	t.Helper()
	root := filepath.Join("..", "..", "..", "repotests", "testdata", "agentplugins_commands_fixture")
	for _, name := range []string{"plugin.json", "skills/fixture-guide/SKILL.md", "io.github.777genius.agentplugins/commands/echo.md", "io.github.777genius.agentplugins/commands/wizard.md"} {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		projectJourneyWrite(t, filepath.Join(target, filepath.FromSlash(name)), string(body))
	}
	projectJourneyWrite(t, filepath.Join(target, "mcp.json"), "{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json\",\"mcpServers\":{\"scope-marker\":{\"type\":\"streamable-http\",\"url\":\"http://127.0.0.1:9/A-one\"}}}")
}
