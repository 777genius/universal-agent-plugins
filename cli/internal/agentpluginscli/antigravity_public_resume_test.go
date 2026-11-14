package agentpluginscli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type antigravityPublicRunner struct {
	t                        *testing.T
	home, config, executable string
	active                   string
	mcpServers               []string
	calls                    int
	disabled                 bool
}

func (r *antigravityPublicRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.t.Helper()
	if command.Dir != r.config || !reflect.DeepEqual(command.Env, []string{"HOME=" + r.home, "USERPROFILE=" + r.home}) {
		r.t.Fatalf("inventory changed the selected profile: %+v", command)
	}
	r.calls++
	if reflect.DeepEqual(command.Argv, []string{r.executable, "--version"}) {
		return legacyports.CommandResult{Stdout: []byte("1.5.3\n"), CleanupComplete: true}, nil
	}
	if !reflect.DeepEqual(command.Argv, []string{r.executable, "plugin", "list", "--json"}) {
		r.t.Fatalf("unregistered inert command: %+v", command)
	}
	body, err := json.Marshal([]any{map[string]any{"name": filepath.Base(r.active), "path": r.active,
		"version": readCLIObject(r.t, filepath.Join(r.active, "plugin.json"))["version"], "enabled": !r.disabled, "skills": []string{"guide"}, "mcpServers": r.mcpServers}})
	return legacyports.CommandResult{Stdout: body, CleanupComplete: true}, err
}

type antigravityPublicFixture struct {
	cliFixture
	client domain.DetectedClient
	source string
	runner *antigravityPublicRunner
}

func newAntigravityPublicFixture(t *testing.T) antigravityPublicFixture {
	t.Helper()
	return newAntigravityPublicFixtureWithMCP(t, false)
}

func newAntigravityPublicFixtureWithMCP(t *testing.T, includeMCP bool) antigravityPublicFixture {
	t.Helper()
	client := fixtureClient(t, domain.ClientAntigravity)
	home := filepath.Dir(client.ConfigRoot)
	client.ConfigRoot = filepath.Join(home, ".gemini", "config")
	client.NativeConfigRoot, client.ExecutablePath = client.ConfigRoot, filepath.Join(home, "inert-agy")
	client.Version = "diagnostic metadata is not live version authority"
	if err := os.MkdirAll(client.ConfigRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(client.ExecutablePath, []byte("inert fixture; never executed"), 0700); err != nil {
		t.Fatal(err)
	}
	f := newCLIFixture(t, []domain.DetectedClient{client})
	runner := &antigravityPublicRunner{t: t, home: home, config: client.ConfigRoot, executable: client.ExecutablePath, mcpServers: []string{}}
	f.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{Runner: runner})
	f.app.Lifecycle.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Runner: runner, Stager: f.app.Lifecycle.Stager})
	source := writeCLIPlugin(t)
	if includeMCP {
		writeCLIMCP(t, source)
		runner.mcpServers = []string{"demo"}
	}
	if err := os.MkdirAll(filepath.Join(source, "skills", "guide"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "skills", "guide", "SKILL.md"), []byte("---\nname: guide\ndescription: Public lifecycle fixture\n---\nRead-only guide.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if stdout, stderr, err := f.execute(false, "add", source, "--target", "antigravity", "--no-save", "--format", "json"); err != nil {
		t.Fatalf("initial public add: %v\n%s\n%s", err, stdout, stderr)
	}
	state, err := f.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatal("initial installation", state, err)
	}
	binding := onlyCLIClient(state.Installations[0])
	if runner.calls != 0 || binding.Activation != domain.ActivationPrepared {
		t.Fatal("initial file delivery started native bootstrap", runner.calls, binding)
	}
	runner.active = binding.TargetLocator
	config, err := json.Marshal(map[string]any{"plugins": map[string]any{filepath.Base(runner.active): map[string]any{"enabled": true}}})
	if err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string][]byte{"config.json": config, "foreign.json": []byte("unchanged foreign profile bytes\n")} {
		if err = os.WriteFile(filepath.Join(client.ConfigRoot, path), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return antigravityPublicFixture{cliFixture: f, client: client, source: source, runner: runner}
}

func (f antigravityPublicFixture) historical(t *testing.T) domain.StateFileV2 {
	t.Helper()
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyCLIClient(state.Installations[0])
	binding.Activation, binding.Verification, binding.Authentication = domain.ActivationActive, domain.VerificationInstalled, domain.AuthenticationComplete
	state.Installations[0].Clients[binding.ClientBindingID] = binding
	if err = f.store.Save(state); err != nil {
		t.Fatal(err)
	}
	return state
}

func antigravityPublicBytes(t *testing.T, f antigravityPublicFixture) map[string]string {
	t.Helper()
	files := []string{filepath.Join(f.client.ConfigRoot, "config.json"), filepath.Join(f.client.ConfigRoot, "foreign.json"),
		filepath.Join(f.runner.active, "plugin.json"), filepath.Join(f.runner.active, "skills", "guide", "SKILL.md")}
	if len(f.runner.mcpServers) != 0 {
		files = append(files, filepath.Join(f.runner.active, "mcp_config.json"))
	}
	result := map[string]string{}
	for _, file := range files {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		result[file] = string(body)
	}
	return result
}

// These execute the public Cobra dispatch and real providers with an inert
// runner. They prove caller sequencing, not native vendor qualification.
func TestAntigravityPublicHistoricalAddReentersConfirmedResume(t *testing.T) {
	for _, mode := range []string{"json", "yes", "tty"} {
		t.Run(mode, func(t *testing.T) {
			f := newAntigravityPublicFixture(t)
			before := f.historical(t)
			files := antigravityPublicBytes(t, f)
			args := []string{"add", f.source, "--target", "antigravity", "--no-save"}
			switch mode {
			case "json":
				args = append(args, "--format", "json")
			case "yes":
				args = append(args, "--yes")
			}
			stdout, stderr, err := f.execute(mode != "json", args...)
			if err != nil || f.runner.calls != 2 {
				t.Fatalf("public %s resume: calls=%d err=%v\n%s\n%s", mode, f.runner.calls, err, stdout, stderr)
			}
			after, err := f.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			old, current := onlyCLIClient(before.Installations[0]), onlyCLIClient(after.Installations[0])
			if current.Activation != domain.ActivationPrepared || current.Verification != domain.VerificationInstalled || current.Authentication != domain.AuthenticationComplete ||
				!reflect.DeepEqual(old.Receipts, current.Receipts) || !reflect.DeepEqual(old.PackageRevision, current.PackageRevision) ||
				!reflect.DeepEqual(before.TransactionReceipts, after.TransactionReceipts) || !reflect.DeepEqual(files, antigravityPublicBytes(t, f)) {
				t.Fatal("public resume changed ownership/authentication or promoted activation", current)
			}
			if strings.Contains(stdout, "Have you completed activation") || strings.Contains(stdout, "Have you completed required authentication") {
				t.Fatal("native observation was replaced with attestation", stdout)
			}
		})
	}
}

func TestAntigravityPublicPreviewDoesNotExecuteOrPersist(t *testing.T) {
	f := newAntigravityPublicFixture(t)
	before := f.historical(t)
	for _, verb := range []string{"add", "repair"} {
		selector := "demo"
		if verb == "add" {
			selector = f.source
		}
		if stdout, stderr, err := f.execute(false, verb, selector, "--target", "antigravity", "--dry-run", "--format", "json"); err != nil {
			t.Fatalf("public %s preview: %v\n%s\n%s", verb, err, stdout, stderr)
		}
	}
	loaded, err := f.app.loadPackage(context.Background(), f.source)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.cleanup != nil {
		t.Cleanup(func() {
			if err := loaded.cleanup(); err != nil {
				t.Error(err)
			}
		})
	}
	service := lifecycleService(f.app, map[domain.ClientID]domain.DetectedClient{f.client.ClientID: f.client})
	preview, err := service.Add(context.Background(), usecase.AddInput{Envelope: loaded.envelope, Client: f.client,
		Scope: domain.ScopeUser, BackendExecutable: f.client.ExecutablePath})
	if err != nil || !preview.RequiresConfirmation || preview.NoChange || preview.Mutated || f.runner.calls != 0 {
		t.Fatal("unconfirmed observation did not remain pending and inert", preview, err, f.runner.calls)
	}
	after, err := f.store.Load()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("preview persisted lifecycle state", after, err)
	}
}

func TestAntigravityPublicDisabledRepeatPreservesOriginalState(t *testing.T) {
	f := newAntigravityPublicFixtureWithMCP(t, true)
	f.runner.disabled = true
	body, err := json.Marshal(map[string]any{"plugins": map[string]any{filepath.Base(f.runner.active): map[string]any{"enabled": false}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(f.client.ConfigRoot, "config.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(f.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(f.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	files := antigravityPublicBytes(t, f)
	stdout, stderr, err := f.execute(false, "add", f.source, "--target", "antigravity", "--no-save", "--format", "json")
	if err != nil || f.runner.calls != 2 || !strings.Contains(stdout, "remains disabled") {
		t.Fatalf("disabled public observation: calls=%d err=%v\n%s\n%s", f.runner.calls, err, stdout, stderr)
	}
	var output struct {
		Data struct {
			Result usecase.AddResult `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil || output.Data.Result.Mutated ||
		output.Data.Result.Activation.Activation != domain.ActivationPrepared {
		t.Fatal("disabled observation changed state or promoted activation", stdout, err)
	}
	after, err := os.ReadFile(f.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	current, err := os.Stat(f.store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) || !os.SameFile(original, current) || original.Mode() != current.Mode() ||
		original.ModTime() != current.ModTime() || !reflect.DeepEqual(files, antigravityPublicBytes(t, f)) {
		t.Fatal("disabled observation changed original state or owned/foreign native bytes")
	}
}

func TestAntigravityPublicIntactRepairUsesExistingGroupActivation(t *testing.T) {
	// Public dispatch and the real Stager run here; native inventory remains inert.
	f := newAntigravityPublicFixtureWithMCP(t, true)
	manifest := readCLIObject(t, filepath.Join(f.source, "plugin.json"))
	manifest["version"] = "2.0.0"
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.source, "plugin.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := f.execute(false, "update", "demo", "--target", "antigravity", "--format", "json")
	if err != nil || f.runner.calls == 0 {
		t.Fatalf("public changed MCP update: calls=%d err=%v\n%s\n%s", f.runner.calls, err, stdout, stderr)
	}
	assertBatchJSON(t, stdout, "update", 1, 0)
	if readCLIObject(t, filepath.Join(f.runner.active, "plugin.json"))["version"] != "2.0.0" {
		t.Fatal("public update did not replace the delivered revision")
	}
	servers := readCLIObject(t, filepath.Join(f.runner.active, "mcp_config.json"))["mcpServers"].(map[string]any)
	if len(servers) != 1 || servers["demo"] == nil {
		t.Fatal("public producer did not deliver the selected native MCP", servers)
	}
	if _, err := os.Lstat(filepath.Join(f.runner.active, "mcp.json")); !os.IsNotExist(err) {
		t.Fatal("native producer retained portable mcp.json", err)
	}
	f.historical(t)
	files := antigravityPublicBytes(t, f)
	calls := f.runner.calls
	stdout, stderr, err = f.execute(false, "repair", "demo", "--target", "antigravity", "--yes", "--format", "json")
	if err != nil || f.runner.calls != calls+2 {
		t.Fatalf("public grouped repair: calls=%d err=%v\n%s\n%s", f.runner.calls, err, stdout, stderr)
	}
	var output struct {
		Data struct {
			Batch bool `json:"batch"`
		} `json:"data"`
	}
	if err = json.Unmarshal([]byte(stdout), &output); err != nil || !output.Data.Batch {
		t.Fatal("public repair bypassed grouped dispatch", stdout, err)
	}
	state, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := onlyCLIClient(state.Installations[0])
	if binding.Activation != domain.ActivationPrepared || binding.Authentication != domain.AuthenticationComplete || !reflect.DeepEqual(files, antigravityPublicBytes(t, f)) {
		t.Fatal("group repair changed selected profile or promoted activation", binding)
	}
}
