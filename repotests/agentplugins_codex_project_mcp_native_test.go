package pluginkitairepo_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"

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
	for _, name := range []string{"install", "native_project_mcp", "native_env_forwarding", "unchanged", "remove", "scope_preservation", "retained_data"} {
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
	// Codex-native name forwarding is not a portable Agent Plugins field.
	// Use the real trusted Codex client and its already qualified direct MCP tool
	// route. Parent receives a synthetic canary; the child reports only its hash.
	// The negative run proves that inherited parent environment alone is not
	// sufficient to qualify env_vars as the mechanism.
	nativeAttempt(stages, "native_env_forwarding", "trusted-canary-positive-rpc.jsonl")
	originalConfig, err := os.ReadFile(projectConfig)
	if err != nil {
		t.Fatal(err)
	}
	secret := "fixture-" + f.base.Nonce
	digest := sha256.Sum256([]byte(secret))
	f.base.ExtraEnv = []string{"UAP_TEST_SECRET_CANARY=" + secret}
	codexProjectMCPSession(t, f, f.base.Project, "trusted-canary-negative", &binding, data, true,
		map[string]string{"default": "", "explicit": ""})
	var nativeConfig map[string]any
	if err := toml.Unmarshal(originalConfig, &nativeConfig); err != nil {
		t.Fatal(err)
	}
	servers, ok := nativeConfig["mcp_servers"].(map[string]any)
	if !ok {
		t.Fatal("Codex project MCP collection unavailable for native-only spike")
	}
	selected, ok := servers["default"].(map[string]any)
	if !ok {
		t.Fatal("selected Codex stdio MCP server unavailable for native-only spike")
	}
	selected["env_vars"] = []string{"UAP_TEST_SECRET_CANARY"}
	forwardedConfig, err := toml.Marshal(nativeConfig)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectConfig, forwardedConfig, 0600); err != nil {
		t.Fatal(err)
	}
	codexProjectMCPSession(t, f, f.base.Project, "trusted-canary-positive", &binding, data, true,
		map[string]string{"default": hex.EncodeToString(digest[:]), "explicit": ""})
	if err := os.WriteFile(projectConfig, originalConfig, 0600); err != nil {
		t.Fatal(err)
	}
	codexProjectMCPSession(t, f, f.base.Project, "trusted-canary-restored", &binding, data, true,
		map[string]string{"default": "", "explicit": ""})
	f.base.ExtraEnv = nil
	assertNativeCanaryAbsentFromObservedFiles(t, f.base.Root, secret)
	requireStable()
	stages["native_env_forwarding"] = nativeStage{Status: "passed", Reason: "pinned real Codex forwarded only explicitly named synthetic env to one MCP child; absence without env_vars and after restoration confirmed; no canary value stored"}

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

func codexProjectMCPSession(t *testing.T, f *codexProjectNativeFixture, project, label string, binding *domain.ClientBinding, data string, present bool, wantCanary ...map[string]string) {
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
	// Native thread startup is asynchronous. Keep all inventory reads within the
	// original request budget, itself bounded by this session's existing deadline.
	ctx, cancel := context.WithTimeout(r.ctx, 35*time.Second)
	defer cancel()
	inventory, err := codexProjectMCPReady(ctx, func(ctx context.Context) (json.RawMessage, error) {
		return r.requestContext(ctx, "mcpServerStatus/list", map[string]any{"threadId": thread.Thread.ID, "limit": 100})
	}, present)
	if err != nil {
		t.Fatal(err)
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
			if len(wantCanary) > 0 {
				expected, selected := wantCanary[0][server.Name]
				if !selected || facts.CanaryDigest != expected {
					t.Fatalf("native MCP forwarding proof failed for server %q (present=%t)", server.Name, facts.CanaryDigest != "")
				}
			}

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

// The only claim is absence from a bounded inventory of this disposable
// fixture. A failed/partial inspection is inconclusive, never a secrecy PASS.
func assertNativeCanaryAbsentFromObservedFiles(t *testing.T, root, canary string) {
	t.Helper()
	needles := [][]byte{[]byte(canary), []byte(base64.StdEncoding.EncodeToString([]byte(canary))), []byte(hex.EncodeToString([]byte(canary)))}
	const maxEntries, maxFileBytes, maxTotalBytes = 8192, 128 << 20, 512 << 20
	entries, total := 0, int64(0)
	err := filepath.WalkDir(root, func(path string, item os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		entries++
		if entries > maxEntries {
			return fmt.Errorf("canary scan exceeded entry limit")
		}
		for _, needle := range needles {
			if bytes.Contains([]byte(path), needle) {
				return fmt.Errorf("canary persisted in a fixture path")
			}
		}
		if item.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			for _, needle := range needles {
				if bytes.Contains([]byte(target), needle) {
					return fmt.Errorf("canary persisted in a fixture symlink")
				}
			}
			return nil // the claim excludes files referenced through symlinks
		}
		if item.IsDir() {
			return nil
		}
		info, err := item.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > maxFileBytes || total+info.Size() > maxTotalBytes {
			return fmt.Errorf("canary scan cannot admit file type or size")
		}
		total += info.Size()
		body, err := os.ReadFile(path)
		if err != nil || int64(len(body)) != info.Size() {
			return fmt.Errorf("canary scan could not fully read observed file")
		}
		for _, needle := range needles {
			if bytes.Contains(body, needle) {
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return fmt.Errorf("synthetic canary detected but fixture path attribution failed")
				}
				return fmt.Errorf("synthetic canary persisted in observed fixture file %q", filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Codex native canary persistence qualification incomplete or unsafe: %v", err)
	}
}
