package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
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
	absent                   bool
	cleanupFailure           bool
}

func (r *antigravityPublicRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.t.Helper()
	if command.Dir != r.config || !reflect.DeepEqual(command.Env, []string{"HOME=" + r.home, "USERPROFILE=" + r.home}) {
		r.t.Fatalf("inventory changed the selected profile: %+v", command)
	}
	r.calls++
	if reflect.DeepEqual(command.Argv, []string{r.executable, "--version"}) {
		return legacyports.CommandResult{Stdout: []byte("1.5.3\n"), CleanupComplete: !r.cleanupFailure}, nil
	}
	if !reflect.DeepEqual(command.Argv, []string{r.executable, "plugin", "list", "--json"}) {
		r.t.Fatalf("unregistered inert command: %+v", command)
	}
	if r.absent {
		return legacyports.CommandResult{Stdout: []byte("[]"), CleanupComplete: true}, nil
	}
	body, err := json.Marshal([]any{map[string]any{"name": filepath.Base(r.active), "path": r.active,
		"version": readCLIObject(r.t, filepath.Join(r.active, "plugin.json"))["version"], "enabled": !r.disabled, "skills": []string{"guide"}, "mcpServers": r.mcpServers}})
	return legacyports.CommandResult{Stdout: body, CleanupComplete: true}, err
}

type antigravityPublicFixture struct {
	cliFixture
	client         domain.DetectedClient
	source         string
	sourceArtifact string
	runner         *antigravityPublicRunner
}

func newAntigravityPublicFixture(t *testing.T) antigravityPublicFixture {
	t.Helper()
	return newAntigravityPublicFixtureWithMCP(t, false)
}

func newAntigravityPublicFixtureWithMCP(t *testing.T, includeMCP bool) antigravityPublicFixture {
	t.Helper()
	return newAntigravityPublicFixtureWithSaved(t, includeMCP, false, nil)
}

func newAntigravityPublicFixtureWithSaved(t *testing.T, includeMCP, saved bool, agreement map[string][]byte) antigravityPublicFixture {
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
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := newCLIFixtureAt(t, []domain.DetectedClient{client}, root)
	runner := &antigravityPublicRunner{t: t, home: home, config: client.ConfigRoot, executable: client.ExecutablePath, mcpServers: []string{}}
	f.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{Runner: runner})
	f.app.Lifecycle.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Runner: runner, Stager: f.app.Lifecycle.Stager})
	if saved {
		configureAntigravityOwnedSDK(t, &f, runner)
	}
	source := writeCLIPlugin(t)
	if includeMCP {
		writeCLIMCP(t, source)
		runner.mcpServers = []string{"demo"}
		if saved {
			dependencyWrite(t, filepath.Join(source, "mcp.json"), "{\"$schema\":\"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json\",\"mcpServers\":{\"demo\":{\"type\":\"stdio\",\"command\":\"go\",\"args\":[\"version\"],\"env\":{\"DATA\":\"${PLUGIN_DATA}\"}}}}")
		}
	}
	if err := os.MkdirAll(filepath.Join(source, "skills", "guide"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "skills", "guide", "SKILL.md"), []byte("---\nname: guide\ndescription: Public lifecycle fixture\n---\nRead-only guide.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	sourceArtifact := source
	if saved {
		sourceArtifact = filepath.Join(t.TempDir(), "owned-source.zip")
		payload := directArchiveBytes(t, source, "zip", "")
		if err := os.WriteFile(sourceArtifact, payload, 0600); err != nil {
			t.Fatal(err)
		}
		source = "https://fixture.invalid/owned-source.zip"
		f.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: f.root, HTTPClient: &http.Client{Transport: cliRoundTripper(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet || request.URL.String() != source || request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" {
				t.Fatal("unexpected owned archive fixture request")
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/zip"}}, Body: io.NopCloser(bytes.NewReader(payload)), ContentLength: int64(len(payload)), Request: request}, nil
		})}}
		args := []string{"add", source, "--target", "antigravity", "--scope", "user", "--yes", "--format", "json"}
		if agreement != nil {
			antigravityCopyColdAgreement(t, f, agreement)
			args = []string{"install", "--locked", "--scope", "user", "--yes", "--format", "json"}
		}
		stdout, stderr, err := f.execute(false, args...)
		if err != nil {
			t.Fatalf("public saved-owner installation: %v\n%s\n%s", err, stdout, stderr)
		}
	} else if stdout, stderr, err := f.execute(false, "add", source, "--target", "antigravity", "--no-save", "--format", "json"); err != nil {
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
	return antigravityPublicFixture{cliFixture: f, client: client, source: source, sourceArtifact: sourceArtifact, runner: runner}
}

func antigravityCopyColdAgreement(t *testing.T, f cliFixture, agreement map[string][]byte) {
	t.Helper()
	if len(agreement) != 2 || len(agreement["uap.toml"]) == 0 || len(agreement["uap.lock"]) == 0 {
		t.Fatal("cold fixture requires exactly the two original public agreement files")
	}
	for _, path := range []string{f.store.Path, f.app.EnvironmentStoreRoot, f.app.ManagedRoot, filepath.Join(f.app.DependenciesRoot, "plugin-data")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("cold fixture already has state, store, delivery or DATA", path, err)
		}
	}
	if err := os.MkdirAll(f.app.DependenciesRoot, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range agreement {
		if err := os.WriteFile(filepath.Join(f.app.DependenciesRoot, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
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

// These saved-owner cases use the actual source acquisition, Cobra, SDK and
// transaction kernel. The existing scanner and selected runner are inert fixtures;
// their successful dispatch is not native runtime or security qualification.
func configureAntigravityOwnedSDK(t *testing.T, f *cliFixture, runner *antigravityPublicRunner) {
	t.Helper()
	root := filepath.Dir(f.store.Path)
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f.app.Installer, err = installer.New(installer.Config{
		StateRoot: root, StateFile: f.store.Path, LockFile: filepath.Join(root, "mutation.lock"),
		OperationsDir: f.operations, PluginDataBase: filepath.Join(root, "plugin-data"),
		ManagedRoot: f.app.ManagedRoot, TempRoot: filepath.Join(root, "installer-tmp"),
		Registry: f.app.ClientRegistry, HelperExecutable: helper, Runner: runner, EnableNativeObserver: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	f.app.DependenciesRoot = root
	f.app.EnvironmentLockPath = filepath.Join(root, "environment.lock")
	f.app.EnvironmentStoreRoot = filepath.Join(root, "store")
	f.app.SecurityEvaluator = dependencyTestScanner{}
}

func antigravityOwnedFixture(t *testing.T) (antigravityPublicFixture, domain.Installation, domain.ClientBinding, domain.DataReceipt) {
	t.Helper()
	f := newAntigravityPublicFixtureWithSaved(t, true, true, nil)
	installed, binding, data := antigravityOwnedFacts(t, f)
	return f, installed, binding, data
}

func antigravityOwnedFacts(t *testing.T, f antigravityPublicFixture) (domain.Installation, domain.ClientBinding, domain.DataReceipt) {
	t.Helper()
	state, err := f.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 1 {
		t.Fatal("fixture lacks one genuine saved installation", err)
	}
	installed, receipt := state.Installations[0], state.EnvironmentReceipts[0]
	binding := onlyCLIClient(installed)
	if receipt.EnvironmentID != environment.EnvironmentUserDefault || receipt.Alias != "demo" || receipt.ClientID != "antigravity" ||
		receipt.InstallationID != installed.InstallationID || receipt.ClientBindingID != binding.ClientBindingID ||
		receipt.Phase != "committed" || receipt.SourceKey != installed.Source.CanonicalSource || receipt.PackageDigest != binding.PackageRevision.TreeDigest {
		t.Fatal("kernel-authored saved-owner identity differs")
	}
	data, ok := installed.DataReceipts[binding.DataReceiptID]
	if !ok || len(installed.DataReceipts) != 1 || data.State != domain.DataReceiptOwned ||
		binding.Activation != domain.ActivationPrepared || binding.Authentication != domain.AuthenticationNotChecked {
		t.Fatal("fixture lacks exact Prepared User and retained DATA ownership")
	}
	if err := f.app.Lifecycle.PluginData.ValidateData(t.Context(), data); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filepath.Join(data.Locator, "retained-marker.txt"), "original owned DATA\n")
	return installed, binding, data
}

func antigravityOwnedPreservation(t *testing.T, f antigravityPublicFixture, data domain.DataReceipt, extra ...string) func() {
	t.Helper()
	manifest, lock := f.app.dependencyPaths()
	roots := append([]string{f.store.Path, f.sourceArtifact, f.client.ConfigRoot, data.Locator, f.app.EnvironmentStoreRoot, manifest, lock}, extra...)
	originals := map[string]map[string]dependencyCheckFileObservation{}
	for _, root := range roots {
		originals[root] = observeDependencyCheckFiles(t, root)
	}
	return func() {
		t.Helper()
		for root, before := range originals {
			assertDependencyCheckFilesUnchanged(t, root, before)
		}
	}
}

func assertAntigravityOwnedRepairDTO(t *testing.T, body string, installed domain.Installation, binding domain.ClientBinding, guidance string) {
	t.Helper()
	var result struct {
		SchemaVersion int                   `json:"schema_version"`
		Command       string                `json:"command"`
		Result        string                `json:"result"`
		Outcome       string                `json:"outcome"`
		Scope         string                `json:"scope"`
		ScopeReason   string                `json:"scope_selection_reason"`
		Actions       []dependencyActionDTO `json:"actions"`
		Results       []dependencyResultDTO `json:"results"`
	}
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		t.Fatal("invalid actual repair DTO", err, body)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatal("repair DTO contains trailing output", err)
	}
	if result.SchemaVersion != 1 || result.Command != "agentplugins repair" || result.Result != "success" ||
		result.Outcome != "completed" || result.Scope != "user" || result.ScopeReason != "explicit" ||
		len(result.Actions) != 1 || len(result.Results) != 1 {
		t.Fatal("saved repair DTO changed command, scope or result count", body)
	}
	action, outcome, revision := result.Actions[0], result.Results[0], binding.PackageRevision
	if action.Alias != "demo" || action.ClientID != "antigravity" || action.InstallationID != installed.InstallationID ||
		action.BindingID != binding.ClientBindingID || action.Action != "repair" || action.Reason != "" ||
		action.FromVersion != revision.Version || action.ToVersion != revision.Version ||
		action.FromRevision != revision.ResolvedRevision || action.Revision != revision.ResolvedRevision ||
		action.FromDigest != revision.TreeDigest || action.PackageDigest != revision.TreeDigest || len(action.ComponentSupport) != 2 {
		t.Fatal("saved repair DTO lost exact owner, source or revision", body)
	}
	components := map[string]bool{}
	for _, component := range action.ComponentSupport {
		key := string(component.Kind) + "/" + component.Name
		if components[key] || component.Support != domain.SupportNative {
			t.Fatal("saved repair DTO contains duplicate or unsupported components", body)
		}
		components[key] = true
	}
	if !components[string(domain.ComponentSkill)+"/guide"] || !components[string(domain.ComponentMCPServer)+"/demo"] ||
		outcome.InstallationID != installed.InstallationID || outcome.ClientID != "antigravity" || outcome.Operation != "repair" ||
		outcome.Outcome != "unchanged" || outcome.Reason != "" || len(outcome.NextActions) != 0 ||
		!reflect.DeepEqual(outcome.ManualActions, []string{guidance}) {
		t.Fatal("saved repair DTO lacks exact transient native inventory evidence", body)
	}
}

func TestAntigravityOwnedIntactRepairReachesReadOnlyInventory(t *testing.T) {
	f, installed, binding, data := antigravityOwnedFixture(t)
	unchanged := antigravityOwnedPreservation(t, f, data)
	_, _, err := f.execute(false, "add", f.source, "--target", "antigravity", "--no-save", "--scope", "user", "--yes", "--format", "json")
	if err == nil || err.Error() != "source belongs to saved dependency demo; use agentplugins add without --no-save or agentplugins update demo" || f.runner.calls != 0 {
		t.Fatal("ownerless public add did not refuse before native invocation", err, f.runner.calls)
	}
	unchanged()
	loaded, err := f.app.loadPackage(t.Context(), f.source)
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
	_, err = service.Add(t.Context(), usecase.AddInput{Envelope: loaded.envelope, Client: f.client, Scope: domain.ScopeUser,
		Hints: loaded.hints, BackendExecutable: f.client.ExecutablePath, Confirmed: true})
	if !errors.Is(err, transaction.ErrEnvironmentAuthority) || f.runner.calls != 0 {
		t.Fatal("kernel owner guard did not refuse before native invocation", err, f.runner.calls)
	}
	unchanged()
	for _, selector := range []string{"", installed.InstallationID} {
		args := []string{"doctor"}
		if selector != "" {
			args = append(args, selector)
		}
		stdout, stderr, err := f.execute(false, append(args, "--scope", "user", "--format", "json")...)
		var report struct {
			SchemaVersion int    `json:"schema_version"`
			Command       string `json:"command"`
			Result        string `json:"result"`
			Data          struct {
				ReadOnly bool `json:"read_only"`
			} `json:"data"`
		}
		decodeErr := json.Unmarshal([]byte(stdout), &report)
		if err != nil || stderr != "" || f.runner.calls != 0 || decodeErr != nil ||
			report.SchemaVersion != 1 || report.Command != "doctor" || report.Result != "success" || !report.Data.ReadOnly {
			t.Fatal("doctor control unexpectedly observed native inventory", err, decodeErr, stdout, stderr)
		}
		unchanged()
	}
	for repetition := 1; repetition <= 2; repetition++ {
		stdout, stderr, err := f.execute(false, "repair", installed.InstallationID, "--target", "antigravity", "--scope", "user", "--yes", "--format", "json")
		if err != nil || stderr != "" || f.runner.calls != 2*repetition {
			t.Fatalf("saved repair did not dispatch exactly version and inventory: calls=%d error=%v\n%s\n%s", f.runner.calls, err, stdout, stderr)
		}
		unchanged()
		assertAntigravityOwnedRepairDTO(t, stdout, installed, binding, "The selected User plugin is enabled in CLI inventory; open or reload Antigravity to verify IDE loading")
		t.Logf("actual saved repair DTO: %s", stdout)
	}
}

func TestAntigravityPublicColdLockedRestoreRetainsOriginalSavedRepairAuthority(t *testing.T) {
	original, installed, _, data := antigravityOwnedFixture(t)
	originalUnchanged := antigravityOwnedPreservation(t, original, data)
	agreement := map[string][]byte{}
	for _, name := range []string{"uap.toml", "uap.lock"} {
		body, err := os.ReadFile(filepath.Join(original.app.DependenciesRoot, name))
		if err != nil {
			t.Fatal(err)
		}
		agreement[name] = body
	}
	cold := newAntigravityPublicFixtureWithSaved(t, true, true, agreement)
	restored, binding, retained := antigravityOwnedFacts(t, cold)
	if cold.client.ConfigRoot == original.client.ConfigRoot || cold.store.Path == original.store.Path || retained.Locator == data.Locator ||
		!reflect.DeepEqual(restored.Source, installed.Source) || !reflect.DeepEqual(restored.Package, installed.Package) {
		t.Fatal("cold public install copied authority or changed the locked source/package")
	}
	for name, body := range agreement {
		got, err := os.ReadFile(filepath.Join(cold.app.DependenciesRoot, name))
		if err != nil || !bytes.Equal(body, got) {
			t.Fatal("cold install changed the only transferred agreement", name, err)
		}
	}
	unchanged := antigravityOwnedPreservation(t, cold, retained)
	stdout, stderr, err := cold.execute(false, "install", "--locked", "--scope", "user", "--yes", "--format", "json")
	if err != nil || cold.runner.calls != 0 {
		t.Fatal("locked no-op was mistaken for native observation", err, cold.runner.calls, stdout, stderr)
	}
	unchanged()
	originalUnchanged()
	for repetition := 1; repetition <= 2; repetition++ {
		stdout, stderr, err = cold.execute(false, "repair", restored.InstallationID, "--target", "antigravity", "--scope", "user", "--yes", "--format", "json")
		if err != nil || stderr != "" || cold.runner.calls != 2*repetition {
			t.Fatal("cold saved repair did not observe the original selected profile", err, cold.runner.calls, stdout, stderr)
		}
		assertAntigravityOwnedRepairDTO(t, stdout, restored, binding, "The selected User plugin is enabled in CLI inventory; open or reload Antigravity to verify IDE loading")
		unchanged()
		originalUnchanged()
	}
}

func TestAntigravityOwnedRepairKeepsNegativeObservationAndConsentBoundaries(t *testing.T) {
	for _, condition := range []string{"disabled", "native-absent", "config-absent", "cleanup-unproved", "dry-run", "unconfirmed", "missing-provenance", "foreign-provenance"} {
		t.Run(condition, func(t *testing.T) {
			f, installed, binding, data := antigravityOwnedFixture(t)
			var extra []string
			wantCalls, wantError := 0, false
			switch condition {
			case "disabled":
				f.runner.disabled, wantCalls = true, 2
				body, err := json.Marshal(map[string]any{"plugins": map[string]any{filepath.Base(f.runner.active): map[string]any{"enabled": false}}})
				if err != nil {
					t.Fatal(err)
				}
				dependencyWrite(t, filepath.Join(f.client.ConfigRoot, "config.json"), string(body))
			case "native-absent":
				f.runner.absent, wantCalls = true, 2
			case "config-absent":
				wantError = true
				if err := os.Remove(filepath.Join(f.client.ConfigRoot, "config.json")); err != nil {
					t.Fatal(err)
				}
			case "cleanup-unproved":
				f.runner.cleanupFailure, wantCalls, wantError = true, 1, true
			case "missing-provenance":
				f.client.NativeConfigRoot = ""
			case "foreign-provenance":
				foreign := filepath.Join(t.TempDir(), ".gemini", "config")
				if err := os.MkdirAll(foreign, 0700); err != nil {
					t.Fatal(err)
				}
				dependencyWrite(t, filepath.Join(foreign, "sentinel.json"), "{\"foreign\":true}\n")
				f.client.NativeConfigRoot, extra = foreign, []string{foreign}
			}
			f.app.Detector = staticDetector{clients: []domain.DetectedClient{f.client}}
			unchanged := antigravityOwnedPreservation(t, f, data, extra...)
			args := []string{"repair", installed.InstallationID, "--target", "antigravity", "--scope", "user", "--yes", "--format", "json"}
			terminal, input := false, ""
			switch condition {
			case "dry-run":
				args = append(args, "--dry-run")
			case "unconfirmed":
				// JSON automation and an explicit target already imply consent.
				// Exercise the existing human confirmation path and decline it.
				args = []string{"repair", installed.InstallationID, "--scope", "user", "--format", "human"}
				terminal, input = true, "n\n"
			}
			stdout, stderr, err := f.executeInput(terminal, input, args...)
			if (err != nil) != wantError || f.runner.calls != wantCalls {
				t.Fatalf("negative saved repair crossed consent/profile/cleanup boundary: calls=%d error=%v\n%s\n%s", f.runner.calls, err, stdout, stderr)
			}
			unchanged()
			if condition == "disabled" {
				assertAntigravityOwnedRepairDTO(t, stdout, installed, binding, "The selected User plugin remains disabled; decide whether to enable it in Antigravity")
			} else if condition == "native-absent" {
				assertAntigravityOwnedRepairDTO(t, stdout, installed, binding, "The exact UAP-owned plugin is absent from native User inventory; inspect it in Antigravity")
			} else if strings.Contains(stdout, "enabled in CLI inventory") || strings.Contains(stdout, "remains disabled") {
				t.Fatal("unobserved repair manufactured native inventory evidence", stdout)
			}
		})
	}
}
