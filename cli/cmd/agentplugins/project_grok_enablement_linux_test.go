//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type projectGrokEnableRunner struct {
	*projectGrokRunner
	home string
	acp  int
}

func (*projectGrokEnableRunner) DuplexCapability() error { return nil }

func (runner *projectGrokEnableRunner) RunDuplexWithPlannedShutdown(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	runner.acp++
	if strings.Join(command.Argv, "\x00") != strings.Join([]string{runner.executable, "agent", "--no-leader", "stdio"}, "\x00") {
		return fmt.Errorf("unexpected native effect command: %v", command.Argv)
	}
	inspection, err := runner.Run(ctx, legacyports.Command{Argv: []string{runner.executable, "inspect", "--json"}, Dir: command.Dir})
	if err != nil {
		return err
	}
	var inspected struct {
		Plugins []struct{ Name, Scope, Path string }
	}
	if err := json.Unmarshal(inspection.Stdout, &inspected); err != nil {
		return err
	}
	body, err := os.ReadFile(filepath.Join(runner.home, ".grok", "config.toml"))
	if err != nil {
		return err
	}
	var config struct {
		Plugins struct {
			Enabled []string `toml:"enabled"`
		} `toml:"plugins"`
	}
	if err := toml.Unmarshal(body, &config); err != nil {
		return err
	}
	plugins := []any{}
	for _, plugin := range inspected.Plugins {
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(plugin.Path)))
		id, enabled := "project/"+digest[:8]+"/"+plugin.Name, false
		for _, member := range config.Plugins.Enabled {
			enabled = enabled || member == id
		}
		manifest, err := os.ReadFile(filepath.Join(plugin.Path, ".grok-plugin", "plugin.json"))
		if err != nil {
			return err
		}
		var metadata struct{ Version string }
		if err := json.Unmarshal(manifest, &metadata); err != nil {
			return err
		}
		plugins = append(plugins, map[string]any{"id": id, "name": plugin.Name, "scope": plugin.Scope, "root": plugin.Path, "version": metadata.Version, "enabled": enabled, "trusted": false})
	}
	var frames, requests bytes.Buffer
	encoder := json.NewEncoder(&frames)
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"protocolVersion": 1, "_meta": map[string]any{"agentVersion": "1.0.70", "currentWorkingDirectory": command.Dir}}}); err != nil {
		return err
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "result": map[string]any{"result": map[string]any{"plugins": plugins}}}); err != nil {
		return err
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	if _, err := io.Copy(writer, &frames); err != nil {
		return err
	}
	return exchange(&requests, reader)
}

func configureProjectGrokEnablement(app agentpluginscli.App, runner *projectGrokEnableRunner) agentpluginscli.App {
	helper, err := os.Executable()
	if err != nil {
		runner.t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{UserHome: runner.home, ProjectRoot: app.ProjectRoot, StateRoot: filepath.Dir(app.ManagedRoot), ManagedRoot: app.ManagedRoot,
		OperationsDir: app.Lifecycle.Kernel.Directory.JournalDir, Registry: app.ClientRegistry, Runner: runner, EnableNativeObserver: true, HelperExecutable: helper})
	if err != nil {
		runner.t.Fatal(err)
	}
	app.Installer = engine
	app.Detector = projectGrokDetector{user: filepath.Join(runner.home, ".grok"), executable: runner.executable, version: "1.0.70"}
	activator, ok := app.Lifecycle.Activator.(providers.Activator)
	if !ok {
		runner.t.Fatal("production composition did not provide its client activator")
	}
	activator.Runner = runner
	app.Lifecycle.Activator = activator
	observer, ok := app.Lifecycle.NativeObserver.(providers.NativeIdentityObserver)
	if !ok {
		runner.t.Fatal("production composition did not provide its native observer")
	}
	observer.Runner = runner
	app.Lifecycle.NativeObserver = observer
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureProjectGrokEnablement(selected, runner), nil
	}
	return app
}

func declineProjectGrokRepair(t *testing.T, journey projectJourney, input string) {
	t.Helper()
	state := journey.state(t, journey.a)
	var output bytes.Buffer
	app := journey.app
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(input), &output, &output, true
	command := agentpluginscli.NewRoot(app)
	command.SetArgs([]string{"repair", "docs", "--target", "grok", "--scope", "project", "--project-root", journey.a, "--confirm-plugin-enable", "--format", "human"})
	err := command.ExecuteContext(t.Context())
	if input != "" && err != nil {
		t.Fatalf("interactive decline: %v\n%s", err, output.String())
	}
	if !strings.Contains(output.String(), "[y/N]") || !reflect.DeepEqual(journey.state(t, journey.a), state) {
		t.Fatalf("consent did not reach the unchanged saved repair confirmation: %v\n%s", err, output.String())
	}
}

func TestProjectGrokCLITLSZipSavedRepairConsentLifecycle(t *testing.T) {
	transport := newProjectJourneyTransport(t, "zip")
	projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://inert.test/mcp"}}}`)
	transport.mu.Lock()
	transport.archive = projectJourneyArchive(t, transport.repo)
	transport.mu.Unlock()
	journey := newProjectJourney(t, transport)
	config := filepath.Join(journey.home, ".grok", "config.toml")
	before := "# retained foreign settings\n[plugins]\nenabled=['foreign']\ndisabled=['other']\n"
	projectJourneyWrite(t, config, before)
	projectJourneyWrite(t, filepath.Join(journey.home, ".grok", ".config-init.lock"), "")
	runner := &projectGrokEnableRunner{projectGrokRunner: &projectGrokRunner{t: t, executable: filepath.Join(journey.home, "grok-not-executed"), version: "1.0.70"}, home: journey.home}
	journey.app = configureProjectGrokEnablement(journey.app, runner)
	journey.run(t, journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "grok")
	installation, binding := journey.owned(t, journey.a)
	journey.run(t, journey.a, "repair", "docs", "--target", "grok", "--yes")
	journey.run(t, journey.a, "repair", "docs", "--target", "grok", "--confirm-plugin-enable", "--dry-run", "--yes")
	declineProjectGrokRepair(t, journey, "n\n")
	declineProjectGrokRepair(t, journey, "")
	if runner.acp != 0 || projectJourneyRead(t, config) != before {
		t.Fatal("ordinary --yes, dry-run, decline or EOF enabled the native plugin")
	}
	for _, args := range [][]string{
		{"repair", "docs", "--confirm-plugin-enable", "--yes"},
		{"repair", "docs", "--target", "grok,cline", "--confirm-plugin-enable", "--yes"},
		{"repair", "docs", "--target", "grok,grok", "--confirm-plugin-enable", "--yes"},
	} {
		if output, err := journey.execute(journey.a, args...); err == nil {
			t.Fatal("non-exact consent selection accepted", args, output)
		}
	}
	if output, err := journey.execute("", "repair", "docs", "--target", "grok", "--confirm-plugin-enable", "--yes"); err == nil {
		t.Fatal("implicit scope accepted", output)
	}
	if runner.acp != 0 || projectJourneyRead(t, config) != before {
		t.Fatal("refused selection touched native config")
	}
	journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "grok", "--confirm-plugin-enable", "--yes")
	_, enabled := journey.owned(t, journey.a)
	if runner.acp != 2 || enabled.Activation != domain.ActivationPrepared || len(enabled.NativeObjects) != len(binding.NativeObjects)+1 {
		t.Fatalf("CLI did not commit exact prepared enablement: %+v ACP=%d", enabled, runner.acp)
	}
	enabledBytes := projectJourneyRead(t, config)
	journey.run(t, journey.a, "repair", "docs", "--target", "grok", "--confirm-plugin-enable", "--yes")
	if runner.acp != 3 || projectJourneyRead(t, config) != enabledBytes {
		t.Fatal("repeated CLI consent changed native config")
	}
	journey.run(t, journey.a, "remove", "docs", "--target", "grok")
	if projectJourneyRead(t, config) != before {
		t.Fatal("CLI removal did not preserve foreign native settings")
	}
	projectJourneyAbsent(t, binding.TargetLocator)
}
