package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Trust is an explicit fixture input in the original disposable user profile.
// The separate untrusted project test remains the installer trust-boundary gate.
func TestAgentpluginsCodexNativeTrustedProjectMCP(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CODEX_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("opt-in source trusted project MCP execution")
	}
	nativeRequireDisposable(t)
	if _, err := os.Lstat("/etc/codex"); !os.IsNotExist(err) {
		t.Fatal("system Codex config must be absent in disposable image")
	}
	client := nativeBinary(t, "AGENTPLUGINS_CODEX_BIN")
	if want := os.Getenv("AGENTPLUGINS_CODEX_SHA256"); len(want) != 64 || nativeSHA(t, client) != want {
		t.Fatal("Codex binary pin mismatch")
	}
	f := &codexProjectNativeFixture{base: newNativeFixture(t, true), client: client, installer: nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN"), unchanged: map[string]string{}}
	f.base.ClientBinDir, f.otherProject = filepath.Dir(client), filepath.Join(f.base.Root, "project-B")
	for _, project := range []string{f.base.Project, f.otherProject} {
		if err := os.MkdirAll(filepath.Join(project, ".git"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	provider := nativeScriptedProvider(t, f.base)
	config := fmt.Sprintf("cli_auth_credentials_store = \"file\"\nmcp_oauth_credentials_store = \"file\"\nmodel_provider = \"fixture\"\nmodel = \"fixture-model\"\n[model_providers.fixture]\nname = \"Fixture\"\nbase_url = %q\nwire_api = \"responses\"\nrequires_openai_auth = false\n[analytics]\nenabled = false\n[feedback]\nenabled = false\n[projects.%q]\ntrust_level = \"trusted\"\n[projects.%q]\ntrust_level = \"trusted\"\n", provider, f.base.Project, f.otherProject)
	f.foreign(t, filepath.Join(f.base.CodexHome, "config.toml"), config)
	f.foreign(t, filepath.Join(f.otherProject, ".codex", "config.toml"), "# preserve control project\nmodel = 'fixture-model'\n")
	f.foreign(t, filepath.Join(f.base.Project, "foreign-keep.txt"), "preserve project foreign bytes\n")
	nativeJSON(t, filepath.Join(f.base.Root, "trust-fixture.json"), map[string]any{"authored_before_baseline": true, "user_config": config, "trusted_projects": []string{f.base.Project, f.otherProject}, "production_trust_mutation": false})
	f.base.writePackage(t, "A", "1.0.0")
	stages := map[string]nativeStage{}
	for _, name := range []string{"install", "native_project_mcp", "unchanged", "remove", "scope_preservation", "retained_data"} {
		stages[name] = nativeStage{Status: "not_evaluated", Reason: "not reached"}
	}
	stages["model_use"] = nativeStage{Status: "not_evaluated", Reason: "no turn/start; direct MCP RPC only"}
	stages["update_repair_runtime"] = nativeStage{Status: "not_evaluated", Reason: "changed update/missing repair are covered by the separate component suite; this test qualifies add/replay/removal MCP only"}
	evidence := f.evidence(t, stages)
	evidence["project_contract"], evidence["native_surface"] = "trusted_project_mcp_v1", "explicit fixture trust; native thread MCP initialization/direct stdio tools; no model turn/authentication/external service or network-isolation claim"
	defer func() {
		evidence["captured_model_provider_requests"] = len(f.base.ProviderRequests)
		f.writeEvidence(t, evidence)
		if err := nativeStageFailures(stages); err != nil {
			t.Error(err)
		}
	}()
	codexProjectMCPSession(t, f, f.otherProject, "control-before-add", nil, "", false)
	nativeAttempt(stages, "install", "trusted-add-installer.json")
	added := f.run(t, "trusted-add", "add", f.base.Package, "--no-save")
	codexNativeProjectPrepared(t, added)
	binding, data := codexProjectMCPBinding(t, f)
	stages["install"] = nativeStage{Status: "passed", Reason: "public project add recorded exact Prepared package, MCP and data ownership"}
	state := filepath.Join(f.base.Project, ".uap", "state-v2.json")
	projectConfig := filepath.Join(f.base.Project, ".codex", "config.toml")
	beforeState, beforeConfig := nativeSHA(t, state), nativeSHA(t, projectConfig)
	requireStable := func() {
		t.Helper()
		if nativeSHA(t, state) != beforeState || nativeSHA(t, projectConfig) != beforeConfig {
			t.Fatal("unchanged project operation/native session rewrote Prepared state or config")
		}
	}
	nativeAttempt(stages, "native_project_mcp", "trusted-add-rpc.jsonl")
	codexProjectMCPSession(t, f, f.base.Project, "trusted-add", &binding, data, true)
	requireStable()
	stages["native_project_mcp"] = nativeStage{Status: "passed", Reason: "exact project configuration servers initialized and direct nonce-correlated stdio tools returned bound package/data/cwd/args"}
	nativeAttempt(stages, "unchanged", "trusted-repeat-add-installer.json")
	for _, step := range []struct {
		label string
		args  []string
	}{{"trusted-repeat-add", []string{"add", f.base.Package, "--no-save"}}, {"trusted-unchanged-update", []string{"update", "native-proof"}}} {
		result := codexProjectAssertionEnvelope(f.run(t, step.label, step.args...))
		codexNativeProjectPrepared(t, result)
		commandsRequireMutated(t, step.label, result, false)
		outcome, err := commandsTargetOutcome(result)
		if err != nil || outcome["no_change"] != true {
			t.Fatalf("unchanged project result: %v %v", outcome, err)
		}
		requireStable()
		codexProjectMCPSession(t, f, f.base.Project, step.label, &binding, data, true)
		requireStable()
	}
	stages["unchanged"] = nativeStage{Status: "passed", Reason: "repeat add/update preserved exact Prepared state/config and direct MCP tool/data behavior"}
	codexProjectMCPSession(t, f, f.otherProject, "control-after-replay", nil, "", false)
	nativeAttempt(stages, "remove", "trusted-remove-installer.json")
	f.run(t, "trusted-remove", "remove", "native-proof")
	codexProjectMCPSession(t, f, f.base.Project, "removed-A", nil, "", false)
	codexProjectMCPSession(t, f, f.otherProject, "removed-B", nil, "", false)
	stages["remove"] = nativeStage{Status: "passed", Reason: "fresh trusted A/B sessions have no fixture servers or new MCP process events after owned removal"}
	marker, err := os.ReadFile(filepath.Join(data, "native-marker.txt"))
	if err != nil || string(marker) != "persist-"+f.base.Nonce {
		t.Fatal("removal changed retained tool data")
	}
	stages["retained_data"] = nativeStage{Status: "passed", Reason: "bounded tool marker remains at exact recorded project data locator"}
	f.requireForeign(t)
	stages["scope_preservation"] = nativeStage{Status: "passed", Reason: "original user trust/provider config, B preferences and A foreign bytes remained exact"}
}

func codexProjectMCPBinding(t *testing.T, f *codexProjectNativeFixture) (domain.ClientBinding, string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(f.base.Project, ".uap", "state-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(body, &state); err != nil || len(state.Installations) != 1 {
		t.Fatalf("exact project installation: %v", err)
	}
	installation := state.Installations[0]
	if installation.DeclaredName != "native-proof" || len(installation.Clients) != 1 {
		t.Fatal("ambiguous project package/binding")
	}
	for _, binding := range installation.Clients {
		data := installation.DataReceipts[binding.DataReceiptID]
		if binding.ClientID != "codex" || binding.Scope != "project" || binding.NativeProfileRoot != f.base.Project || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || binding.PhysicalArtifact == "" || data.DataReceiptID != binding.DataReceiptID || data.Scope != "project" || data.PhysicalBackend != binding.PhysicalArtifact || data.State != domain.DataReceiptOwned || data.OwnershipDigest == "" {
			t.Fatal("project binding/data authority differs")
		}
		nativeRequireContained(t, filepath.Join(f.base.Project, ".uap", "managed"), binding.TargetLocator)
		nativeRequireContained(t, filepath.Join(f.base.Project, ".uap", "plugin-data"), data.Locator)
		owned := map[string]bool{}
		for _, object := range binding.NativeObjects {
			if object.Kind == "codex_project_mcp_server" {
				if object.Path != filepath.Join(f.base.Project, ".codex", "config.toml") || object.ManagedDigest == "" || owned[object.LogicalName] {
					t.Fatal("MCP ownership differs")
				}
				owned[object.LogicalName] = true
			}
		}
		if len(owned) != 2 || !owned["default"] || !owned["explicit"] {
			t.Fatal("exact project MCP receipt names missing")
		}
		return binding, data.Locator
	}
	t.Fatal("project binding absent")
	return domain.ClientBinding{}, ""
}

func codexProjectMCPSession(t *testing.T, f *codexProjectNativeFixture, project, label string, binding *domain.ClientBinding, data string, present bool) {
	t.Helper()
	before, err := os.ReadFile(f.base.Events)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	selected := *f.base
	selected.Project = project
	r := startNativeRPC(t, &selected, f.client, label)
	defer r.close()
	r.must(t, "initialize", map[string]any{"clientInfo": map[string]string{"name": "uap_project_mcp_fixture", "version": "1.0.0"}, "capabilities": map[string]any{"experimentalApi": true}})
	if err := json.NewEncoder(r.in).Encode(map[string]any{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	var thread struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(r.must(t, "thread/start", map[string]any{"cwd": project, "ephemeral": true, "approvalPolicy": "never", "sandbox": "read-only", "model": "fixture-model", "modelProvider": "fixture"}), &thread); err != nil || thread.Thread.ID == "" {
		t.Fatal("native project thread missing")
	}
	var inventory struct {
		Data []struct {
			Name, PluginID, RuntimeStatus string
			Tools                         map[string]json.RawMessage
		} `json:"data"`
		NextCursor *string `json:"nextCursor"`
	}
	if err := json.Unmarshal(r.must(t, "mcpServerStatus/list", map[string]any{"threadId": thread.Thread.ID, "limit": 100}), &inventory); err != nil || inventory.NextCursor != nil {
		t.Fatal("incomplete native MCP inventory")
	}
	if present {
		seen := map[string]bool{}
		for _, server := range inventory.Data {
			if binding == nil || server.PluginID != "" || (server.Name != "default" && server.Name != "explicit") || server.RuntimeStatus != "connected" || seen[server.Name] || len(server.Tools) != 1 || server.Tools["inspect_runtime"] == nil {
				t.Fatalf("unexpected project MCP server: %+v", server)
			}
			seen[server.Name] = true
			operation := "read"
			if label == "trusted-add" {
				operation = "write"
			}
			facts := nativeDecodeFacts(t, r.must(t, "mcpServer/tool/call", map[string]any{"threadId": thread.Thread.ID, "server": server.Name, "tool": "inspect_runtime", "arguments": map[string]string{"nonce": f.base.Nonce, "operation": operation, "marker": "persist-" + f.base.Nonce}}))
			nativeAssertEvent(t, f.base, facts)
			cwd := binding.TargetLocator
			if server.Name == "explicit" {
				cwd = filepath.Join(cwd, "skills")
			}
			if facts.Nonce != f.base.Nonce || facts.Revision != "A" || facts.Root != binding.TargetLocator || facts.Data != data || facts.CWD != cwd || facts.Marker != "persist-"+f.base.Nonce || facts.Literal != "literal ${UNKNOWN} $HOME" || facts.Once != facts.Root+"/${UNKNOWN}" || len(facts.Argv) != 3 || facts.Argv[0] != "argument with spaces" || facts.Argv[1] != facts.Root+"/argument" || facts.Argv[2] != "${UNKNOWN}" {
				t.Fatalf("native project runtime facts differ: %+v", facts)
			}
		}
		if len(seen) != 2 {
			t.Fatal("native project MCP servers missing")
		}
	} else if len(inventory.Data) != 0 {
		t.Fatalf("project MCP leaked into absent/control session: %+v", inventory.Data)
	}
	r.close()
	if !present {
		after, err := os.ReadFile(f.base.Events)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("absent/control session started fixture MCP processes")
		}
	}
	if len(f.base.ProviderRequests) != 0 {
		t.Fatal("direct MCP test unexpectedly requested model inference")
	}
	f.requireForeign(t)
}
