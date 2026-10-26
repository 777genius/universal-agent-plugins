//go:build linux

package pluginkitairepo_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// The acquisition identity is the unchanged typed Kimi contract: client:string,
// acquisition:{sha256,pin_verified,members:[{name,kind,bytes,sha256}]},
// runtime_files:[{path,sha256}]. Only the existing exact 2.3.2 bytes are admitted.
// This separate constructor establishes its 150-second context BEFORE any
// fixture construction, hashing, provisioning or public/native command.
func newKimiSingleToolFixture(t *testing.T) *kimiProjectNativeFixture {
	t.Helper()
	started := time.Now()
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED") != "1" || os.Getuid() <= 0 || os.Geteuid() != os.Getuid() || os.Getgid() <= 0 || os.Getegid() != os.Getgid() {
		t.Fatal("Kimi native execution requires actual opted-in GitHub-hosted Linux amd64")
	}
	f := &kimiProjectNativeFixture{started: started, packages: map[string]*kimiProjectNativePackage{}, immutable: map[string]string{}, protected: map[string]string{}, files: map[string][]byte{}, serverIDs: map[string]bool{}, foreignMCP: map[string]any{}, protectedTrees: map[string]map[string]string{}, evidence: map[string]any{"client": "kimi", "activation": "prepared", "qualification": map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}, "vendor_authentication_qualified": false, "real_model_qualified": false}}
	f.ctx, f.cancel = context.WithDeadline(context.Background(), started.Add(150*time.Second))
	f.client, f.installer = nativeBinary(t, "AGENTPLUGINS_KIMI_BIN"), nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	f.python = nativeBinary(t, "AGENTPLUGINS_PROJECT_NATIVE_PYTHON")
	var err error
	f.git, err = exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	f.git, err = filepath.EvalSymlinks(f.git)
	if err != nil {
		t.Fatal(err)
	}
	f.output = os.Getenv("AGENTPLUGINS_PROJECT_NATIVE_OUTPUT")
	if err := projectNativeRuntimeParents(filepath.Join(f.output, "leaf")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(f.output)
	if err != nil || len(entries) != 0 {
		t.Fatal("Kimi output must be an existing empty ordinary directory")
	}
	body, err := projectNativeRuntimeRead(os.Getenv("AGENTPLUGINS_PROJECT_NATIVE_IDENTITY"), kimiProjectNativeLimit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kimiProjectObject(body); err != nil {
		t.Fatal(err)
	}
	var identity struct {
		Client      string `json:"client"`
		Acquisition struct {
			SHA     string `json:"sha256"`
			Pin     bool   `json:"pin_verified"`
			Members []struct {
				Name, Kind string
				Bytes      int64
				SHA        string `json:"sha256"`
			} `json:"members"`
		} `json:"acquisition"`
		RuntimeFiles []struct {
			Path string `json:"path"`
			SHA  string `json:"sha256"`
		} `json:"runtime_files"`
	}
	if json.Unmarshal(body, &identity) != nil || identity.Client != "kimi" || identity.Acquisition.SHA != kimiProjectArchiveSHA || !identity.Acquisition.Pin || len(identity.Acquisition.Members) != 1 || len(identity.RuntimeFiles) != 1 {
		t.Fatal("Kimi exact immutable acquisition receipt differs")
	}
	member := identity.Acquisition.Members[0]
	if member.Name != "kimi" || member.Kind != "file" || member.Bytes != 192416960 || member.SHA != kimiProjectExecutableSHA || identity.RuntimeFiles[0].Path != f.client || identity.RuntimeFiles[0].SHA != member.SHA || filepath.Base(f.client) != "kimi" {
		t.Fatal("Kimi exact executable member differs")
	}
	f.immutable[f.client] = member.SHA
	for _, path := range []string{f.installer, f.python, f.git, nativeBinary(t, "AGENTPLUGINS_NATIVE_PROBE_BIN"), nativeBinary(t, "AGENTPLUGINS_LINTAI_BIN"), filepath.Join(RepoRoot(t), "scripts", "project_native_collectors", "kimi.py"), filepath.Join(RepoRoot(t), "scripts", "project_native_service.py")} {
		digest, err := projectNativeRuntimeDigest(path)
		if err != nil {
			t.Fatal(err)
		}
		f.immutable[path] = digest
	}
	identitySource, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	f.evidence["source_identity"], f.evidence["acquisition"] = identitySource, identity
	f.check(t)
	f.base = newNativeFixture(t, false)
	f.base.ClientBinDir = filepath.Dir(f.client)
	f.projects = map[string]string{"a": f.base.Project, "b": filepath.Join(f.base.Root, "project-b"), "c": filepath.Join(f.base.Root, "project-c"), "user": filepath.Join(f.base.Root, "outside")}
	for key, path := range f.projects {
		if key != "a" {
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
		}
	}
	f.immutable[f.base.Probe] = nativeSHA(t, f.base.Probe)
	for _, name := range []string{"project_native_research.py", "native_client_research.py", "native_client_project_artifacts.py", "research-grok-native.py", "run-native-client-matrix.py", "native_client_draft.py"} {
		path := filepath.Join(RepoRoot(t), "scripts", name)
		digest, err := projectNativeRuntimeDigest(path)
		if err != nil {
			t.Fatal(err)
		}
		f.immutable[path] = digest
	}
	f.evidence["scanner"] = nativeProvisionScanner(t, f.base)
	return f
}

type kimiSingleToolAttempt struct {
	f              *kimiProjectNativeFixture
	provider       *kimiAttemptProvider
	secret, marker string
	facts          map[string]any
}

func (a *kimiSingleToolAttempt) validateFacts(body string) error {
	value, err := projectNativeRuntimeJSON([]byte(body))
	if err != nil {
		return err
	}
	facts, ok := value.(map[string]any)
	if !ok || projectNativeRuntimeKeys(facts, "nonce revision cwd argv plugin_root plugin_data literal once marker pid session", "") != nil {
		return fmt.Errorf("Kimi returned facts shape differs")
	}
	f, p := a.f, a.f.packages["a"]
	want := map[string]any{"nonce": f.base.Nonce + "-a", "revision": "a-one", "cwd": p.Binding.TargetLocator,
		"argv": []any{"Kimi owned a", p.Binding.TargetLocator + "/argument"}, "plugin_root": p.Binding.TargetLocator,
		"plugin_data": p.Data, "literal": "", "once": "", "marker": a.marker}
	for key, expected := range want {
		if !reflect.DeepEqual(facts[key], expected) {
			return fmt.Errorf("Kimi returned facts authority differs: %s", key)
		}
	}
	if !projectNativeRuntimeNumber(facts["pid"], 1, 1<<30, true) || !projectNativeRuntimeText(facts["session"], 256) || facts["session"] == "" {
		return fmt.Errorf("Kimi probe process/session missing")
	}
	bodyEvents, err := projectNativeRuntimeRead(filepath.Join(f.base.Root, "a-events.jsonl"), 1<<20)
	if err != nil || !bytes.HasSuffix(bodyEvents, []byte{'\n'}) {
		return fmt.Errorf("Kimi fresh probe events absent")
	}
	counts := map[string]int{}
	last := time.Time{}
	for _, line := range bytes.Split(bytes.TrimSuffix(bodyEvents, []byte{'\n'}), []byte{'\n'}) {
		value, err := projectNativeRuntimeJSON(line)
		if err != nil {
			return err
		}
		event, ok := value.(map[string]any)
		if !ok || projectNativeRuntimeKeys(event, "method pid session nonce timestamp facts", "") != nil || event["pid"] != facts["pid"] || event["session"] != facts["session"] || event["nonce"] != facts["nonce"] {
			return fmt.Errorf("Kimi probe event identity differs")
		}
		method, _ := event["method"].(string)
		stamp, _ := event["timestamp"].(string)
		when, err := time.Parse(time.RFC3339Nano, stamp)
		if err != nil || when.Before(f.started) || when.After(time.Now()) || when.Before(last) || counts["tools/call"] != 0 {
			return fmt.Errorf("Kimi probe event time/order differs")
		}
		last = when
		switch method {
		case "initialize":
			if counts["initialize"] != 0 || counts["tools/list"] != 0 || event["facts"] != nil {
				return fmt.Errorf("Kimi duplicate initialize")
			}
		case "tools/list":
			if counts["initialize"] != 1 || event["facts"] != nil {
				return fmt.Errorf("Kimi catalog ordering differs")
			}
		case "tools/call":
			if counts["initialize"] != 1 || counts["tools/list"] < 1 || !reflect.DeepEqual(event["facts"], facts) {
				return fmt.Errorf("Kimi tool result/event join differs")
			}
		default:
			return fmt.Errorf("Kimi unexpected probe method")
		}
		counts[method]++
	}
	if counts["initialize"] != 1 || counts["tools/list"] < 1 || counts["tools/list"] > 4 || counts["tools/call"] != 1 {
		return fmt.Errorf("Kimi probe exchange incomplete")
	}
	a.facts = facts
	return nil
}

func (a *kimiSingleToolAttempt) setup(t *testing.T) {
	t.Helper()
	f := a.f
	a.secret, a.marker = "uap-kimi-key-"+f.base.Nonce, "retained-"+f.base.Nonce
	a.provider = &kimiAttemptProvider{key: a.secret, id: "uap_" + f.base.Nonce + "_attempt", nonce: f.base.Nonce + "-a", marker: a.marker, sentinel: "UAP_KIMI_DONE_" + f.base.Nonce, validate: a.validateFacts}
	if err := a.provider.start(); err != nil {
		t.Fatal(err)
	}
	// Register the fixture key in this owner before any command can be captured.
	config := "telemetry = false\nauto_session_title = false\ndefault_permission_mode = \"manual\"\ndefault_model = \"" + kimiAttemptModel + "\"\n" +
		"[providers.uap-fixture]\ntype = \"openai\"\nbase_url = \"" + a.provider.server.URL + "/v1\"\napi_key = \"" + a.secret + "\"\n" +
		"[models." + kimiAttemptModel + "]\nprovider = \"uap-fixture\"\nmodel = \"" + kimiAttemptModel + "\"\nmax_context_size = 32768\nmax_output_size = 1024\ncapabilities = [\"tool_use\"]\n" +
		"[thinking]\nenabled = false\n[loop_control]\nmax_steps_per_turn = 2\nmax_attempts_per_step = 1\ncompaction_max_attempts = 1\n" +
		"[tools]\nenabled = [\"" + kimiAttemptTool + "\"]\n"
	preferences := filepath.Join(f.base.Home, ".kimi-code", "config.toml")
	projectNativeRuntimeWrite(t, preferences, []byte(config), 0600)
	f.protected[preferences] = nativeSHA(t, preferences)
	for _, scope := range []string{"a", "b", "c"} {
		r := f.command(t, "git-"+scope, f.projects[scope], f.env(), []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}, 30*time.Second, false)
		if len(r.Stdout) != 0 || len(r.Stderr) != 0 {
			t.Fatal("Kimi Git baseline emitted diagnostics")
		}
	}
	r := f.command(t, "version", f.projects["user"], f.env(), []string{f.client, "--version"}, 30*time.Second, false)
	if string(r.Stdout) != "2.3.2\n" || len(r.Stderr) != 0 {
		t.Fatal("Kimi exact version differs")
	}
	for _, root := range []string{f.base.Home, f.projects["a"], f.projects["b"], f.projects["c"]} {
		path := filepath.Join(root, ".kimi-code", "foreign-keep.txt")
		projectNativeRuntimeWrite(t, path, []byte("retained original authority\n"), 0600)
		f.protected[path] = nativeSHA(t, path)
	}
	f.source(t, "a", "a-one")
	f.protectedTrees[f.packages["a"].Source] = kimiProjectTree(t, f.packages["a"].Source)
	f.uap(t, "add-a", "a", true, false, "add", f.packages["a"].Source, "--no-save")
	f.receipt(t, "a")
	p := f.packages["a"]
	configPath := filepath.Join(f.projects["a"], ".kimi-code", "mcp.json")
	body, err := projectNativeRuntimeRead(configPath, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	object, err := kimiProjectObject(body)
	if err != nil {
		t.Fatal(err)
	}
	entries, ok := object["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("Kimi installed MCP map absent")
	}
	server, ok := entries[p.Name].(map[string]any)
	if !ok || kimiProjectValidateServer(server, p, "a", f.base.Root, f.base.Nonce) != nil {
		t.Fatal("Kimi installed projection differs from source/receipt")
	}
	f.keep(t, ".kimi-code-mcp.json.installed", body)
	skill, err := projectNativeRuntimeRead(p.Skill, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	f.keep(t, "uap-kimi-native-a-SKILL.md.installed", skill)
	for _, path := range []string{configPath, p.Skill, filepath.Join(f.projects["a"], ".uap", "state-v2.json")} {
		f.protected[path] = nativeSHA(t, path)
	}
	f.protectedTrees[p.Binding.TargetLocator] = kimiProjectTree(t, p.Binding.TargetLocator)
	f.evidence["binding"], f.evidence["data_locator"] = p.Binding, p.Data
	f.check(t)
}

func (a *kimiSingleToolAttempt) run(t *testing.T) {
	t.Helper()
	f, p := a.f, a.f.packages["a"]
	path := filepath.Join(f.base.Root, "attempt-request.json")
	config := filepath.Join(f.projects["a"], ".kimi-code", "mcp.json")
	body, err := projectNativeRuntimeRead(config, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	object, _ := kimiProjectObject(body)
	server := object["mcpServers"].(map[string]any)[p.Name]
	identities := map[string]string{}
	for _, inventory := range []map[string]string{f.immutable, f.protected} {
		for path, digest := range inventory {
			identities[path] = digest
		}
	}
	for root, inventory := range f.protectedTrees {
		for relative, value := range inventory {
			_, digest, ok := strings.Cut(value, ":")
			if ok && digest != "directory" {
				identities[filepath.Join(root, relative)] = digest
			}
		}
	}
	request := map[string]any{"schema_version": 1, "root": f.base.Root, "home": f.base.Home, "projects": f.projects, "client": f.client, "identities": identities,
		"observation": map[string]any{"server": map[string]any{"name": p.Name, "origin": config, "config": server, "sha256": nativeSHA(t, config), "event": filepath.Join(f.base.Root, "a-events.jsonl"), "nonce": f.base.Nonce + "-a", "cwd": f.projects["a"]},
			"skill":   map[string]any{"name": p.Name, "description": "Kimi installed a-one", "path": p.Skill, "source": "project", "sha256": nativeSHA(t, p.Skill)},
			"call_id": a.provider.id, "provider_key": a.secret, "nonce": f.base.Nonce + "-a", "marker": a.marker, "data": p.Data}}
	encoded, _ := json.Marshal(request)
	projectNativeRuntimeWrite(t, path, encoded, 0600)
	f.protected[path] = nativeSHA(t, path)
	dataBefore := kimiProjectTree(t, p.Data)
	r := f.command(t, "native-attempt", f.projects["user"], f.env(), []string{f.python, filepath.Join(RepoRoot(t), "scripts", "project_native_collectors", "kimi_attempt.py"), "--attempt-request", path}, 70*time.Second, false)
	if len(r.Stderr) != 0 {
		t.Fatal("Kimi private observer emitted diagnostics")
	}
	if _, err := projectNativeRuntimeJSON(r.Stdout); err != nil {
		t.Fatal("Kimi private observer evidence is not bounded JSON")
	}
	f.keep(t, "python-api.json", r.Stdout)
	var envelope struct {
		SchemaVersion int               `json:"schema_version"`
		Files         map[string]string `json:"files"`
	}
	if json.Unmarshal(r.Stdout, &envelope) != nil || envelope.SchemaVersion != 1 || envelope.Files["research.json"] == "" {
		t.Fatal("Kimi private API evidence missing")
	}
	var summary map[string]any
	if json.Unmarshal([]byte(envelope.Files["research.json"]), &summary) != nil {
		t.Fatal("Kimi private API summary malformed")
	}
	observation, ok := summary["observations"].(map[string]any)
	if !ok || observation["native_history_grammar_frozen"] != false || observation["provider_call_id"] != a.provider.id {
		t.Fatal("Kimi initial native capture boundary differs")
	}
	for _, key := range []string{"native_session_id", "native_turn_id", "native_capture_complete"} {
		f.evidence[key] = observation[key]
	}
	a.provider.mu.Lock()
	issued, returned, problem, count := a.provider.issued, a.provider.returned, a.provider.err, len(a.provider.requests)
	a.provider.mu.Unlock()
	f.evidence["provider_call_id"], f.evidence["provider_returned"] = a.provider.id, returned
	if problem != nil || !issued || !returned || count != 2 || a.facts == nil {
		t.Fatal("Kimi provider/native probe exchange incomplete; bounded initial evidence retained")
	}
	f.evidence["probe_facts"] = a.facts
	marker, err := projectNativeRuntimeRead(filepath.Join(p.Data, "native-marker.txt"), 256)
	if err != nil || string(marker) != a.marker {
		t.Fatal("Kimi native marker differs")
	}
	after := kimiProjectTree(t, p.Data)
	delete(after, "native-marker.txt")
	if !reflect.DeepEqual(after, dataBefore) {
		t.Fatal("Kimi tool changed unrelated persistent data")
	}
	f.check(t)
	// Even source-guided success is not measured native-history qualification.
	t.Error("Kimi initial native history capture requires independent finite-schema review; no qualification")
}

func (a *kimiSingleToolAttempt) finish(t *testing.T) {
	t.Helper()
	f := a.f
	defer f.cancel()
	if a.provider != nil && a.provider.server != nil {
		a.provider.server.CloseClientConnections()
		a.provider.server.Close()
		a.provider.mu.Lock()
		for index, body := range a.provider.requests {
			f.keep(t, fmt.Sprintf("provider-request-%d.json", index+1), body)
		}
		a.provider.mu.Unlock()
	}
	if time.Since(f.started) > 180*time.Second {
		t.Error("Kimi original runtime attempt total deadline exceeded")
	}
	if body, err := projectNativeRuntimeRead(filepath.Join(f.base.Root, "a-events.jsonl"), 1<<20); err == nil {
		f.keep(t, "probe-events.jsonl", body)
	}
	f.evidence["test_failed"], f.evidence["elapsed_ms"] = t.Failed(), time.Since(f.started).Milliseconds()
	body, _ := json.MarshalIndent(f.evidence, "", "  ")
	f.keep(t, "runtime-attempt.json", append(body, '\n'))
	for name, body := range f.files {
		if a.secret != "" {
			body = bytes.ReplaceAll(body, []byte(a.secret), []byte("[kimi-fixture-key-redacted]"))
		}
		if len(body) > 1<<20 || (a.secret != "" && bytes.Contains(body, []byte(a.secret))) {
			t.Error("Kimi secret/bounded artifact admission failed")
			return
		}
		f.files[name] = body
	}
	if err := kimiAttemptAdmit(f.files); err != nil {
		t.Error(err)
		return
	}
	file, err := os.OpenFile(filepath.Join(f.output, "research.zip"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Error(err)
		return
	}
	archive := zip.NewWriter(file)
	names := make([]string, 0, len(f.files))
	for name := range f.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		writer, err := archive.Create(name)
		if err != nil {
			t.Error(err)
			break
		}
		if _, err = writer.Write(f.files[name]); err != nil {
			t.Error(err)
			break
		}
	}
	if err := archive.Close(); err != nil {
		t.Error(err)
	}
	if err := file.Close(); err != nil {
		t.Error(err)
	}
}

func kimiAttemptAdmit(files map[string][]byte) error {
	allowed := map[string]bool{"runtime-attempt.json": true, "probe-events.jsonl": true, "python-api.json": true,
		".kimi-code-mcp.json.installed": true, "uap-kimi-native-a-SKILL.md.installed": true}
	for index := 1; index <= 8; index++ {
		allowed[fmt.Sprintf("provider-request-%d.json", index)] = true
	}
	labels := []string{"git-a", "git-b", "git-c", "version", "add-a", "native-attempt"}
	for _, label := range labels {
		for _, suffix := range []string{".stdout.log", ".stderr.log", ".command.json"} {
			allowed[label+suffix] = true
		}
	}
	if files["runtime-attempt.json"] == nil || len(files) > len(allowed) {
		return fmt.Errorf("Kimi initial artifact inventory differs")
	}
	total := 0
	for name, body := range files {
		total += len(body)
		if !allowed[name] || len(body) > 1<<20 || bytes.Contains(body, []byte("uap-kimi-key-")) || strings.Contains(name, "uap-kimi-key-") {
			return fmt.Errorf("Kimi unknown/private/unbounded artifact member refused")
		}
	}
	if total > 24<<20 {
		return fmt.Errorf("Kimi initial artifact aggregate exceeds bound")
	}
	for _, label := range labels {
		count := 0
		for _, suffix := range []string{".stdout.log", ".stderr.log", ".command.json"} {
			if _, exists := files[label+suffix]; exists {
				count++
			}
		}
		if count != 0 && count != 3 {
			return fmt.Errorf("Kimi partial command artifact triple refused")
		}
	}
	missing := false
	for index := 1; index <= 8; index++ {
		_, exists := files[fmt.Sprintf("provider-request-%d.json", index)]
		if !exists {
			missing = true
		} else if missing {
			return fmt.Errorf("Kimi provider capture sequence gap refused")
		}
	}
	result, err := kimiProjectObject(files["runtime-attempt.json"])
	if err != nil || result["client"] != "kimi" || result["runtime_attempt_complete"] != false || result["vendor_authentication_qualified"] != false || result["real_model_qualified"] != false || result["test_failed"] != true {
		return fmt.Errorf("Kimi initial capture cannot claim successful qualification")
	}
	flags, ok := result["qualification"].(map[string]any)
	if !ok || len(flags) != 5 {
		return fmt.Errorf("Kimi initial qualification flags absent")
	}
	for _, key := range []string{"project-lifecycle", "native-restart-removal", "mcp-initialize", "mcp-tools-list", "mcp-tools-call"} {
		if flags[key] != false {
			return fmt.Errorf("Kimi initial capture promoted unobserved qualification")
		}
	}
	return nil
}

func TestAgentpluginsKimiNativeProjectRuntimeAttempt(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_KIMI_PROJECT_RUNTIME_ATTEMPT") != "1" {
		t.Skip("opt-in genuine hosted Kimi installed single-tool initial attempt")
	}
	f := newKimiSingleToolFixture(t)
	a := &kimiSingleToolAttempt{f: f}
	f.evidence["version"], f.evidence["phase"], f.evidence["provider_kind"] = "2.3.2", "installed-single-tool-attempt", "scripted-openai-chat-completions"
	f.evidence["runtime_attempt_complete"] = false
	f.evidence["native_source_commit"] = "b8dd3a134060c6e80ba4a1659335cd63b68483f2"
	defer a.finish(t)
	for _, name := range []string{"kimi_attempt.py", "kimi.py"} {
		path := filepath.Join(RepoRoot(t), "scripts", "project_native_collectors", name)
		digest, err := projectNativeRuntimeDigest(path)
		if err != nil {
			t.Fatal(err)
		}
		f.immutable[path] = digest
	}
	a.setup(t)
	a.run(t)
}
