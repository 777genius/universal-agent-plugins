package pluginkitairepo_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// This suite is intentionally opt-in and uses only a freshly provisioned binary.
// macOS release binaries read managed CFPreferences independently of HOME; this
// runner requires a clean Linux container/VM until that boundary is isolated.
func nativeBinary(t *testing.T, key string) string {
	t.Helper()
	p := os.Getenv(key)
	if !filepath.IsAbs(p) {
		t.Fatalf("%s must name an absolute scratch binary", key)
	}
	st, err := os.Stat(p)
	if err != nil || !st.Mode().IsRegular() || st.Mode()&0111 == 0 {
		t.Fatalf("invalid %s binary", key)
	}
	return p
}

type nativeRPC struct {
	cmd      *exec.Cmd
	in       io.WriteCloser
	messages chan []byte
	cancel   context.CancelFunc
	log      *os.File
	next     int
}

func startNativeRPC(t *testing.T, f *nativeFixture, client, label string) *nativeRPC {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	cmd := exec.CommandContext(ctx, client, "app-server")
	cmd.Dir = f.Project
	cmd.Env = f.env(filepath.Dir(client))
	in, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	stderr, err := os.Create(filepath.Join(f.Root, label+"-stderr.log"))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	cmd.Stderr = stderr
	log, err := os.Create(filepath.Join(f.Root, label+"-rpc.jsonl"))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		stderr.Close()
		log.Close()
		t.Fatal(err)
	}
	r := &nativeRPC{cmd: cmd, in: in, cancel: cancel, log: log, messages: make(chan []byte, 128)}
	go func() {
		defer close(r.messages)
		scan := bufio.NewScanner(io.LimitReader(out, 16<<20))
		scan.Buffer(make([]byte, 4096), 2<<20)
		for scan.Scan() {
			b := bytes.Clone(scan.Bytes())
			select {
			case r.messages <- b:
			case <-ctx.Done():
				return
			}
		}
	}()
	t.Cleanup(func() { r.close(); stderr.Close() })
	return r
}
func (r *nativeRPC) close() {
	if r.cancel != nil {
		r.in.Close()
		r.cancel()
		_ = r.cmd.Wait()
		r.log.Close()
		r.cancel = nil
	}
}
func (r *nativeRPC) request(method string, params any) (json.RawMessage, error) {
	r.next++
	id := r.next
	request := map[string]any{"id": id, "method": method, "params": params}
	if err := json.NewEncoder(r.log).Encode(map[string]any{"direction": "send", "message": request}); err != nil {
		return nil, err
	}
	if err := json.NewEncoder(r.in).Encode(request); err != nil {
		return nil, err
	}
	timer := time.NewTimer(35 * time.Second)
	defer timer.Stop()
	for {
		select {
		case b, ok := <-r.messages:
			if !ok {
				return nil, fmt.Errorf("app-server closed during %s", method)
			}
			var row struct {
				ID     int             `json:"id"`
				Result json.RawMessage `json:"result"`
				Error  json.RawMessage `json:"error"`
			}
			if err := json.Unmarshal(b, &row); err != nil {
				return nil, err
			}
			if err := json.NewEncoder(r.log).Encode(map[string]any{"direction": "receive", "message": json.RawMessage(b)}); err != nil {
				return nil, err
			}
			if row.ID == id {
				if len(row.Error) > 0 {
					return nil, fmt.Errorf("%s: %s", method, row.Error)
				}
				return row.Result, nil
			}
		case <-timer.C:
			return nil, fmt.Errorf("timeout: %s", method)
		}
	}
}
func (r *nativeRPC) must(t *testing.T, method string, params any) json.RawMessage {
	t.Helper()
	b, err := r.request(method, params)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func nativeCommand(t *testing.T, f *nativeFixture, binary, label string, args ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = f.env(filepath.Dir(binary))
	cmd.Dir = f.Project
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	nativeWrite(t, filepath.Join(f.Root, label+"-stderr.log"), stderr.Bytes(), 0600)
	nativeWrite(t, filepath.Join(f.Root, label+".log"), b, 0600)
	return b, err
}
func nativeInstaller(t *testing.T, f *nativeFixture, installer, client, label string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, installer, append(args, "--target", "codex", "--format", "json")...)
	cmd.Env = f.env(filepath.Dir(client))
	cmd.Dir = f.Project
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	nativeWrite(t, filepath.Join(f.Root, label+"-installer-stderr.log"), stderr.Bytes(), 0600)
	nativeWrite(t, filepath.Join(f.Root, label+"-installer.json"), b, 0600)
	if err != nil {
		t.Fatalf("%s: %v\nstdout: %s\nstderr: %s", label, err, b, stderr.Bytes())
	}
	if err := nativeValidateInstallerJSON(b); err != nil {
		t.Fatalf("%s JSON contract: %v; stderr: %s", label, err, stderr.Bytes())
	}
	return b
}
func nativeSession(t *testing.T, f *nativeFixture, client, label, revision, operation string, stages map[string]nativeStage) nativeFacts {
	t.Helper()
	nativeAttempt(stages, label+"_native_tool", label+"-rpc.jsonl")
	r := startNativeRPC(t, f, client, label)
	defer r.close()
	r.must(t, "initialize", map[string]any{"clientInfo": map[string]string{"name": "uap_native_fixture", "version": "1.0.0"}, "capabilities": map[string]any{"experimentalApi": true}})
	if err := json.NewEncoder(r.in).Encode(map[string]any{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	var thread struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(r.must(t, "thread/start", map[string]any{"cwd": f.Project, "ephemeral": true, "approvalPolicy": "never", "sandbox": "read-only", "model": "fixture-model", "modelProvider": "fixture"}), &thread); err != nil || thread.Thread.ID == "" {
		t.Fatalf("thread start: %v", err)
	}
	var inventory struct {
		Data []struct {
			Name          string                     `json:"name"`
			PluginID      string                     `json:"pluginId"`
			RuntimeStatus string                     `json:"runtimeStatus"`
			Tools         map[string]json.RawMessage `json:"tools"`
		} `json:"data"`
		NextCursor *string `json:"nextCursor"`
	}
	b := r.must(t, "mcpServerStatus/list", map[string]any{"threadId": thread.Thread.ID, "limit": 100})
	if err := json.Unmarshal(b, &inventory); err != nil {
		t.Fatal(err)
	}
	if inventory.NextCursor != nil {
		t.Fatal("unexpected inventory pagination; refusing incomplete evidence")
	}
	var observed nativeFacts
	count := 0
	serverNames := map[string]bool{}
	for _, server := range inventory.Data {
		if server.PluginID != f.PluginID {
			continue
		}
		if serverNames[server.Name] {
			t.Fatalf("duplicate native plugin server: %s", server.Name)
		}
		serverNames[server.Name] = true
		if label == "partial_failure" && server.Name == "startup-failure" {
			if server.RuntimeStatus != "failed" || len(server.Tools) != 0 {
				t.Fatalf("startup-failure server was not independently failed: %+v", server)
			}
			continue
		}
		if server.Name == "http" {
			result, err := r.request("mcpServer/tool/call", map[string]any{"threadId": thread.Thread.ID, "server": server.Name, "tool": "inspect_runtime", "arguments": map[string]string{"nonce": f.Nonce}})
			if err != nil {
				stages[label+"_http"] = nativeStage{Status: "failed", Reason: err.Error()}
			} else {
				facts := nativeDecodeFacts(t, result)
				nativeAssertEvent(t, f, facts)
				stages[label+"_http"] = nativeStage{Status: "passed", Reason: "Native HTTP call correlated; HTTP process roots are harness behavior", Artifacts: []string{"http-events.jsonl", label + "-rpc.jsonl"}}
			}
			continue
		}
		if server.Name != "default" && server.Name != "explicit" {
			continue
		}
		count++
		result := r.must(t, "mcpServer/tool/call", map[string]any{"threadId": thread.Thread.ID, "server": server.Name, "tool": "inspect_runtime", "arguments": map[string]string{"nonce": f.Nonce, "operation": operation, "marker": "persist-" + f.Nonce}})
		facts := nativeDecodeFacts(t, result)
		nativeAssertEvent(t, f, facts)
		if facts.Revision != revision || facts.Nonce != f.Nonce || facts.Marker != "persist-"+f.Nonce {
			t.Fatalf("runtime mismatch: %+v", facts)
		}
		for _, p := range []string{facts.Root, facts.Data, facts.CWD} {
			rel, err := filepath.Rel(f.Root, p)
			if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
				t.Fatalf("runtime path escapes fixture: %s", p)
			}
		}
		if !strings.Contains(facts.Root, string(filepath.Separator)+"plugins/cache/") {
			t.Fatalf("not native installed cache: %s", facts.Root)
		}
		for _, path := range []string{"plugin.json", ".codex-plugin/plugin.json"} {
			if _, err := os.Stat(filepath.Join(facts.Root, path)); err != nil {
				t.Fatalf("missing root/sidecar: %v", err)
			}
		}
		wantCWD := facts.Root
		if server.Name == "explicit" {
			wantCWD = filepath.Join(facts.Root, "skills")
		}
		if facts.CWD != wantCWD {
			stages[label+"_cwd_"+server.Name] = nativeStage{Status: "failed", Reason: fmt.Sprintf("observed %s, wanted %s", facts.CWD, wantCWD)}
		} else {
			stages[label+"_cwd_"+server.Name] = nativeStage{Status: "passed"}
		}
		if facts.Literal != "literal ${UNKNOWN} $HOME" || facts.Once != facts.Root+"/${UNKNOWN}" || len(facts.Argv) != 3 || facts.Argv[0] != "argument with spaces" || facts.Argv[1] != facts.Root+"/argument" || facts.Argv[2] != "${UNKNOWN}" {
			t.Fatalf("literal/substitution mismatch: %+v", facts)
		}
		observed = facts
	}
	wantServers := 4
	if label == "partial_failure" {
		wantServers = 5
		if !serverNames["startup-failure"] || serverNames["unavailable"] || serverNames["unsupported-sse"] {
			t.Fatalf("failure isolation inventory: %v", serverNames)
		}
	}
	if len(serverNames) != wantServers || !serverNames["default"] || !serverNames["explicit"] || !serverNames["http"] || !serverNames["redirect"] {
		t.Fatalf("unexpected exact native server inventory: %v", serverNames)
	}
	if count != 2 {
		t.Fatalf("expected 2 plugin servers, got %d: %s", count, b)
	}
	stages[label+"_native_tool"] = nativeStage{Status: "passed", Artifacts: []string{label + "-rpc.jsonl", "events.jsonl"}}
	nativeAttempt(stages, label+"_skill_discovery", label+"-rpc.jsonl")
	var skills struct {
		Data []struct {
			Skills []struct {
				Name     string `json:"name"`
				Path     string `json:"path"`
				PluginID string `json:"pluginId"`
				Enabled  bool   `json:"enabled"`
			} `json:"skills"`
		} `json:"data"`
	}
	b = r.must(t, "skills/list", map[string]any{"cwds": []string{f.Project}, "forceReload": true})
	if err := json.Unmarshal(b, &skills); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, group := range skills.Data {
		for _, skill := range group.Skills {
			if label == "partial_failure" && skill.PluginID == f.PluginID && strings.Contains(skill.Path, "invalid-proof") {
				t.Fatal("invalid skill reached native discovery")
			}
			if nativeSkillIdentity(skill.Name, skill.PluginID, skill.Path, skill.Enabled, f.PluginID, observed.Root) {
				body, err := os.ReadFile(skill.Path)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Contains(body, []byte("UAP_SKILL_BODY_"+revision+"_"+f.Nonce)) {
					t.Fatal("stale skill cache")
				}
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("plugin skill absent: %s", b)
	}
	stages[label+"_native_tool"] = nativeStage{Status: "passed", Artifacts: []string{label + "-rpc.jsonl", "events.jsonl"}}
	stages[label+"_skill_discovery"] = nativeStage{Status: "passed", Reason: "Exact plugin-attributed skill path and body bytes; no model use claimed"}
	if label == "A" && f.ProviderRequests != nil {
		nativeAttempt(stages, "skill_context", "skill-context-request.json")
		r.must(t, "turn/start", map[string]any{"threadId": thread.Thread.ID, "input": []any{
			map[string]any{"type": "text", "text": "Use the explicitly selected skill and reply briefly.", "text_elements": []any{}},
			map[string]string{"type": "skill", "name": "native-proof:native-proof", "path": filepath.Join(observed.Root, "skills", "native-proof", "SKILL.md")},
		}})
		select {
		case body := <-f.ProviderRequests:
			nativeWrite(t, filepath.Join(f.Root, "skill-context-request.json"), body, 0600)
			var request map[string]json.RawMessage
			if err := json.Unmarshal(body, &request); err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(request["input"], []byte("UAP_SKILL_BODY_A_"+f.Nonce)) {
				t.Fatal("native turn did not inject selected installed skill body into outgoing input")
			}
			stages["skill_context"] = nativeStage{Status: "passed", Reason: "model_provider=scripted_loopback; real native explicit-skill turn injected BODY into outgoing Responses input; no real model skill use claimed", Artifacts: []string{"skill-context-request.json", label + "-rpc.jsonl"}}
		case <-time.After(30 * time.Second):
			t.Fatal("native explicit-skill turn did not reach scripted provider")
		}
	}
	return observed
}

func TestAgentpluginsCodexNativeLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CODEX_NATIVE_E2E") != "1" {
		t.Skip("opt-in native client execution")
	}
	if runtime.GOOS != "linux" || os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_LINUX") != "1" {
		t.Fatal("requires a newly provisioned disposable Linux container/VM; macOS managed preferences are not isolated by HOME")
	}
	if _, err := os.Lstat("/etc/codex"); !os.IsNotExist(err) {
		t.Fatal("system Codex config must be absent in disposable image")
	}
	client := nativeBinary(t, "AGENTPLUGINS_CODEX_BIN")
	installer := nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	f := newNativeFixture(t, true)
	f.ClientBinDir = filepath.Dir(client)
	stages := map[string]nativeStage{}
	for _, name := range []string{"install", "A_native_tool", "B_native_tool", "same_version_native_tool", "unchanged", "owned_repair", "native_cache_repair", "partial_failure", "all_unsupported", "remove", "repeat_remove", "foreign_preservation", "collision_drift", "immutable_git", "immutable_git_native", "http", "redirect_headers", "skill_context", "real_model_skill_use", "oauth"} {
		stages[name] = nativeStage{Status: "not_evaluated", Reason: "not reached"}
	}
	evidence := map[string]any{"started_utc": time.Now().UTC().Format(time.RFC3339Nano), "os": runtime.GOOS, "arch": runtime.GOARCH, "profile": f.Root, "client_version": "0.153.4", "client_source": "3d2ee51ca2d5db578f328aa75e20aa22c0197c9a", "client_sha256": nativeSHA(t, client), "installer_sha256": nativeSHA(t, installer), "acquisition": "local_directory", "native_surface": "codex app-server direct thread MCP RPC", "stages": stages}
	defer func() {
		evidence["finished_utc"] = time.Now().UTC().Format(time.RFC3339Nano)
		hashes := map[string]string{}
		entries, _ := os.ReadDir(f.Root)
		for _, e := range entries {
			if !e.IsDir() && (strings.HasSuffix(e.Name(), ".jsonl") || strings.HasSuffix(e.Name(), ".log") || strings.HasSuffix(e.Name(), "-installer.json") || e.Name() == "skill-context-request.json" || e.Name() == "immutable-git.json") {
				hashes[e.Name()] = nativeSHA(t, filepath.Join(f.Root, e.Name()))
			}
		}
		evidence["artifact_sha256"] = hashes
		nativeJSON(t, filepath.Join(f.Root, "evidence.json"), evidence)
	}()
	identity, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range identity {
		evidence[key] = value
	}
	providerURL := nativeScriptedProvider(t, f)
	nativeWrite(t, filepath.Join(f.CodexHome, "config.toml"), []byte("cli_auth_credentials_store = \"file\"\nmcp_oauth_credentials_store = \"file\"\nmodel_provider = \"fixture\"\nmodel = \"fixture-model\"\n[model_providers.fixture]\nname = \"Fixture\"\nbase_url = \""+providerURL+"\"\nwire_api = \"responses\"\nrequires_openai_auth = false\n[analytics]\nenabled = false\n[feedback]\nenabled = false\n"), 0600)
	b, err := nativeCommand(t, f, client, "client-version", "--version")
	if err != nil || strings.TrimSpace(string(b)) != "codex-cli 0.153.4" {
		t.Fatalf("wrong pinned client: %s %v", b, err)
	}
	nativeAttempt(stages, "scanner_prerequisite", "evidence.json")
	evidence["security_scanner"] = nativeProvisionScanner(t, f)
	stages["scanner_prerequisite"] = nativeStage{Status: "passed", Reason: "Verified pinned official scanner prepopulated supported fresh cache"}
	nativeHTTP(t, f)
	f.writePackage(t, "A", "1.0.0")
	evidence["manifest_sha256"] = nativeSHA(t, filepath.Join(f.Package, "plugin.json"))
	nativeAttempt(stages, "install", "add-installer.json")
	nativeAttempt(stages, "local_security_scan", "add-installer.json")
	addOutput := nativeInstaller(t, f, installer, client, "add", "add", f.Package)
	var securityOutput any
	if err := json.Unmarshal(addOutput, &securityOutput); err != nil {
		t.Fatal(err)
	}
	if err := nativeValidateMCPSelections(securityOutput); err != nil {
		t.Fatal(err)
	}
	if nativeFindString(securityOutput, "evidence_source") != "local_scan" {
		t.Fatal("first add did not report ordinary local_scan security assessment")
	}
	stages["local_security_scan"] = nativeStage{Status: "passed", Artifacts: []string{"add-installer.json"}}
	stages["install"] = nativeStage{Status: "passed", Artifacts: []string{"add-installer.json"}}
	var output any
	if err := json.Unmarshal(addOutput, &output); err != nil {
		t.Fatal(err)
	}
	active := nativeFindString(output, "active_path")
	if active == "" {
		state, err := os.ReadFile(filepath.Join(f.Root, "installer-state", "state-v2.json"))
		if err != nil {
			t.Fatal(err)
		}
		var doc any
		if err := json.Unmarshal(state, &doc); err != nil {
			t.Fatal(err)
		}
		active = nativeFindString(doc, "target_locator")
	}
	if active == "" {
		t.Fatal("installer state omitted managed target_locator")
	}
	nativeRequireContained(t, f.Root, active)
	physical := nativeFindString(securityOutput, "physical_artifact_id")
	if physical == "" {
		t.Fatal("installer plan omitted physical artifact identity")
	}
	f.PluginID = "native-proof@" + providers.ManagedMarketplaceName(physical)
	nativeAttempt(stages, "native_inventory", "installed-inventory.log")
	b, err = nativeCommand(t, f, client, "installed-inventory", "plugin", "list", "--json")
	if err != nil {
		t.Fatalf("native inventory: %v %s", err, b)
	}
	var inventory struct {
		Installed []struct {
			ID          string `json:"pluginId"`
			Name        string `json:"name"`
			Marketplace string `json:"marketplaceName"`
			Source      struct {
				Path string `json:"path"`
			} `json:"source"`
			Installed bool `json:"installed"`
			Enabled   bool `json:"enabled"`
		} `json:"installed"`
	}
	if err := json.Unmarshal(b, &inventory); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range inventory.Installed {
		if p.Name == "native-proof" && p.ID == f.PluginID && p.ID == p.Name+"@"+p.Marketplace && p.Source.Path == active && p.Installed && p.Enabled {
			found = true
			evidence["native_plugin_id"] = p.ID
		}
	}
	if !found {
		t.Fatalf("exact native installation missing: %s", b)
	}
	stages["native_inventory"] = nativeStage{Status: "passed", Artifacts: []string{"installed-inventory.log"}}
	foreign := map[string][]byte{
		filepath.Join(f.CodexHome, "skills", "foreign-proof", "SKILL.md"):                                          []byte("---\nname: foreign-proof\ndescription: Foreign sandbox sentinel\n---\nForeign fixture body\n"),
		filepath.Join(f.CodexHome, "plugins", "cache", "foreign-market", "foreign-plugin", "1.0.0", "plugin.json"): []byte(`{"name":"foreign-plugin","version":"1.0.0"}`),
		filepath.Join(f.CodexHome, "plugins", "marketplaces", "foreign-market", "sentinel.txt"):                    []byte("foreign marketplace fixture"),
	}
	for path, body := range foreign {
		nativeWrite(t, path, body, 0600)
	}
	a := nativeSession(t, f, client, "A", "A", "write", stages)
	nativeAttempt(stages, "B_native_tool", "update-B-installer.json")
	f.writePackage(t, "B", "1.1.0")
	nativeInstaller(t, f, installer, client, "update-B", "update", "native-proof")
	bFacts := nativeSession(t, f, client, "B", "B", "read", stages)
	nativeAttempt(stages, "B_data_identity", "B-rpc.jsonl")
	if a.Data != bFacts.Data {
		t.Fatal("native data identity changed")
	}
	nativeAttempt(stages, "same_version_native_tool", "update-same-version-installer.json")
	stages["B_data_identity"] = nativeStage{Status: "passed"}
	f.writePackage(t, "C", "1.1.0")
	nativeInstaller(t, f, installer, client, "update-same-version", "update", "native-proof")
	c := nativeSession(t, f, client, "same_version", "C", "read", stages)
	nativeAttempt(stages, "same_version_data_identity", "same_version-rpc.jsonl")
	if c.Data != a.Data {
		t.Fatal("same-version data identity changed")
	}
	stages["same_version_data_identity"] = nativeStage{Status: "passed"}
	nativeAttempt(stages, "unchanged", "unchanged-installer.json")
	statePath := filepath.Join(f.Root, "installer-state", "state-v2.json")
	unchangedState, unchangedManaged, unchangedCache := nativeSHA(t, statePath), nativeTreeDigest(t, active), nativeTreeDigest(t, c.Root)
	b = nativeInstaller(t, f, installer, client, "unchanged", "add", f.Package)
	if err := nativeUnchangedResult(b); err != nil {
		t.Fatal(err)
	}
	if nativeSHA(t, statePath) != unchangedState || nativeTreeDigest(t, active) != unchangedManaged || nativeTreeDigest(t, c.Root) != unchangedCache {
		t.Fatal("non-mutating same-source add changed state or managed/cache content")
	}
	stages["unchanged"] = nativeStage{Status: "passed", Reason: "Unchanged package: explicit mutated=false and identical state/managed/cache bytes; authentication convergence not claimed"}
	// A changed existing directory deliberately loses provable content ownership.
	// Verify fail-closed protection, then restore only bytes this fixture saved.
	nativeAttempt(stages, "owned_artifact_guard", "owned-guard.log")
	skillPath := filepath.Join(active, "skills", "native-proof", "SKILL.md")
	originalSkill, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	originalInfo, err := os.Stat(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	originalDigest := nativeTreeDigest(t, active)
	nativeWrite(t, skillPath, []byte("owned damage"), originalInfo.Mode().Perm())
	damagedDigest, cacheDigest := nativeTreeDigest(t, active), nativeTreeDigest(t, c.Root)
	guardState, guardMarker := nativeSHA(t, statePath), nativeSHA(t, filepath.Join(a.Data, "native-marker.txt"))
	guard, guardErr := nativeCommand(t, f, installer, "owned-guard", "repair", "native-proof", "--target", "codex", "--format", "json")
	guardStderr, err := os.ReadFile(filepath.Join(f.Root, "owned-guard-stderr.log"))
	if err != nil {
		t.Fatal(err)
	}
	if err := nativeRepairGuardResult(guard, guardStderr, guardErr); err != nil {
		t.Fatal(err)
	}
	if nativeTreeDigest(t, active) != damagedDigest || nativeTreeDigest(t, c.Root) != cacheDigest || nativeSHA(t, statePath) != guardState || nativeSHA(t, filepath.Join(a.Data, "native-marker.txt")) != guardMarker {
		t.Fatal("refused repair changed state, managed/cache bytes or native data")
	}
	stages["owned_artifact_guard"] = nativeStage{Status: "passed", Reason: "Existing modified artifact refused as indeterminate ownership; managed/cache bytes unchanged", Artifacts: []string{"owned-guard.log", "owned-guard-stderr.log"}}
	nativeWrite(t, skillPath, originalSkill, originalInfo.Mode().Perm())
	if nativeTreeDigest(t, active) != originalDigest {
		t.Fatal("fixture original bytes were not restored exactly")
	}
	// A foreign directory at the formerly owned path must not be adopted.
	nativeAttempt(stages, "collision_drift", "foreign-collision.log")
	collisionBackup := filepath.Join(f.Root, "collision-original-backup")
	if err := os.Rename(active, collisionBackup); err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, filepath.Join(active, "foreign-sentinel"), []byte("foreign-"+f.Nonce), 0600)
	collisionDigest := nativeTreeDigest(t, active)
	collision, collisionErr := nativeCommand(t, f, installer, "foreign-collision", "repair", "native-proof", "--target", "codex", "--format", "json")
	collisionStderr, err := os.ReadFile(filepath.Join(f.Root, "foreign-collision-stderr.log"))
	if err != nil {
		t.Fatal(err)
	}
	if err := nativeCollisionGuardResult(collision, collisionStderr, collisionErr); err != nil {
		t.Fatal(err)
	}
	if nativeTreeDigest(t, active) != collisionDigest || nativeSHA(t, statePath) != guardState || nativeTreeDigest(t, c.Root) != cacheDigest {
		t.Fatal("foreign collision refusal mutated protected bytes")
	}
	stages["collision_drift"] = nativeStage{Status: "passed", Reason: "Modified owned artifact and foreign replacement directory independently refused; state/cache/foreign bytes preserved", Artifacts: []string{"owned-guard.log", "foreign-collision.log", "foreign-collision-stderr.log"}}
	// Explicit fixture reset of only the test-authored foreign directory.
	if err := os.Rename(active, filepath.Join(f.Root, "foreign-collision-retained")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(collisionBackup, active); err != nil {
		t.Fatal(err)
	}
	// Actual reconstruction tests an absent recorded artifact separately.
	nativeAttempt(stages, "owned_repair", "owned-repair-installer.json")
	backup := filepath.Join(f.Root, "owned-artifact-backup")
	if err := os.Rename(active, backup); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatal("recorded artifact is not absent after fixture backup")
	}
	nativeInstaller(t, f, installer, client, "owned-repair", "repair", "native-proof")
	nativeSession(t, f, client, "owned_repair", "C", "read", stages)
	stages["owned_repair"] = nativeStage{Status: "passed", Reason: "Absent recorded artifact reconstructed by supported repair; fresh tool/skill verified"}
	// Native cache is a separate ownership surface; refusal is recorded independently.
	nativeAttempt(stages, "native_cache_repair", "native-cache-repair.log")
	nativeWrite(t, filepath.Join(c.Root, "skills", "native-proof", "SKILL.md"), []byte("native cache damage"), 0600)
	repair, repairErr := nativeCommand(t, f, installer, "native-cache-repair", "repair", "native-proof", "--target", "codex", "--format", "json")
	if repairErr != nil {
		stages["native_cache_repair"] = nativeStage{Status: "failed", Reason: "Supported repair refused cache damage: " + string(repair)}
	} else {
		nativeSession(t, f, client, "native_cache_repair", "C", "read", stages)
		stages["native_cache_repair"] = nativeStage{Status: "passed"}
	}
	nativeAttempt(stages, "partial_failure", "partial-failure-installer.json")
	var partialMCP map[string]any
	partialBody, err := os.ReadFile(filepath.Join(f.Package, "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(partialBody, &partialMCP); err != nil {
		t.Fatal(err)
	}
	partialServers := partialMCP["mcpServers"].(map[string]any)
	partialServers["unavailable"] = map[string]any{"type": "stdio", "command": "uap-deliberately-unavailable-" + f.Nonce}
	partialServers["startup-failure"] = map[string]any{"type": "stdio", "command": "./bin/probe", "args": []string{"--fail-startup"}, "env": map[string]string{"UAP_TEST_ROOT": f.Root, "UAP_TEST_EVENTS": f.Events, "UAP_TEST_NONCE": f.Nonce}}
	partialServers["unsupported-sse"] = map[string]any{"type": "sse", "url": f.HTTPURL + "/unsupported-sse"}
	nativeJSON(t, filepath.Join(f.Package, "mcp.json"), partialMCP)
	nativeWrite(t, filepath.Join(f.Package, "skills", "invalid-proof", "SKILL.md"), []byte("No required skill frontmatter"), 0600)
	partialOutput := nativeInstaller(t, f, installer, client, "partial-failure", "update", "native-proof")
	if !bytes.Contains(partialOutput, []byte("declared_sse_not_supported_by_client")) || !bytes.Contains(partialOutput, []byte("stdio_runtime_unavailable")) {
		t.Fatal("installer did not distinguish planner-unsupported SSE")
	}
	nativeSession(t, f, client, "partial_failure", "C", "read", stages)
	startupEvents, err := os.ReadFile(f.Events)
	if err != nil {
		t.Fatal(err)
	}
	startupObserved := false
	for _, line := range bytes.Split(startupEvents, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var event struct {
			Method string `json:"method"`
			Nonce  string `json:"nonce"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		if event.Method == "startup-failed" && event.Nonce == f.Nonce {
			startupObserved = true
		}
	}
	if !startupObserved {
		t.Fatal("native startup failure had no correlated fixture process event")
	}

	stages["partial_failure"] = nativeStage{Status: "passed", Reason: "UAP omitted unavailable runtime, unsupported SSE and invalid skill; an existing executable failed at native startup independently while healthy sibling tools and skill remained usable", Artifacts: []string{"partial-failure-installer.json", "partial_failure-rpc.jsonl"}}
	nativeAttempt(stages, "all_unsupported", "all-unsupported.log")
	unsupportedState, unsupportedManaged, unsupportedCache := nativeSHA(t, statePath), nativeTreeDigest(t, active), nativeTreeDigest(t, c.Root)
	unsupportedConfig := nativeSHA(t, filepath.Join(f.CodexHome, "config.toml"))
	skillsBackup := filepath.Join(f.Root, "unsupported-skills-backup")
	if err := os.Rename(filepath.Join(f.Package, "skills"), skillsBackup); err != nil {
		t.Fatal(err)
	}
	nativeJSON(t, filepath.Join(f.Package, "mcp.json"), map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": map[string]any{"unsupported-sse": partialServers["unsupported-sse"]}})
	unsupportedOut, unsupportedErr := nativeCommand(t, f, installer, "all-unsupported", "update", "native-proof", "--target", "codex", "--format", "json")
	unsupportedStderr, err := os.ReadFile(filepath.Join(f.Root, "all-unsupported-stderr.log"))
	if err != nil {
		t.Fatal(err)
	}
	if unsupportedErr == nil || !strings.Contains(string(unsupportedStderr), "target codex is unsupported; group preflight caused no mutation") {
		t.Fatalf("expected exact all-unsupported refusal: %v %s", unsupportedErr, unsupportedStderr)
	}
	if err := nativeUnmutatedPreflightResult(unsupportedOut); err != nil {
		t.Fatal(err)
	}
	if nativeSHA(t, statePath) != unsupportedState || nativeTreeDigest(t, active) != unsupportedManaged || nativeTreeDigest(t, c.Root) != unsupportedCache || nativeSHA(t, filepath.Join(f.CodexHome, "config.toml")) != unsupportedConfig {
		t.Fatal("all-unsupported refusal mutated installed surfaces")
	}
	stages["all_unsupported"] = nativeStage{Status: "passed", Reason: "SSE-only update refused in preflight with mutated=false; state, managed/cache bytes and native config unchanged", Artifacts: []string{"all-unsupported.log", "all-unsupported-stderr.log"}}
	// Restore only the source fixture bytes; this reset is not repair evidence.
	if err := os.Rename(skillsBackup, filepath.Join(f.Package, "skills")); err != nil {
		t.Fatal(err)
	}
	nativeJSON(t, filepath.Join(f.Package, "mcp.json"), partialMCP)
	nativeAttempt(stages, "remove", "remove-installer.json")
	// Codex has no supported CLI verb for UAP to silently uninstall a plugin
	// on the user's behalf (see usecase/remove.go and remove_group.go); every
	// real remove against Codex must be an acknowledged external uninstall.
	nativeInstaller(t, f, installer, client, "remove", "remove", "native-proof", "--external-uninstalled")
	// UAP's own directory removal must be immediate and independent of
	// whatever a later fresh app-server session reports; this isolates that
	// claim from the app-server-level check below (nativeAssertAbsent), which
	// caught a real regression where Codex's own config.toml retained a
	// stale enabled-plugin entry and a fresh app-server silently
	// re-materialized the plugin from its original source (fixed in
	// providers/activator.go's removeCodexPlugin).
	if _, statErr := os.Lstat(active); statErr == nil {
		t.Fatalf("managed active path still exists immediately after remove, before any fresh app-server session: %s", active)
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("stat active path after remove: %v", statErr)
	}
	b, err = nativeCommand(t, f, client, "removed-inventory", "plugin", "list", "--json")
	if err != nil || nativeHasPluginID(t, b, fmt.Sprint(evidence["native_plugin_id"])) {
		t.Fatalf("native remove not verified: %v %s", err, b)
	}
	marker, err := os.ReadFile(filepath.Join(a.Data, "native-marker.txt"))
	if err != nil || string(marker) != "persist-"+f.Nonce {
		t.Fatal("remove lost native data marker")
	}
	nativeAssertAbsent(t, f, client, fmt.Sprint(evidence["native_plugin_id"]), "removed")
	stages["remove"] = nativeStage{Status: "passed", Reason: "New CLI/app-server inventory excludes plugin; native data retained"}
	nativeAttempt(stages, "foreign_preservation", "removed-rpc.jsonl")
	for path, body := range foreign {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("foreign bytes changed: %s", path)
		}
	}
	stages["foreign_preservation"] = nativeStage{Status: "passed", Reason: "Foreign skill/cache/marketplace sentinel bytes retained"}
	nativeAttempt(stages, "repeat_remove", "repeat-remove.log")
	repeat, repeatErr := nativeCommand(t, f, installer, "repeat-remove", "remove", "native-proof", "--external-uninstalled", "--target", "codex", "--format", "json")
	repeatStderr, readErr := os.ReadFile(filepath.Join(f.Root, "repeat-remove-stderr.log"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !nativeRepeatRemoveExpected(append(append([]byte{}, repeat...), repeatStderr...), repeatErr) {
		stages["repeat_remove"] = nativeStage{Status: "failed", Reason: fmt.Sprintf("unexpected repeated removal error: %v; %s", repeatErr, repeat)}
		t.Fatal(stages["repeat_remove"].Reason)
	}
	for path, body := range foreign {
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("repeat remove changed foreign bytes: %s", path)
		}
	}
	nativeAssertAbsent(t, f, client, fmt.Sprint(evidence["native_plugin_id"]), "repeat-removed")
	marker, err = os.ReadFile(filepath.Join(a.Data, "native-marker.txt"))
	if err != nil || string(marker) != "persist-"+f.Nonce {
		t.Fatal("repeated removal lost native data marker")
	}
	stages["repeat_remove"] = nativeStage{Status: "passed", Reason: "Expected idempotent/not-found outcome; new native inventory absent and foreign/data bytes preserved"}
	nativeAttempt(stages, "immutable_git", "immutable-git.json")
	nativeJSON(t, filepath.Join(f.Root, "immutable-git.json"), nativeImmutableAcquisition(t, f))
	stages["immutable_git"] = nativeStage{Status: "passed", Reason: "Exact-SHA production acquisition with synthetic local Git transport; acquisition only, not native Git E2E", Artifacts: []string{"immutable-git.json"}}
	nativeCheckHTTP(t, f, stages)
	stages["http"] = stages["A_http"]
	stages["immutable_git_native"] = nativeStage{Status: "not_evaluated", Reason: "CLI admits immutable GitHub sources only and disables Git config rewrites; synthetic supported transport seam proves acquisition separately"}
	for _, name := range []string{"real_model_skill_use", "oauth"} {
		stages[name] = nativeStage{Status: "not_evaluated", Reason: "No authenticated provider or real model requested"}
	}
}

func nativeFindString(v any, key string) string {
	switch value := v.(type) {
	case map[string]any:
		if s, ok := value[key].(string); ok && s != "" {
			return s
		}
		for _, child := range value {
			if s := nativeFindString(child, key); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range value {
			if s := nativeFindString(child, key); s != "" {
				return s
			}
		}
	}
	return ""
}
func nativeRequireContained(t *testing.T, root, path string) {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		t.Fatalf("path escapes disposable fixture: %s", path)
	}
}
func nativeAssertAbsent(t *testing.T, f *nativeFixture, client, pluginID, label string) {
	t.Helper()
	r := startNativeRPC(t, f, client, label)
	defer r.close()
	r.must(t, "initialize", map[string]any{"clientInfo": map[string]string{"name": "uap_native_fixture", "version": "1.0.0"}, "capabilities": map[string]any{"experimentalApi": true}})
	if err := json.NewEncoder(r.in).Encode(map[string]any{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	result := r.must(t, "mcpServerStatus/list", map[string]any{"limit": 100})
	if nativeHasPluginID(t, result, pluginID) {
		t.Fatalf("removed plugin still in new app-server: %s", result)
	}
	skills := r.must(t, "skills/list", map[string]any{"cwds": []string{f.Project}, "forceReload": true})
	if nativeHasPluginID(t, skills, pluginID) {
		t.Fatal("removed plugin skill still discovered")
	}
}
func nativeCheckHTTP(t *testing.T, f *nativeFixture, stages map[string]nativeStage) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(f.Root, "http-events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	seen, redirect, leaked, redirectSource := false, false, false, false
	negotiated := ""
	stages["generated_protocol_header_priority"] = nativeStage{Status: "not_evaluated", Reason: "No post-negotiation request with a generated protocol header observed"}
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var e struct {
			Origin     string `json:"origin"`
			Method     string `json:"method"`
			URI        string `json:"uri"`
			Probe      string `json:"probe_header"`
			Literal    string `json:"literal_header"`
			Protocol   string `json:"protocol"`
			RPCMethod  string `json:"rpc_method"`
			Negotiated string `json:"negotiated_protocol"`
		}
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatal(err)
		}
		if e.Origin == "protocol-response" {
			negotiated = e.Negotiated
			continue
		}
		if strings.HasPrefix(e.URI, "/mcp?") && e.RPCMethod != "initialize" && e.RPCMethod != "" && negotiated != "" {
			previous := stages["generated_protocol_header_priority"]
			if e.Protocol != "" && e.Protocol != negotiated {
				stages["generated_protocol_header_priority"] = nativeStage{Status: "failed", Reason: "Post-negotiation header " + e.Protocol + " differs from negotiated " + negotiated}
			} else if e.Protocol == negotiated && previous.Status != "failed" {
				stages["generated_protocol_header_priority"] = nativeStage{Status: "passed", Reason: "Post-negotiation header exactly matches negotiated " + negotiated}
			}
		}
		if e.Origin == "declared-source" && e.URI == "/redirect" && e.Probe == f.Nonce {
			redirectSource = true
		}
		if e.Origin == "redirect-destination" {
			redirect = true
			if e.Probe != "" {
				leaked = true
			}
		}
		if !seen && strings.HasPrefix(e.URI, "/mcp?") {
			seen = true
			if e.Method != "POST" || e.Probe != f.Nonce || e.Literal != "${UNKNOWN}" || e.URI != "/mcp?literal=%24%7BUNKNOWN%7D" {
				stages["http_request_contract"] = nativeStage{Status: "failed", Reason: string(line)}
			} else {
				stages["http_request_contract"] = nativeStage{Status: "passed"}
			}

		}
	}
	if leaked {
		stages["redirect_headers"] = nativeStage{Status: "failed", Reason: "Configured custom test header reached another loopback origin"}
	} else if redirect && redirectSource {
		stages["redirect_headers"] = nativeStage{Status: "passed", Reason: "Redirect destination received no configured test header"}
	} else if redirectSource && !redirect {
		stages["redirect_headers"] = nativeStage{Status: "passed", Reason: "Configured request reached redirect source; client did not follow to another origin during completed sessions", Artifacts: []string{"http-events.jsonl"}}
	} else {
		stages["redirect_headers"] = nativeStage{Status: "not_evaluated", Reason: "No correlated source/destination redirect exchange; containment not proven"}
	}
}

func nativeAttempt(stages map[string]nativeStage, name, artifact string) {
	stages[name] = nativeStage{Status: "failed", Reason: "Attempt started but did not complete its assertions; inspect the test failure and transcript", Artifacts: []string{artifact}}
}
func nativeRepeatRemoveExpected(output []byte, err error) bool {
	if err == nil {
		return true
	}
	message := strings.TrimSpace(string(output))
	return message == `agentplugins: installation "native-proof" was not found` || message == `agentplugins: plugin is not installed for target "codex" in user scope; no target was changed`
}

func nativeValidateInstallerJSON(b []byte) error {
	var envelope struct {
		Schema  int             `json:"schema_version"`
		Command string          `json:"command"`
		Result  string          `json:"result"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return err
	}
	if envelope.Schema != 1 || envelope.Command == "" || envelope.Result != "success" || len(envelope.Data) == 0 {
		return fmt.Errorf("installer did not report a successful versioned JSON envelope: %s", b)
	}
	return nil
}

func nativeValidateMCPSelections(v any) error {
	selected := map[string]bool{}
	var visit func(any)
	visit = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			if node["kind"] == "mcp_server" && (node["support"] == "projected" || node["support"] == "native") {
				name, _ := node["name"].(string)
				selected[name] = true
			}
			for _, child := range node {
				visit(child)
			}
		case []any:
			for _, child := range node {
				visit(child)
			}
		}
	}
	visit(v)
	if len(selected) != 4 || !selected["default"] || !selected["explicit"] || !selected["http"] || !selected["redirect"] {
		return fmt.Errorf("installer plan did not select exact fixture MCP inventory: %v", selected)
	}
	return nil
}

// Pinned Codex namespace.rs qualifies skills as plugin-name:base-name. MCP
// inventory instead preserves raw server keys; attribution is separate pluginId.
func nativeSkillIdentity(name, pluginID, path string, enabled bool, expectedPluginID, cacheRoot string) bool {
	return enabled && name == "native-proof:native-proof" && pluginID == expectedPluginID && path == filepath.Join(cacheRoot, "skills", "native-proof", "SKILL.md")
}

func nativeHasPluginID(t *testing.T, b []byte, id string) bool {
	t.Helper()
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		t.Fatal(err)
	}
	var has func(any) bool
	has = func(v any) bool {
		switch node := v.(type) {
		case map[string]any:
			if node["pluginId"] == id {
				return true
			}
			for _, child := range node {
				if has(child) {
					return true
				}
			}
		case []any:
			for _, child := range node {
				if has(child) {
					return true
				}
			}
		}
		return false
	}
	return has(value)
}

func nativeUnchangedResult(b []byte) error {
	if err := nativeValidateInstallerJSON(b); err != nil {
		return err
	}
	var envelope struct {
		Data struct {
			Result struct {
				Mutated *bool `json:"mutated"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return err
	}
	if envelope.Data.Result.Mutated == nil || *envelope.Data.Result.Mutated {
		return fmt.Errorf("same-source add did not explicitly report data.result.mutated=false")
	}
	return nil
}
func nativeRepairGuardResult(stdout, stderr []byte, commandErr error) error {
	if commandErr == nil || !strings.Contains(string(stderr), "native identity ownership is indeterminate; refusing repair") {
		return fmt.Errorf("expected ownership guard refusal, got %v; %s", commandErr, stderr)
	}
	return nativeUnmutatedPreflightResult(stdout)
}
func nativeCollisionGuardResult(stdout, stderr []byte, commandErr error) error {
	const expected = "agentplugins: group repair preflight failed; no target was changed: observe prepared identity for codex: native package has no recognized authoritative manifest"
	lines := strings.Split(strings.TrimSpace(string(stderr)), "\n")
	if commandErr == nil || lines[len(lines)-1] != expected {
		return fmt.Errorf("expected manifest-less foreign-directory refusal, got %v; %s", commandErr, stderr)
	}
	return nativeUnmutatedPreflightResult(stdout)
}
func nativeUnmutatedPreflightResult(stdout []byte) error {
	var result struct {
		Result string `json:"result"`
		Data   struct {
			Status  string `json:"status"`
			Targets []struct {
				Output struct {
					Result struct {
						Mutated *bool `json:"mutated"`
					} `json:"result"`
				} `json:"output"`
			} `json:"targets"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout, &result); err != nil {
		return err
	}
	if result.Result != "failure" || result.Data.Status != "preflight_failed" || len(result.Data.Targets) != 1 {
		return fmt.Errorf("unexpected ownership refusal envelope")
	}
	m := result.Data.Targets[0].Output.Result.Mutated
	if m == nil || *m {
		return fmt.Errorf("ownership refusal did not explicitly preserve targets")
	}
	return nil
}

// Public exact-SHA skill-only acquisition. The outer disposable-container runner
// must disconnect the network and acknowledge the nonce before native discovery.
func TestAgentpluginsCodexImmutableGitDiscovery(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CODEX_GIT_E2E") != "1" {
		t.Skip("opt-in public immutable Git native discovery")
	}
	if runtime.GOOS != "linux" || os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_LINUX") != "1" {
		t.Fatal("requires disposable Linux boundary")
	}
	if _, err := os.Lstat("/etc/codex"); !os.IsNotExist(err) {
		t.Fatal("system Codex config must be absent")
	}
	gate := os.Getenv("AGENTPLUGINS_GIT_GATE_DIR")
	if !filepath.IsAbs(gate) {
		t.Fatal("AGENTPLUGINS_GIT_GATE_DIR must be an absolute fresh outer-runner directory")
	}
	if info, err := os.Lstat(gate); err == nil && !info.IsDir() {
		t.Fatal("Git gate must be a real directory")
	}
	if entries, err := os.ReadDir(gate); err == nil {
		if len(entries) != 0 {
			t.Fatal("Git gate directory must be empty")
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, name := range []string{"git-acquisition-complete", "git-network-isolated"} {
		if _, err := os.Lstat(filepath.Join(gate, name)); !os.IsNotExist(err) {
			t.Fatal("Git gate must be fresh and absent")
		}
	}
	client, installer := nativeBinary(t, "AGENTPLUGINS_CODEX_BIN"), nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	f := newNativeFixture(t, true)
	f.ClientBinDir = filepath.Dir(client)
	stages := map[string]nativeStage{"git_acquisition": {Status: "not_evaluated"}, "native_skill_discovery": {Status: "not_evaluated"}, "native_mcp_from_git": {Status: "not_evaluated", Reason: "Reviewed public fixture contains a skill only"}}
	evidence, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	stages["real_model_skill_use"] = nativeStage{Status: "not_evaluated", Reason: "Skill discovery only; no real model invocation"}
	stages["oauth"] = nativeStage{Status: "not_evaluated", Reason: "Credential-free fixture"}
	stages["remove"] = nativeStage{Status: "not_evaluated", Reason: "not reached"}
	evidence["os"] = runtime.GOOS
	evidence["arch"] = runtime.GOARCH
	evidence["stages"] = stages
	evidence["profile"] = f.Root
	evidence["client_sha256"] = nativeSHA(t, client)
	evidence["installer_sha256"] = nativeSHA(t, installer)
	evidence["started_utc"] = time.Now().UTC().Format(time.RFC3339Nano)
	defer func() {
		evidence["finished_utc"] = time.Now().UTC().Format(time.RFC3339Nano)
		hashes := map[string]string{}
		entries, _ := os.ReadDir(f.Root)
		for _, entry := range entries {
			if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".log") || strings.HasSuffix(entry.Name(), ".jsonl") || strings.HasSuffix(entry.Name(), "-installer.json")) {
				hashes[entry.Name()] = nativeSHA(t, filepath.Join(f.Root, entry.Name()))
			}
		}
		evidence["artifact_sha256"] = hashes
		nativeJSON(t, filepath.Join(f.Root, "evidence.json"), evidence)
	}()
	nativeWrite(t, filepath.Join(f.CodexHome, "config.toml"), []byte("cli_auth_credentials_store = \"file\"\nmcp_oauth_credentials_store = \"file\"\n[analytics]\nenabled = false\n[feedback]\nenabled = false\n"), 0600)
	version, err := nativeCommand(t, f, client, "client-version", "--version")
	if err != nil || strings.TrimSpace(string(version)) != "codex-cli 0.153.4" {
		t.Fatalf("wrong client %s %v", version, err)
	}
	evidence["client_version"] = "0.153.4"
	evidence["client_source"] = "3d2ee51ca2d5db578f328aa75e20aa22c0197c9a"
	evidence["security_scanner"] = nativeProvisionScanner(t, f)
	const revision = "4d163c6281a9b4929f482f387efe340b4dc175a1"
	const source = "Booyaka101/agent-plugins-conformance-kit@" + revision + "//fixtures/core/AP-6.2-MISSING-LOCATION-OK/plugin"
	evidence["source"] = source
	nativeAttempt(stages, "git_acquisition", "git-add-installer.json")
	output := nativeInstaller(t, f, installer, client, "git-add", "add", source)
	var result any
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	physical := nativeFindString(result, "physical_artifact_id")
	if physical == "" {
		t.Fatal("missing native physical identity")
	}
	f.PluginID = "demo@" + providers.ManagedMarketplaceName(physical)
	stateBytes, err := os.ReadFile(filepath.Join(f.Root, "installer-state", "state-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state any
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		t.Fatal(err)
	}
	if nativeFindString(state, "resolved_revision") != revision {
		t.Fatal("installer source is not exact pinned Git revision")
	}
	active := nativeFindString(state, "target_locator")
	nativeRequireContained(t, f.Root, active)
	const manifestSHA = "c9036920ac155cbb4d0a09deca484a98e64071677df431dfd897db855880d3ad"
	if nativeSHA(t, filepath.Join(active, "plugin.json")) != manifestSHA {
		t.Fatal("acquired manifest does not match reviewed pinned fixture")
	}
	evidence["reviewed_git_subtree"] = "ea758f6ddb033f2b3eb2db138212ab7525e1ce7c"
	evidence["managed_manifest_sha256"] = nativeSHA(t, filepath.Join(active, "plugin.json"))
	evidence["source_tree_digest"] = nativeFindString(result, "tree_digest")
	evidence["source_manifest_digest"] = nativeFindString(result, "manifest_digest")
	evidence["resolved_revision"] = revision
	stages["git_acquisition"] = nativeStage{Status: "passed", Reason: "Actual CLI immutable public GitHub acquisition without Directory", Artifacts: []string{"git-add-installer.json"}}
	nativeWrite(t, filepath.Join(gate, "git-acquisition-complete"), []byte(f.Nonce), 0600)
	timer := time.NewTimer(55 * time.Second)
	defer timer.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	isolated := false
	for !isolated {
		select {
		case <-timer.C:
			t.Fatal("outer runner did not acknowledge network isolation within 55s")
		case <-tick.C:
			path := filepath.Join(gate, "git-network-isolated")
			info, err := os.Lstat(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil || !info.Mode().IsRegular() {
				t.Fatal("isolation gate must be regular file")
			}
			body, err := os.ReadFile(path)
			if err != nil || string(body) != f.Nonce {
				t.Fatal("isolation gate nonce mismatch")
			}
			isolated = true
		}
	}
	evidence["network_boundary"] = "outer runner disconnected container network after acquisition and acknowledged per-run nonce before app-server"
	nativeAttempt(stages, "native_skill_discovery", "git-native-rpc.jsonl")
	r := startNativeRPC(t, f, client, "git-native")
	defer r.close()
	r.must(t, "initialize", map[string]any{"clientInfo": map[string]string{"name": "uap_native_fixture", "version": "1.0.0"}, "capabilities": map[string]any{"experimentalApi": true}})
	if err := json.NewEncoder(r.in).Encode(map[string]any{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	var listing struct {
		Data []struct {
			Skills []struct {
				Name     string `json:"name"`
				Path     string `json:"path"`
				PluginID string `json:"pluginId"`
				Enabled  bool   `json:"enabled"`
			} `json:"skills"`
		} `json:"data"`
	}
	if err := json.Unmarshal(r.must(t, "skills/list", map[string]any{"cwds": []string{f.Project}, "forceReload": true}), &listing); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, group := range listing.Data {
		for _, skill := range group.Skills {
			if skill.Name != "demo:alpha" || skill.PluginID != f.PluginID || !skill.Enabled {
				continue
			}
			nativeRequireContained(t, f.Root, skill.Path)
			if !strings.Contains(skill.Path, "/plugins/cache/") || !strings.HasSuffix(skill.Path, "/skills/alpha/SKILL.md") {
				t.Fatal("skill is not from installed native cache")
			}
			body, err := os.ReadFile(skill.Path)
			if err != nil || string(body) != "---\nname: alpha\ndescription: Fixture skill alpha for the Agent Plugins conformance kit.\n---\n\nFixture body.\n" || nativeSHA(t, skill.Path) != "f77d30f91adee306136e36b0bedd535a826db3b2884b6dfbd5ba8f3d548efd3b" {
				t.Fatal("native Git skill body mismatch")
			}
			evidence["native_skill_sha256"] = nativeSHA(t, skill.Path)
			evidence["native_plugin_id"] = skill.PluginID
			evidence["native_skill_path"] = skill.Path
			found = true
		}
	}
	if !found {
		t.Fatal("exact Git plugin-attributed skill absent from native discovery")
	}
	stages["native_skill_discovery"] = nativeStage{Status: "passed", Reason: "Fresh isolated native app-server discovered exact plugin-attributed demo:alpha installed cache body; no model use or MCP execution claimed", Artifacts: []string{"git-native-rpc.jsonl"}}
	r.close()
	nativeAttempt(stages, "remove", "git-remove-installer.json")
	nativeInstaller(t, f, installer, client, "git-remove", "remove", "demo", "--external-uninstalled")
	if _, err := os.Lstat(active); !os.IsNotExist(err) {
		t.Fatal("Git-installed managed directory remains after remove")
	}
	nativeAssertAbsent(t, f, client, f.PluginID, "git-removed")
	stages["remove"] = nativeStage{Status: "passed", Reason: "Normal remove deleted managed artifact; fresh app-server excludes plugin tools/skills", Artifacts: []string{"git-remove-installer.json", "git-removed-rpc.jsonl"}}

}
