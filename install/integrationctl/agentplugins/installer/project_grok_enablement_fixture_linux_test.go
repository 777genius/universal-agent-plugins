//go:build linux

package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/grok"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// The public facade, real package projection and durable state run unchanged.
// Only Grok's process transport is an inert, finite native-protocol fixture.
type grokEnableSDKFixture struct {
	project, home, pkg, config string
	engine                     *Engine
	native                     *grokEnableSDKNative
	request                    Request
}

type grokEnableSDKNative struct {
	home, executable string
	calls, duplex    int
	wrongReadback    bool
	afterReadback    func()
}

func newGrokEnableSDKFixture(t *testing.T, saved bool) *grokEnableSDKFixture {
	t.Helper()
	base := t.TempDir()
	f := &grokEnableSDKFixture{project: filepath.Join(base, "project"), home: filepath.Join(base, "home"), pkg: filepath.Join(base, "package")}
	f.config = filepath.Join(f.home, ".grok", "config.toml")
	deathWrite(t, filepath.Join(f.project, ".git", "fixture"), "inert project boundary")
	deathWrite(t, f.config, "# private foreign settings\n[plugins]\nenabled=['foreign']\ndisabled=['other']\n")
	deathWrite(t, filepath.Join(f.home, ".grok", ".config-init.lock"), "")
	deathWrite(t, filepath.Join(f.pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"grok-sdk-owned","version":"1.0.0"}`)
	deathWrite(t, filepath.Join(f.pkg, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: inert Grok SDK fixture\n---\nOriginal immutable skill.\n")
	f.native = &grokEnableSDKNative{home: f.home, executable: filepath.Join(base, "grok-fixture")}
	f.engine = f.reopen(t, f.home)
	f.request = Request{Operation: OpInstall, Scope: domain.ScopeProject, ProjectRoot: f.project, PackageRoot: f.pkg,
		ClientID: "grok", ClientConfigRoot: filepath.Join(f.home, ".grok"), ClientExecutable: f.native.executable, ClientVersion: "1.0.70",
		InstallationID: "00000000-0000-4000-8000-000000001070", OperationID: "grok-sdk-install", RequiredComponents: []string{"skills"}}
	if saved {
		f.request.EnvironmentAuthority = &domain.EnvironmentAuthority{EnvironmentID: "project-default", Alias: "docs", ClientID: "grok", ExpectedAction: string(OpInstall), SourceKey: "github::"}
	}
	return f
}

func (f *grokEnableSDKFixture) reopen(t *testing.T, home string) *Engine {
	t.Helper()
	registry, err := clients.NewRegistry(grok.New())
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := New(Config{ProjectRoot: f.project, UserHome: home, StateRoot: filepath.Join(f.project, ".uap"), Registry: registry,
		Runner: f.native, EnableNativeObserver: true, TrustedLocalPackages: true, HelperExecutable: helper})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func (f *grokEnableSDKFixture) selected(t *testing.T, operation Operation, consent bool) Request {
	t.Helper()
	req := f.request
	req.Operation, req.OperationID, req.ConfirmPluginEnable = operation, "grok-sdk-"+string(operation), consent
	if req.EnvironmentAuthority != nil {
		view, err := f.engine.Inspect(t.Context())
		if err != nil || len(view.Installations) != 1 || len(view.Installations[0].Bindings) != 1 {
			t.Fatalf("saved selection: %+v %v", view, err)
		}
		binding := view.Installations[0].Bindings[0]
		if binding.Owner == nil {
			t.Fatal("public install did not produce an environment receipt")
		}
		authority := *req.EnvironmentAuthority
		authority.ExpectedAction, authority.ReceiptID = string(operation), binding.Owner.ReceiptID
		authority.InstallationID, authority.ClientBindingID = req.InstallationID, binding.BindingID
		req.EnvironmentAuthority = &authority
	}
	return req
}

func (f *grokEnableSDKFixture) execute(t *testing.T, req Request, decision Decision) (Result, error) {
	t.Helper()
	prepared, err := f.engine.Prepare(t.Context(), req)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		if err := prepared.Close(); err != nil {
			t.Error(err)
		}
	}()
	return f.engine.Apply(t.Context(), prepared, decision)
}

func (f *grokEnableSDKFixture) install(t *testing.T) {
	t.Helper()
	if result, err := f.execute(t, f.request, Decision{Confirmed: true}); err != nil || result.Outcome != OutcomeCompleted {
		t.Fatalf("public install: %+v %v", result, err)
	}
}

func (f *grokEnableSDKFixture) binding(t *testing.T) domain.ClientBinding {
	t.Helper()
	state, err := f.engine.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("stored binding: %+v %v", state, err)
	}
	for _, binding := range state.Installations[0].Clients {
		return binding
	}
	t.Fatal("owned binding absent")
	return domain.ClientBinding{}
}

func (native *grokEnableSDKNative) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	native.calls++
	if !reflect.DeepEqual(command.Argv, []string{native.executable, "inspect", "--json"}) {
		return legacyports.CommandResult{}, fmt.Errorf("unexpected command: %v", command.Argv)
	}
	rows, err := native.plugins(command.Dir)
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	plugins := []any{}
	for _, row := range rows {
		plugins = append(plugins, map[string]any{"name": row["name"], "scope": "project", "path": row["root"], "enabled": false, "provides": map[string]any{"mcpServers": 0}})
	}
	body, err := json.Marshal(map[string]any{"grokVersion": "1.0.70", "cwd": command.Dir, "projectRoot": command.Dir, "projectTrusted": false, "skills": []any{}, "plugins": plugins})
	return legacyports.CommandResult{Stdout: body}, err
}

func (*grokEnableSDKNative) DuplexCapability() error { return nil }

func (native *grokEnableSDKNative) RunDuplexWithPlannedShutdown(_ context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	native.duplex++
	if !reflect.DeepEqual(command.Argv, []string{native.executable, "agent", "--no-leader", "stdio"}) {
		return fmt.Errorf("unexpected ACP command: %v", command.Argv)
	}
	plugins, err := native.plugins(command.Dir)
	if err != nil {
		return err
	}
	if native.wrongReadback && native.duplex == 2 {
		for _, row := range plugins {
			row["enabled"] = false
		}
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
	err = exchange(&requests, reader)
	if native.afterReadback != nil && native.duplex == 2 {
		native.afterReadback()
	}
	decoder := json.NewDecoder(&requests)
	for _, method := range []string{"initialize", "_x.ai/plugins/list"} {
		var request map[string]any
		if decodeErr := decoder.Decode(&request); decodeErr != nil || request["method"] != method {
			return errors.Join(err, decodeErr, fmt.Errorf("unexpected native request: %v", request))
		}
	}
	var extra any
	if decodeErr := decoder.Decode(&extra); decodeErr != io.EOF {
		return errors.Join(err, decodeErr, fmt.Errorf("extra native request: %v", extra))
	}
	return err
}

func (native *grokEnableSDKNative) plugins(project string) ([]map[string]any, error) {
	body, err := os.ReadFile(filepath.Join(native.home, ".grok", "config.toml"))
	if err != nil {
		return nil, err
	}
	var settings struct {
		Plugins struct {
			Enabled []string `toml:"enabled"`
		} `toml:"plugins"`
	}
	if err := toml.Unmarshal(body, &settings); err != nil {
		return nil, err
	}
	root := filepath.Join(project, ".grok", "plugins")
	entries, err := os.ReadDir(root)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	rows := []map[string]any{}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		body, err := os.ReadFile(filepath.Join(path, ".grok-plugin", "plugin.json"))
		if err != nil {
			return nil, err
		}
		var manifest struct{ Name, Version string }
		if err := json.Unmarshal(body, &manifest); err != nil {
			return nil, err
		}
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(path)))
		id, enabled := "project/"+digest[:8]+"/"+manifest.Name, false
		for _, member := range settings.Plugins.Enabled {
			enabled = enabled || member == id
		}
		rows = append(rows, map[string]any{"id": id, "name": manifest.Name, "version": manifest.Version, "root": path, "scope": "project", "enabled": enabled, "trusted": false})
	}
	return rows, nil
}
