package grok

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type fakeGrok struct {
	entries       []pluginEntry
	commands      [][]string
	failUninstall bool
	failInstall   bool
}

func (f *fakeGrok) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	f.commands = append(f.commands, append([]string(nil), command.Argv...))
	args := command.Argv
	if len(args) < 3 || args[1] != "plugin" {
		return legacyports.CommandResult{}, errors.New("unexpected command")
	}
	switch args[2] {
	case "list":
		body, _ := json.Marshal(f.entries)
		return legacyports.CommandResult{Stdout: body}, nil
	case "uninstall":
		if f.failUninstall {
			return legacyports.CommandResult{ExitCode: 1}, nil
		}
		f.entries = []pluginEntry{}
		return legacyports.CommandResult{}, nil
	case "install":
		if f.failInstall {
			return legacyports.CommandResult{ExitCode: 1}, nil
		}
		f.entries = []pluginEntry{{Name: "demo", Status: "installed", Source: args[4], Version: "2.0.0"}}
		return legacyports.CommandResult{}, nil
	default:
		return legacyports.CommandResult{}, errors.New("unexpected command")
	}
}

func grokRequest(t *testing.T) domain.ActivationRequest {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".grok")
	active := filepath.Join(root, "plugins", "demo")
	client := domain.DetectedClient{ClientID: domain.ClientGrok, ConfigRoot: root, ExecutablePath: "/test/bin/grok"}
	return domain.ActivationRequest{
		Client: client, DeclaredName: "demo", BackendExecutable: client.ExecutablePath,
		Plan:     domain.DeliveryPlan{ClientID: domain.ClientGrok, DeclaredName: "demo", DeclaredVersion: "2.0.0", NativeRegistryRoot: root, NativeRegistryExecutable: client.ExecutablePath, ActivePath: active},
		Delivery: domain.StagedDelivery{ClientID: domain.ClientGrok, ActivePath: active},
	}
}

func TestGrokListRequiresUnambiguousNativeIdentity(t *testing.T) {
	for _, body := range []string{"null", `{}`, `[{"name":"demo","status":"installed"},{"name":"demo","status":"disabled"}]`, `[{"name":"demo","status":"future"}]`, `[] {}`} {
		if _, err := parseList([]byte(body)); err == nil {
			t.Errorf("accepted ambiguous listing %s", body)
		}
	}
	if entries, err := parseList([]byte(`[]`)); err != nil || len(entries) != 0 {
		t.Fatalf("empty listing: %v, %v", entries, err)
	}
}

func TestGrokActivationUpdateAndReadOnlyVerification(t *testing.T) {
	r := grokRequest(t)
	f := &fakeGrok{entries: []pluginEntry{}}
	env := clients.Env{Runner: f}
	if out, err := New().Activate(context.Background(), env, r); err != nil || out.Verification != domain.VerificationInstalled {
		t.Fatalf("install: %+v %v", out, err)
	}
	before := len(f.commands)
	r.VerifyOnly = true
	if out, err := New().Activate(context.Background(), env, r); err != nil || out.Activation != domain.ActivationActive {
		t.Fatalf("verify: %+v %v", out, err)
	}
	if got := f.commands[before:]; len(got) != 1 || !reflect.DeepEqual(got[0][1:], []string{"plugin", "list", "--json"}) {
		t.Fatalf("verify mutated Grok: %v", got)
	}
	r.VerifyOnly = false
	r.Replacing = true
	f.entries[0].Version = "1.0.0"
	if out, err := New().Activate(context.Background(), env, r); err != nil || out.Verification != domain.VerificationInstalled {
		t.Fatalf("update: %+v %v", out, err)
	}
	before = len(f.commands)
	if out, err := New().Activate(context.Background(), env, r); err != nil || out.Verification != domain.VerificationInstalled {
		t.Fatalf("same-version update: %+v %v", out, err)
	}
	if len(f.commands)-before < 4 {
		t.Fatalf("same-version replacement skipped native reinstall: %v", f.commands[before:])
	}
}

func TestGrokRejectsForeignAndFailedUninstall(t *testing.T) {
	r := grokRequest(t)
	f := &fakeGrok{entries: []pluginEntry{{Name: "demo", Status: "installed", Source: filepath.Join(t.TempDir(), "foreign"), Version: "2.0.0"}}}
	if _, err := New().Activate(context.Background(), clients.Env{Runner: f}, r); err == nil {
		t.Fatal("foreign same-name plugin accepted")
	}
	if len(f.commands) != 1 {
		t.Fatalf("foreign plugin caused mutation: %v", f.commands)
	}
	f.entries[0].Source, f.entries[0].Version = r.Delivery.ActivePath, "1.0.0"
	f.failUninstall = true
	if _, err := New().Activate(context.Background(), clients.Env{Runner: f}, r); err == nil {
		t.Fatal("failed uninstall allowed update")
	}
	for _, command := range f.commands {
		if len(command) > 2 && command[2] == "install" {
			t.Fatalf("installed over failed uninstall: %v", f.commands)
		}
	}
}

func TestGrokDoesNotOverrideDisabledPlugin(t *testing.T) {
	r := grokRequest(t)
	f := &fakeGrok{entries: []pluginEntry{{Name: "demo", Status: "disabled", Source: r.Delivery.ActivePath, Version: "1.0.0"}}}
	out, err := New().Activate(context.Background(), clients.Env{Runner: f}, r)
	if err != nil || out.Activation != domain.ActivationManual {
		t.Fatalf("disabled plugin: %+v %v", out, err)
	}
	if len(f.commands) != 1 {
		t.Fatalf("disabled plugin was changed: %v", f.commands)
	}
}

func TestGrokUnconfirmedRemovalDoesNotTouchClient(t *testing.T) {
	r := grokRequest(t)
	f := &fakeGrok{entries: []pluginEntry{{Name: "demo", Status: "installed", Source: r.Delivery.ActivePath, Version: "2.0.0"}}}
	request := domain.DeactivationRequest{Client: r.Client, DeclaredName: "demo", BackendExecutable: r.BackendExecutable, ManagedArtifactPath: r.Delivery.ActivePath}
	if _, err := New().Deactivate(context.Background(), clients.Env{Runner: f}, request); err != nil || len(f.commands) != 0 {
		t.Fatal("unconfirmed removal touched Grok")
	}
	request.Confirmed = true
	if out, err := New().Deactivate(context.Background(), clients.Env{Runner: f}, request); err != nil || !out.ExternalRemovalComplete {
		t.Fatalf("remove: %+v %v", out, err)
	}
}
