package agentpluginscli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tailscale/hujson"
	"gopkg.in/yaml.v3"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// These recovery cases need known local native setup, not a remote HTTP
// authentication assumption. The configured stdio process is never executed.
func managementReadyRepository(t *testing.T) (string, string) {
	t.Helper()
	repo, _ := savedJourneyRepository(t)
	dependencyWrite(t, filepath.Join(repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"demo":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}}}}`)
	savedJourneyGit(t, repo, "add", ".")
	savedJourneyGit(t, repo, "commit", "-m", "test: locally observable management fixture")
	return repo, savedJourneyGit(t, repo, "rev-parse", "HEAD")
}

func managementWithoutCodexCLI(t *testing.T, journey *savedJourney, backend *ManagementBackend) *ManagementBackend {
	t.Helper()
	cfg := backend.config
	cfg.Targets = append([]management.StartupTarget(nil), cfg.Targets...)
	for index := range cfg.Targets {
		if cfg.Targets[index].Client == "codex" {
			cfg.Targets[index].Executable = ""
		}
	}
	selected, err := NewManagementBackend(backend.app, cfg)
	if err != nil {
		t.Fatal(err)
	}
	journey.cli.app.Detector = selected.app.Detector
	t.Cleanup(selected.Close)
	return selected
}

type managementNativeRunner struct {
	base     ports.CommandRunner
	profiles map[string]string
}

func newManagementNativeRunner(detected []domain.DetectedClient, base ports.CommandRunner) *managementNativeRunner {
	runner := &managementNativeRunner{base: base, profiles: map[string]string{}}
	for _, client := range detected {
		if client.ClientID == domain.ClientOpenCode {
			runner.profiles[client.ExecutablePath] = client.ConfigRoot
		}
	}
	return runner
}
func (r *managementNativeRunner) Run(ctx context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	profile, selected := r.profiles[command.Argv[0]]
	if !selected {
		return r.base.Run(ctx, command)
	}
	if fixture, ok := r.base.(*dependencyFixtureRunner); ok {
		fixture.calls++
	}
	configRoot := ""
	for _, entry := range command.Env {
		if strings.HasPrefix(entry, "OPENCODE_CONFIG_DIR=") {
			configRoot = strings.TrimPrefix(entry, "OPENCODE_CONFIG_DIR=")
		}
	}
	if configRoot != profile || command.Dir != profile {
		return legacyports.CommandResult{}, fmt.Errorf("management native fixture lost selected profile")
	}
	document, err := managementOpenCodeConfig(profile)
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	args := strings.Join(command.Argv[1:], " ")
	switch args {
	case "debug config":
		body, err := json.Marshal(document)
		return legacyports.CommandResult{Stdout: body}, err
	case "debug skill":
		rows := []map[string]string{}
		entries, err := os.ReadDir(filepath.Join(profile, "skills"))
		if err != nil && !os.IsNotExist(err) {
			return legacyports.CommandResult{}, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(profile, "skills", entry.Name(), "SKILL.md")
			body, err := os.ReadFile(path)
			if err != nil {
				return legacyports.CommandResult{}, err
			}
			text := strings.ReplaceAll(string(body), "\r\n", "\n")
			if !strings.HasPrefix(text, "---\n") {
				return legacyports.CommandResult{}, fmt.Errorf("fixture skill has no metadata")
			}
			header, content, ok := strings.Cut(text[4:], "\n---\n")
			if !ok {
				return legacyports.CommandResult{}, fmt.Errorf("fixture skill metadata is incomplete")
			}
			var metadata struct{ Name string }
			if err := yaml.Unmarshal([]byte(header), &metadata); err != nil {
				return legacyports.CommandResult{}, err
			}
			if metadata.Name == "" {
				return legacyports.CommandResult{}, fmt.Errorf("fixture skill has no name")
			}
			rows = append(rows, map[string]string{"name": metadata.Name, "location": path, "content": content})
		}
		body, err := json.Marshal(rows)
		return legacyports.CommandResult{Stdout: body}, err
	case "mcp list":
		servers, _ := document["mcp"].(map[string]any)
		if nested, ok := servers["servers"].(map[string]any); ok {
			servers = nested
		}
		names := make([]string, 0, len(servers))
		for name := range servers {
			names = append(names, name)
		}
		sort.Strings(names)
		var body strings.Builder
		body.WriteString("┌  MCP Servers\n")
		for _, name := range names {
			server, ok := servers[name].(map[string]any)
			if !ok {
				return legacyports.CommandResult{}, fmt.Errorf("fixture MCP entry is invalid")
			}
			command, local := server["command"].([]any)
			connected := server["type"] == "local" && local && len(command) > 0 && server["enabled"] != false && server["disabled"] != true
			if connected {
				fmt.Fprintf(&body, "● ✓ %s connected\n", name)
			} else {
				fmt.Fprintf(&body, "● ✘ %s disconnected\n", name)
			}
		}
		fmt.Fprintf(&body, "└  %d server(s)\n", len(names))
		return legacyports.CommandResult{Stdout: []byte(body.String())}, nil
	default:
		return legacyports.CommandResult{}, fmt.Errorf("unexpected management native fixture command")
	}
}
func (r *managementNativeRunner) RunDuplexToCompletion(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	if underlying, ok := r.base.(ports.CompletionCommandRunner); ok {
		return underlying.RunDuplexToCompletion(ctx, command, exchange)
	}
	if native, ok := r.base.(*dependencyClaudeListing); ok {
		return savedCodexDiscovery(command, native.codexIDs, exchange)
	}
	return fmt.Errorf("management native fixture has no selected interactive protocol")
}
func managementOpenCodeConfig(profile string) (map[string]any, error) {
	document := map[string]any{}
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		body, err := os.ReadFile(filepath.Join(profile, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		body, err = hujson.Standardize(body)
		if err != nil {
			return nil, err
		}
		var current map[string]any
		if err := json.Unmarshal(body, &current); err != nil {
			return nil, err
		}
		for key, value := range current {
			document[key] = value
		}
	}
	return document, nil
}
func managementDependencySpecHarness(t *testing.T, repo string, spec dependencyClientSpec) (cliFixture, *dependencyFixtureRunner) {
	t.Helper()
	if spec.id != domain.ClientOpenCode {
		fixture, _, runner := dependencySpecHarness(t, repo, spec)
		return fixture, runner
	}
	client := fixtureClient(t, spec.id)
	if spec.executable {
		// Management captures canonical executable identity before observation.
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		client.ExecutablePath = filepath.Join(root, string(spec.id))
		dependencyWrite(t, client.ExecutablePath, "inert native fixture; never launched")
	}
	runner := &dependencyFixtureRunner{id: spec.id}
	cfg := installer.Config{Runner: newManagementNativeRunner([]domain.DetectedClient{client}, runner), EnableNativeObserver: true, OpenCodeProbe: func(_ context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
		runner.probes++
		return clientdetect.ProbeEvidence{VersionEvidence: opencodehost.VersionEvidence{Version: "1.18.34", Source: "executable_version", ProbeStatus: "ok", ExecutableIdentity: "fixture:" + target.Executable}}, nil
	}}
	fixture, _ := newDependencyHarness(t, repo, []domain.DetectedClient{client}, cfg)
	return fixture, runner
}
