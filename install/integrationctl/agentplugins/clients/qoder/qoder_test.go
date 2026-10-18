package qoder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type fakeQoder struct {
	entries       []pluginEntry
	commands      [][]string
	failUninstall bool
	failInstall   bool
	unknownList   bool
}

func (f *fakeQoder) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	f.commands = append(f.commands, append([]string(nil), command.Argv...))
	args := command.Argv
	if len(args) < 3 || args[1] != "plugins" {
		return legacyports.CommandResult{}, errors.New("unexpected command")
	}
	switch args[2] {
	case "list":
		if f.unknownList {
			return legacyports.CommandResult{Stdout: []byte(`{"shape":"unknown"}`)}, nil
		}
		entries := []map[string]string{}
		for _, entry := range f.entries {
			item := map[string]string{"id": entry.ID}
			if entry.Version != "" {
				item["version"] = entry.Version
			}
			entries = append(entries, item)
		}
		body, _ := json.Marshal(entries)
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
		f.entries = []pluginEntry{{ID: "demo@local", Version: "2.0.0"}}
		return legacyports.CommandResult{}, nil
	default:
		return legacyports.CommandResult{}, errors.New("unexpected command")
	}
}

func qoderRequest(t *testing.T) domain.ActivationRequest {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".qoder")
	active := filepath.Join(t.TempDir(), "managed", "clients", "qoder", "demo")
	if err := os.MkdirAll(active, 0o755); err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientQoder, ConfigRoot: root, ExecutablePath: "/test/bin/qoder"}
	return domain.ActivationRequest{
		Client: client, DeclaredName: "demo", BackendExecutable: client.ExecutablePath,
		Plan:     domain.DeliveryPlan{ClientID: domain.ClientQoder, DeclaredName: "demo", DeclaredVersion: "2.0.0", NativeRegistryRoot: root, NativeRegistryExecutable: client.ExecutablePath, ActivePath: active},
		Delivery: domain.StagedDelivery{ClientID: domain.ClientQoder, ActivePath: active},
	}
}

func argv(t *testing.T, commands [][]string, verb string) []string {
	t.Helper()
	for _, command := range commands {
		if len(command) > 2 && command[2] == verb {
			return command[1:]
		}
	}
	t.Fatalf("no %q command in %v", verb, commands)
	return nil
}

func TestQoderListRequiresUnambiguousNativeIdentity(t *testing.T) {
	for _, body := range []string{
		"null", `{}`, `[] {}`, `"demo@local"`,
		`[{"id":"demo@local"},{"id":"demo@local"}]`,
		`[{"id":"demo"}]`, `[{"id":"@local"}]`, `[{"id":"demo@"}]`, `[{"id":""}]`,
		`[{"id":"demo@local","version":null}]`, `[{"id":"demo@local","version":2}]`,
		`[{"id":"demo@local","ID":"demo@marketplace"}]`, `[{"ID":"demo@local"}]`,
		`[{"id":"demo@local","version":"1","version":"2"}]`,
		`[{"id":"demo@team@marketplace"}]`, `[{"id":"demo@@local"}]`, `[{"id":"demo@local@extra"}]`,
	} {
		if _, err := parseList([]byte(body)); !errors.Is(err, errUnknownList) {
			t.Errorf("accepted ambiguous listing %s", body)
		}
	}
	if entries, err := parseList([]byte(`[]`)); err != nil || len(entries) != 0 {
		t.Fatalf("empty listing: %v, %v", entries, err)
	}
}

func TestQoderListPreservesRecognizedNativeOutput(t *testing.T) {
	body := `[
		{"id":"demo@local","version":"2.0.0","status":"enabled"},
		{"id":"other@marketplace"}
	]`
	got, err := parseList([]byte(body))
	want := []pluginEntry{
		{ID: "demo@local", Version: "2.0.0"},
		{ID: "other@marketplace"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("recognized listing = %+v, %v; want %+v", got, err, want)
	}
}

func TestQoderActivationUpdateAndReadOnlyVerification(t *testing.T) {
	r := qoderRequest(t)
	f := &fakeQoder{}
	env := clients.Env{Runner: f}
	if out, err := New().Activate(context.Background(), env, r); err != nil || out.Verification != domain.VerificationInstalled {
		t.Fatalf("install: %+v %v", out, err)
	}
	if got := argv(t, f.commands, "install"); !reflect.DeepEqual(got, []string{"plugins", "install", r.Delivery.ActivePath, "--scope", "user"}) {
		t.Fatalf("install argv = %v", got)
	}
	before := len(f.commands)
	r.VerifyOnly = true
	if out, err := New().Activate(context.Background(), env, r); err != nil || out.Activation != domain.ActivationActive {
		t.Fatalf("verify: %+v %v", out, err)
	}
	if got := f.commands[before:]; len(got) != 1 || !reflect.DeepEqual(got[0][1:], []string{"plugins", "list", "--json"}) {
		t.Fatalf("verify mutated Qoder: %v", got)
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

func TestQoderVerifyKnownAndUnknownListJSON(t *testing.T) {
	r := qoderRequest(t)
	known := &fakeQoder{entries: []pluginEntry{{ID: "demo@local", Version: "2.0.0"}}}
	r.VerifyOnly = true
	if out, err := New().Activate(context.Background(), clients.Env{Runner: known}, r); err != nil || out.Activation != domain.ActivationActive || out.Verification != domain.VerificationInstalled {
		t.Fatalf("known listing: %+v %v", out, err)
	}
	unknown := &fakeQoder{unknownList: true}
	out, err := New().Activate(context.Background(), clients.Env{Runner: unknown}, r)
	if err != nil || out.Activation != domain.ActivationManual || out.ActivationAttested {
		t.Fatalf("unknown listing: %+v %v", out, err)
	}
	r.ActivationComplete = true
	out, err = New().Activate(context.Background(), clients.Env{Runner: unknown}, r)
	if err != nil || out.Activation != domain.ActivationActive || !out.ActivationAttested {
		t.Fatalf("attested unknown listing: %+v %v", out, err)
	}
	for _, command := range unknown.commands {
		if !reflect.DeepEqual(command[1:], []string{"plugins", "list", "--json"}) {
			t.Fatalf("unknown listing caused mutation: %v", unknown.commands)
		}
	}
}

func TestQoderRejectsForeignAndAbsentRegistration(t *testing.T) {
	r := qoderRequest(t)
	f := &fakeQoder{entries: []pluginEntry{{ID: "demo@marketplace", Version: "2.0.0"}}}
	if _, err := New().Activate(context.Background(), clients.Env{Runner: f}, r); err == nil {
		t.Fatal("foreign same-name plugin accepted")
	}
	if len(f.commands) != 1 {
		t.Fatalf("foreign plugin caused mutation: %v", f.commands)
	}
	f.entries = []pluginEntry{{ID: "demo@local", Version: "1.0.0"}}
	r.VerifyOnly = true
	if _, err := New().Activate(context.Background(), clients.Env{Runner: f}, r); err == nil {
		t.Fatal("different version verified as installed")
	}
}

func TestQoderMissingVersionCannotProveInstall(t *testing.T) {
	r := qoderRequest(t)
	r.VerifyOnly = true
	f := &fakeQoder{entries: []pluginEntry{{ID: "demo@local"}}}
	out, err := New().Activate(context.Background(), clients.Env{Runner: f}, r)
	if err != nil || out.Activation != domain.ActivationManual || out.Verification == domain.VerificationInstalled || out.ActivationAttested {
		t.Fatalf("missing version proved install: %+v %v", out, err)
	}
	if len(f.commands) != 1 {
		t.Fatalf("unproven version mutated Qoder: %v", f.commands)
	}
	r.ActivationComplete = true
	out, err = New().Activate(context.Background(), clients.Env{Runner: f}, r)
	if err != nil || out.Activation != domain.ActivationActive || !out.ActivationAttested {
		t.Fatalf("attested missing version: %+v %v", out, err)
	}
	r.ActivationComplete, r.VerifyOnly = false, false
	before := len(f.commands)
	out, err = New().Activate(context.Background(), clients.Env{Runner: f}, r)
	if err != nil || out.Activation != domain.ActivationManual || out.Verification == domain.VerificationInstalled {
		t.Fatalf("unproven version reinstall claim: %+v %v", out, err)
	}
	if len(f.commands) != before+1 {
		t.Fatalf("unproven version mutated Qoder: %v", f.commands[before:])
	}
}

func TestQoderForeignRegistrationOutranksAttestedUnprovenVersion(t *testing.T) {
	r := qoderRequest(t)
	r.VerifyOnly = true
	r.ActivationComplete = true
	f := &fakeQoder{entries: []pluginEntry{{ID: "demo@local"}, {ID: "demo@marketplace"}}}
	out, err := New().Activate(context.Background(), clients.Env{Runner: f}, r)
	if err == nil || out.Activation == domain.ActivationActive || out.ActivationAttested {
		t.Fatalf("attested unproven version overrode a foreign registration: %+v %v", out, err)
	}
	if len(f.commands) != 1 {
		t.Fatalf("foreign pair mutated Qoder: %v", f.commands)
	}
}

func TestQoderRejectsAmbiguousPluginNames(t *testing.T) {
	plan := domain.DeliveryPlan{ClientID: domain.ClientQoder, DeclaredName: "demo@team", Status: domain.PlanReady}
	if err := New().CheckPlanPrecondition(clients.PlanInput{}, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanUnsupported || plan.Activation != domain.ActivationFailed {
		t.Fatalf("ambiguous name planned: %+v", plan)
	}
	r := qoderRequest(t)
	r.DeclaredName, r.Plan.DeclaredName = "demo@team", "demo@team"
	f := &fakeQoder{entries: []pluginEntry{{ID: "demo@team@local", Version: "2.0.0"}}}
	if _, err := New().Activate(context.Background(), clients.Env{Runner: f}, r); err == nil || len(f.commands) != 0 {
		t.Fatalf("ambiguous name activated: %v", f.commands)
	}
	request := domain.DeactivationRequest{Client: r.Client, DeclaredName: "demo@team", BackendExecutable: r.BackendExecutable, Confirmed: true}
	if _, err := New().Deactivate(context.Background(), clients.Env{Runner: f}, request); err == nil || len(f.commands) != 0 {
		t.Fatalf("ambiguous name uninstalled: %v", f.commands)
	}
	plan = domain.DeliveryPlan{ClientID: domain.ClientQoder, DeclaredName: "demo", Status: domain.PlanReady}
	if err := New().CheckPlanPrecondition(clients.PlanInput{}, &plan); err != nil || plan.Status != domain.PlanReady {
		t.Fatalf("qualified name rejected: %+v %v", plan, err)
	}
}

func TestQoderDeactivation(t *testing.T) {
	r := qoderRequest(t)
	f := &fakeQoder{entries: []pluginEntry{{ID: "demo@local", Version: "2.0.0"}}}
	request := domain.DeactivationRequest{Client: r.Client, DeclaredName: "demo", BackendExecutable: r.BackendExecutable, ManagedArtifactPath: r.Delivery.ActivePath}
	if _, err := New().Deactivate(context.Background(), clients.Env{Runner: f}, request); err != nil || len(f.commands) != 0 {
		t.Fatal("unconfirmed removal touched Qoder")
	}
	request.Confirmed = true
	f.entries = []pluginEntry{{ID: "demo@local", Version: "2.0.0"}, {ID: "demo@marketplace"}}
	if _, err := New().Deactivate(context.Background(), clients.Env{Runner: f}, request); err == nil {
		t.Fatal("ambiguous name uninstalled")
	}
	if got := f.commands; len(got) != 1 {
		t.Fatalf("refusal mutated Qoder: %v", got)
	}
	f.entries = []pluginEntry{{ID: "demo@marketplace"}}
	if out, err := New().Deactivate(context.Background(), clients.Env{Runner: f}, request); err != nil || !out.ExternalRemovalComplete {
		t.Fatalf("foreign-only removal: %+v %v", out, err)
	}
	f.entries = []pluginEntry{{ID: "demo@local", Version: "2.0.0"}}
	before := len(f.commands)
	if out, err := New().Deactivate(context.Background(), clients.Env{Runner: f}, request); err != nil || !out.ExternalRemovalComplete {
		t.Fatalf("remove: %+v %v", out, err)
	}
	if got := f.commands[before+1]; !reflect.DeepEqual(got[1:], []string{"plugins", "uninstall", "demo"}) {
		t.Fatalf("uninstall argv = %v", got)
	}
}

func TestQoderInspectNativeRegistryFindings(t *testing.T) {
	r := qoderRequest(t)
	inspect := func(entries []pluginEntry, managed *domain.ClientBinding) clients.RegistryFinding {
		f := &fakeQoder{entries: entries}
		finding, err := New().InspectNativeRegistry(context.Background(), clients.Env{Runner: f}, r.Client, r.Plan, managed)
		if err != nil {
			t.Fatal(err)
		}
		return finding
	}
	managed := &domain.ClientBinding{ClientID: string(domain.ClientQoder)}
	if finding := inspect(nil, nil); finding != clients.RegistryClear {
		t.Fatalf("absent = %v", finding)
	}
	if finding := inspect([]pluginEntry{{ID: "demo@local"}}, managed); finding != clients.RegistryExpected {
		t.Fatalf("managed local install = %v", finding)
	}
	if finding := inspect([]pluginEntry{{ID: "demo@local"}}, nil); finding != clients.RegistryCollision {
		t.Fatalf("unmanaged local install = %v", finding)
	}
	if finding := inspect([]pluginEntry{{ID: "demo@marketplace"}}, managed); finding != clients.RegistryCollision {
		t.Fatalf("foreign claim = %v", finding)
	}
	f := &fakeQoder{unknownList: true}
	if finding, err := New().InspectNativeRegistry(context.Background(), clients.Env{Runner: f}, r.Client, r.Plan, managed); finding != clients.RegistryIndeterminate || !errors.Is(err, errUnknownList) {
		t.Fatalf("unknown listing = %v, %v", finding, err)
	}
	r.Plan.DeclaredName = "demo@team"
	f = &fakeQoder{}
	if finding, err := New().InspectNativeRegistry(context.Background(), clients.Env{Runner: f}, r.Client, r.Plan, managed); finding != clients.RegistryIndeterminate || err == nil {
		t.Fatalf("ambiguous name registry = %v, %v", finding, err)
	}
}

func TestQoderDetectSurfaces(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".qoder"), 0o755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "qoder")
	for _, present := range []bool{true, false} {
		probes := clients.HostProbes{HomeDir: home, GOOS: "linux", Lstat: os.Lstat, ReadDir: os.ReadDir, EvalSymlinks: filepath.EvalSymlinks,
			LookPath: func(name string) (string, error) {
				if name == "qoder" && present {
					return binary, nil
				}
				return "", fmt.Errorf("TEST absent")
			}}
		detection := New().DetectSurfaces(clients.NewHost(probes))
		want := ""
		if present {
			want = binary
		}
		if detection.Err != nil || detection.ExecutablePath != want || detection.ConfigRoot != filepath.Join(home, ".qoder") {
			t.Fatalf("binary present=%v: %+v", present, detection)
		}
		found := map[string]bool{}
		for _, surface := range detection.Surfaces {
			found[surface.ID] = surface.Detected
		}
		if found["qoder_cli"] != present || !found["qoder_config"] {
			t.Fatalf("surfaces present=%v: %+v", present, detection.Surfaces)
		}
	}
}
