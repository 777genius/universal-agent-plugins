package pluginkitairepo_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type nativeFixture struct {
	Root, Project, Home, CodexHome, Package, Probe, Events, Nonce string
	PluginID, ClientBinDir, HTTPURL, RedirectURL                  string
	ProviderRequests                                              chan []byte
}
type nativeStage struct {
	Status    string   `json:"status"`
	Reason    string   `json:"reason,omitempty"`
	Artifacts []string `json:"artifacts,omitempty"`
}
type nativeFacts struct {
	Nonce    string   `json:"nonce"`
	Revision string   `json:"revision"`
	CWD      string   `json:"cwd"`
	Argv     []string `json:"argv"`
	Root     string   `json:"plugin_root"`
	Data     string   `json:"plugin_data"`
	Literal  string   `json:"literal"`
	Once     string   `json:"once"`
	Marker   string   `json:"marker"`
	PID      int      `json:"pid"`
	Session  string   `json:"session"`
}

func nativeWrite(t *testing.T, path string, b []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, mode); err != nil {
		t.Fatal(err)
	}
}
func nativeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, path, append(b, '\n'), 0600)
}
func nativeSHA(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func newNativeFixture(t *testing.T, preserve bool) *nativeFixture {
	t.Helper()
	root := t.TempDir()
	if preserve {
		var err error
		root, err = os.MkdirTemp("", "uap-native-evidence-")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("native evidence: %s", root)
	}
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	f := &nativeFixture{Root: root, Project: filepath.Join(root, "project"), Home: filepath.Join(root, "home"), CodexHome: filepath.Join(root, "home", ".codex"), Package: filepath.Join(root, "package"), Probe: filepath.Join(root, nativeExecutableName("native-probe")), Events: filepath.Join(root, "events.jsonl"), Nonce: hex.EncodeToString(nonce[:])}
	for _, p := range []string{f.Project, f.Home, f.CodexHome, filepath.Join(root, "tmp"), filepath.Join(root, "xdg-config"), filepath.Join(root, "xdg-data"), filepath.Join(root, "xdg-cache"), filepath.Join(root, "xdg-state")} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if supplied := os.Getenv("AGENTPLUGINS_NATIVE_PROBE_BIN"); supplied != "" {
		supplied = nativeBinary(t, "AGENTPLUGINS_NATIVE_PROBE_BIN")
		b, err := os.ReadFile(supplied)
		if err != nil {
			t.Fatal(err)
		}
		nativeWrite(t, f.Probe, b, 0700)
		return f
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", f.Probe, "./testdata/agentplugins_native_probe")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, b)
	}
	return f
}

// Environment is assembled explicitly. Never inherit auth, proxy, shell, HOME or PATH.
func (f *nativeFixture) env(binDir string) []string {
	clientDir := f.ClientBinDir
	if clientDir == "" {
		clientDir = binDir
	}
	return append(nativePlatformEnvironment(f.Root, f.Home, binDir, clientDir), "AGENTPLUGINS_HOME="+filepath.Join(f.Root, "installer-state"), "CODEX_HOME="+f.CodexHome)
}
func (f *nativeFixture) writePackage(t *testing.T, revision, version string) {
	t.Helper()
	nativeJSON(t, filepath.Join(f.Package, "plugin.json"), map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": "native-proof", "version": version, "description": "Disposable native lifecycle fixture"})
	nativeWrite(t, filepath.Join(f.Package, "skills", "native-proof", "SKILL.md"), []byte("---\nname: native-proof\ndescription: Disposable native lifecycle fixture\n---\n\nUAP_SKILL_BODY_"+revision+"_"+f.Nonce+"\n"), 0600)
	b, err := os.ReadFile(f.Probe)
	if err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, filepath.Join(f.Package, "bin", nativeExecutableName("probe")), b, 0700)
	env := map[string]string{"UAP_TEST_ROOT": f.Root, "UAP_TEST_EVENTS": f.Events, "UAP_TEST_NONCE": f.Nonce, "UAP_TEST_REVISION": revision, "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": "${PLUGIN_ROOT}/${UNKNOWN}"}
	server := func(cwd string) map[string]any {
		m := map[string]any{"type": "stdio", "command": "./bin/../bin/" + nativeExecutableName("probe"), "args": []string{"argument with spaces", "${PLUGIN_ROOT}/argument", "${UNKNOWN}"}, "env": env}
		if cwd != "" {
			m["cwd"] = cwd
		}
		return m
	}
	servers := map[string]any{"default": server(""), "explicit": server("./skills")}
	if f.HTTPURL != "" {
		servers["http"] = map[string]any{"type": "streamable-http", "url": f.HTTPURL + "/mcp?literal=%24%7BUNKNOWN%7D", "headers": map[string]string{"X-UAP-Probe": f.Nonce, "X-UAP-Literal": "${UNKNOWN}", "MCP-Protocol-Version": "harmless-fixture-priority"}}
	}
	if f.RedirectURL != "" {
		servers["redirect"] = map[string]any{"type": "streamable-http", "url": f.RedirectURL, "headers": map[string]string{"X-UAP-Probe": f.Nonce}}
	}
	nativeJSON(t, filepath.Join(f.Package, "mcp.json"), map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": servers})
}
func nativeDecodeFacts(t *testing.T, result json.RawMessage) nativeFacts {
	t.Helper()
	var r struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(result, &r); err != nil || r.IsError || len(r.Content) != 1 {
		t.Fatalf("invalid tool result: %s", result)
	}
	var f nativeFacts
	if err := json.Unmarshal([]byte(r.Content[0].Text), &f); err != nil {
		t.Fatal(err)
	}
	return f
}
func nativeAssertEvent(t *testing.T, f *nativeFixture, facts nativeFacts) {
	t.Helper()
	b, err := os.ReadFile(f.Events)
	if err != nil {
		t.Fatal(err)
	}
	methods := map[string]bool{}
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var e struct {
			Method  string      `json:"method"`
			Session string      `json:"session"`
			Nonce   string      `json:"nonce"`
			Facts   nativeFacts `json:"facts"`
		}
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatal(err)
		}
		if e.Session == facts.Session && e.Nonce == facts.Nonce {
			methods[e.Method] = true
			if e.Method == "tools/call" && e.Facts.Revision != facts.Revision {
				t.Fatal("event/result revision mismatch")
			}
		}
	}
	for _, m := range []string{"initialize", "tools/list", "tools/call"} {
		if !methods[m] {
			t.Fatalf("no correlated %s event", m)
		}
	}
}
func TestAgentpluginsNativeFixtureProtocol(t *testing.T) {
	f := newNativeFixture(t, false)
	f.writePackage(t, "A", "1.0.0")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.Probe, "fixture arg")
	cmd.Dir = f.Project
	cmd.Env = append(f.env(filepath.Dir(f.Probe)), "UAP_TEST_ROOT="+f.Root, "UAP_TEST_EVENTS="+f.Events, "UAP_TEST_NONCE="+f.Nonce, "UAP_TEST_REVISION=A", "PLUGIN_ROOT="+f.Package, "PLUGIN_DATA="+filepath.Join(f.Root, "data"))
	requests := []map[string]any{{"jsonrpc": "2.0", "id": "init-string", "method": "initialize", "params": map[string]any{"protocolVersion": "2025-06-18"}}, {"jsonrpc": "2.0", "method": "notifications/initialized"}, {"jsonrpc": "2.0", "id": 2, "method": "tools/list"}, {"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": map[string]any{"name": "inspect_runtime", "arguments": map[string]string{"nonce": f.Nonce, "operation": "write", "marker": "retained"}}}, {"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": map[string]any{"name": "inspect_runtime", "arguments": map[string]string{"nonce": "wrong"}}}}
	var input bytes.Buffer
	for _, r := range requests {
		if err := json.NewEncoder(&input).Encode(r); err != nil {
			t.Fatal(err)
		}
	}
	cmd.Stdin = &input
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	scan := bufio.NewScanner(bytes.NewReader(b))
	var rows []struct {
		ID     json.RawMessage `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	for scan.Scan() {
		var row struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(scan.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	if len(rows) != 4 || string(rows[0].ID) != `"init-string"` || !bytes.Contains(rows[0].Result, []byte("2025-06-18")) || len(rows[3].Error) == 0 {
		t.Fatalf("protocol rows: %s", b)
	}
	facts := nativeDecodeFacts(t, rows[2].Result)
	if facts.Marker != "retained" || facts.Nonce != f.Nonce || facts.CWD != f.Project {
		t.Fatalf("facts: %+v", facts)
	}
	nativeAssertEvent(t, f, facts)
}
func TestAgentpluginsNativeFixtureRejectsEscapedData(t *testing.T) {
	f := newNativeFixture(t, false)
	outside := t.TempDir()
	link := filepath.Join(f.Root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.Probe)
	cmd.Dir = f.Project
	cmd.Env = append(f.env(filepath.Dir(f.Probe)), "UAP_TEST_ROOT="+f.Root, "UAP_TEST_EVENTS="+f.Events, "UAP_TEST_NONCE="+f.Nonce, "PLUGIN_DATA="+link)
	cmd.Stdin = strings.NewReader(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"inspect_runtime","arguments":{"nonce":%q,"operation":"write","marker":"unsafe"}}}`+"\n", f.Nonce))
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("escapes test root")) {
		t.Fatalf("escape accepted: %s", b)
	}
	if _, err := os.Stat(filepath.Join(outside, "native-marker.txt")); !os.IsNotExist(err) {
		t.Fatal("wrote outside test root")
	}
}

// The HTTP endpoint forwards to our own probe process. Its cwd/env are explicitly
// harness behavior, never evidence of the client's stdio runtime roots.
func nativeHTTP(t *testing.T, f *nativeFixture) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	cmd := exec.CommandContext(ctx, f.Probe)
	cmd.Dir = f.Project
	cmd.Env = append(f.env(filepath.Dir(f.Probe)), "UAP_TEST_ROOT="+f.Root, "UAP_TEST_EVENTS="+f.Events, "UAP_TEST_NONCE="+f.Nonce, "UAP_TEST_REVISION=HTTP", "PLUGIN_ROOT="+f.Package, "PLUGIN_DATA="+filepath.Join(f.Root, "http-data"))
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
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	reader := bufio.NewReader(out)
	var mu sync.Mutex
	log, err := os.Create(filepath.Join(f.Root, "http-events.jsonl"))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	record := func(r *http.Request, origin, rpcMethod string) {
		_ = json.NewEncoder(log).Encode(map[string]any{"timestamp": time.Now().UTC().Format(time.RFC3339Nano), "origin": origin, "rpc_method": rpcMethod, "method": r.Method, "uri": r.URL.RequestURI(), "protocol": r.Header.Get("MCP-Protocol-Version"), "probe_header": r.Header.Get("X-UAP-Probe"), "literal_header": r.Header.Get("X-UAP-Literal")})
	}
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		record(r, "redirect-destination", "")
		http.Error(w, "test destination", http.StatusNotFound)
	}))
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/redirect" {
			record(r, "declared-source", "")
			http.Redirect(w, r, destination.URL+"/cross-origin", http.StatusTemporaryRedirect)
			return
		}
		if r.Method != "POST" {
			record(r, "declared-source", "")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "read", 400)
			return
		}
		var request map[string]json.RawMessage
		if json.Unmarshal(body, &request) != nil {
			http.Error(w, "JSON", 400)
			return
		}
		var rpcMethod string
		_ = json.Unmarshal(request["method"], &rpcMethod)
		record(r, "declared-source", rpcMethod)
		if _, ok := request["id"]; !ok {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		if _, err := in.Write(append(body, '\n')); err != nil {
			http.Error(w, "fixture pipe", 500)
			return
		}
		response, err := reader.ReadBytes('\n')
		if err != nil {
			http.Error(w, "fixture response", 500)
			return
		}
		if rpcMethod == "initialize" {
			var result struct {
				Result struct {
					Protocol string `json:"protocolVersion"`
				} `json:"result"`
			}
			_ = json.Unmarshal(response, &result)
			_ = json.NewEncoder(log).Encode(map[string]string{"origin": "protocol-response", "negotiated_protocol": result.Result.Protocol})
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}))
	f.HTTPURL = source.URL
	f.RedirectURL = source.URL + "/redirect"
	t.Cleanup(func() {
		cancel()
		in.Close()
		_ = cmd.Wait()
		source.Close()
		destination.Close()
		log.Close()
	})
}

func TestAgentpluginsNativeFixtureHTTP(t *testing.T) {
	f := newNativeFixture(t, false)
	nativeHTTP(t, f)
	body := strings.NewReader(`{"jsonrpc":"2.0","id":"http","method":"initialize","params":{"protocolVersion":"2025-03-26"}}`)
	req, err := http.NewRequest("POST", f.HTTPURL+"/mcp", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-UAP-Probe", f.Nonce)
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil || res.StatusCode != 200 || !bytes.Contains(b, []byte(`"id":"http"`)) {
		t.Fatalf("HTTP fixture: %s %v", b, err)
	}
}

func TestAgentpluginsNativeFixtureEvidenceOutcomes(t *testing.T) {
	for _, tc := range []struct{ name, requestHeader, negotiated, want string }{
		{"missing", "", "2025-06-18", "not_evaluated"},
		{"matching", "2025-06-18", "2025-06-18", "passed"},
		{"authored-sentinel", "harmless-fixture-priority", "2025-06-18", "failed"},
		{"unnegotiated", "2025-06-18", "", "not_evaluated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &nativeFixture{Root: t.TempDir(), Nonce: "nonce"}
			var b bytes.Buffer
			enc := json.NewEncoder(&b)
			_ = enc.Encode(map[string]string{"origin": "declared-source", "method": "POST", "uri": "/mcp?literal=%24%7BUNKNOWN%7D", "rpc_method": "initialize", "probe_header": "nonce", "literal_header": "${UNKNOWN}"})
			_ = enc.Encode(map[string]string{"origin": "protocol-response", "negotiated_protocol": tc.negotiated})
			_ = enc.Encode(map[string]string{"origin": "declared-source", "method": "POST", "uri": "/mcp?literal=%24%7BUNKNOWN%7D", "rpc_method": "tools/list", "protocol": tc.requestHeader})
			nativeWrite(t, filepath.Join(f.Root, "http-events.jsonl"), b.Bytes(), 0600)
			stages := map[string]nativeStage{}
			nativeCheckHTTP(t, f, stages)
			if got := stages["generated_protocol_header_priority"].Status; got != tc.want {
				t.Fatalf("header evidence=%s, want %s", got, tc.want)
			}
		})
	}
	stages := map[string]nativeStage{"unattempted": {Status: "not_evaluated"}}
	nativeAttempt(stages, "install", "add-installer.json")
	if stages["install"].Status != "failed" || stages["unattempted"].Status != "not_evaluated" {
		t.Fatal("attempt/unattempted evidence conflated")
	}
	if nativeRepeatRemoveExpected([]byte("network timeout"), fmt.Errorf("exit 1")) {
		t.Fatal("arbitrary repeat-remove error accepted")
	}
	if !nativeRepeatRemoveExpected([]byte(`agentplugins: installation "native-proof" was not found`), fmt.Errorf("exit 1")) {
		t.Fatal("expected absent installation rejected")
	}
}

func TestAgentpluginsNativeFixtureStageFailures(t *testing.T) {
	stages := map[string]nativeStage{
		"tool":     {Status: "passed"},
		"oauth":    {Status: "not_evaluated"},
		"data":     {Status: "not_applicable"},
		"optional": {Status: "skipped"},
	}
	if err := nativeStageFailures(stages); err != nil {
		t.Fatalf("bounded successful evidence rejected: %v", err)
	}
	nativeAttempt(stages, "install", "add-installer.json")
	if err := nativeStageFailures(stages); err == nil || !strings.Contains(err.Error(), "install:") {
		t.Fatalf("unfinished attempted stage accepted: %v", err)
	}
	stages["install"] = nativeStage{Status: "passed"}
	for _, name := range []string{"A_http", "A_cwd_default", "native_cache_repair", "generated_protocol_header_priority", "redirect_headers"} {
		stages[name] = nativeStage{Status: "failed", Reason: "fixture mismatch"}
		if err := nativeStageFailures(stages); err == nil || !strings.Contains(err.Error(), name+": fixture mismatch") {
			t.Fatalf("%s failure accepted or omitted: %v", name, err)
		}
		delete(stages, name)
	}
	stages["z"] = nativeStage{Status: "failed", Reason: "last"}
	stages["a"] = nativeStage{Status: "failed", Reason: "first"}
	if err := nativeStageFailures(stages); err == nil || err.Error() != "native stages failed: a: first; z: last" {
		t.Fatalf("failure reporting is incomplete or nondeterministic: %v", err)
	}
}

// ReleaseScanner.resolve supports a prepopulated cache. The runtime still runs
// the normal scan-agent-plugin command and validates its pinned report/policy.
func nativeProvisionScanner(t *testing.T, f *nativeFixture) map[string]string {
	t.Helper()
	binary := nativeBinary(t, "AGENTPLUGINS_LINTAI_BIN")
	digest := nativeSHA(t, binary)
	if expected := os.Getenv("AGENTPLUGINS_LINTAI_SHA256"); len(expected) != 64 || digest != expected {
		t.Fatal("provisioned lintai binary digest mismatch")
	}
	// Platform/digest pins mirror agentplugins/adapters/securityscan/release.go's
	// releaseAssets table exactly, so this cache prepopulation stays consistent
	// with what the real ReleaseScanner would resolve to on the same host.
	platform := runtime.GOOS + "-" + runtime.GOARCH
	if platform == "linux-amd64" {
		for _, path := range []string{"/lib/ld-musl-x86_64.so.1", "/lib64/ld-musl-x86_64.so.1"} {
			if _, err := os.Stat(path); err == nil {
				platform += "-musl"
				break
			}
		}
	}
	pins := map[string]string{
		"darwin-arm64":     "be8b263e2323074080d928ea7c2129458299a6d03f7a9f178dfc1aa8e6bc17ff",
		"darwin-amd64":     "abc170612a847bf1a896ef85a4ee93977baa8275f303dac3ed27bf34050b7513",
		"linux-arm64":      "132a37610575bd251ecaf0be4c6090dad144dd1397c99aad989a3944c63c3d4a",
		"linux-amd64":      "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da",
		"linux-amd64-musl": "3da60f749c61e2caca029a44a9ce422d570aef8c57f82ce51c411c8cec12f61b",
		"windows-amd64":    "2f61f6a83a160afa3feed9ea1722b82d0d938ebff865a4e20d39b5f55270c911",
		"windows-arm64":    "484c30e7ef55310e0aec595c06870dbb5454ae7d29404ca797e00acad6011453",
	}
	archive, ok := pins[platform]
	if !ok {
		t.Fatalf("lintai security scanner has no pinned platform asset for %s", platform)
	}

	if os.Getenv("AGENTPLUGINS_LINTAI_ARCHIVE_SHA256") != archive {
		t.Fatal("lintai archive provenance does not match pinned platform asset")
	}
	destination := filepath.Join(f.Root, "installer-state", "security", "lintai", "0.1.3", platform, nativeExecutableName("lintai"))
	b, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, destination, b, 0700)
	if nativeSHA(t, destination) != digest {
		t.Fatal("cached lintai digest mismatch")
	}
	return map[string]string{"version": "0.1.3", "source_tag": "v0.1.3", "binary_sha256": digest, "archive_sha256": archive, "platform": platform, "route": "prepopulated supported scanner cache; ordinary CLI security assessment"}
}

func TestAgentpluginsNativeFixtureAdmission(t *testing.T) {
	f := newNativeFixture(t, false)
	f.HTTPURL = "http://127.0.0.1:12345"
	f.RedirectURL = f.HTTPURL + "/redirect"
	f.writePackage(t, "A", "1.0.0")
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	load := func() domain.PackageEnvelope {
		envelope, err := (loader.Loader{Registry: registry}).Load(context.Background(), domain.LoadInput{SnapshotRoot: f.Package, ExecutableFiles: []string{"bin/" + nativeExecutableName("probe")}})
		if err != nil {
			t.Fatal(err)
		}
		return envelope
	}
	envelope := load()
	if !envelope.MCP.Enabled || len(envelope.MCP.Servers) != 4 || len(envelope.Skills) != 1 {
		t.Fatalf("fixture components disabled or missing: %+v; diagnostics:%+v", envelope.Inventory, envelope.Diagnostics)
	}
	for name, kind := range map[string]string{"default": "stdio", "explicit": "stdio", "http": "streamable-http", "redirect": "streamable-http"} {
		if envelope.MCP.Servers[name].Type != kind {
			t.Fatalf("wrong %s fixture type", name)
		}
	}
	for _, d := range envelope.Diagnostics {
		if d.Severity == domain.SeverityError {
			t.Fatalf("fixture admission: %+v", d)
		}
	}
	plan, err := (planner.Planner{ManagedRoot: filepath.Join(f.Root, "managed")}).Plan(context.Background(), envelope, domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: f.CodexHome}, domain.ScopeUser, "native-proof-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{}
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentMCPServer && component.Support == domain.SupportProjected {
			selected[component.Name] = true
		}
	}
	if len(selected) != 4 {
		t.Fatalf("fixture MCP plan not selected: %+v", plan)
	}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var planObject any
	if err := json.Unmarshal(planJSON, &planObject); err != nil {
		t.Fatal(err)
	}
	if err := nativeValidateMCPSelections(planObject); err != nil {
		t.Fatal(err)
	}
	// Nil loader error alone is insufficient: malformed optional MCP is skipped.
	path := filepath.Join(f.Package, "mcp.json")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	delete(doc, "$schema")
	nativeJSON(t, path, doc)
	invalid := load()
	if invalid.MCP.Enabled || len(invalid.MCP.Servers) != 0 {
		t.Fatal("missing MCP schema did not disable MCP")
	}
}
func TestAgentpluginsNativeFixtureInstallerJSON(t *testing.T) {
	good := []byte(`{"schema_version":1,"command":"add","result":"success","data":{"security":{"evidence_source":"local_scan"}}}`)
	if err := nativeValidateInstallerJSON(good); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{append([]byte("Resolving...\n"), good...), []byte(`{"schema_version":1,"command":"add","result":"failure","data":{}}`), append(append([]byte{}, good...), good...)} {
		if nativeValidateInstallerJSON(bad) == nil {
			t.Fatalf("invalid/failing CLI output accepted: %s", bad)
		}
	}
}

func TestAgentpluginsNativeFixtureCodexSkillIdentity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	path := filepath.Join(root, "skills", "native-proof", "SKILL.md")
	id := "native-proof@agentplugins-test"
	if !nativeSkillIdentity("native-proof:native-proof", id, path, true, id, root) {
		t.Fatal("pinned namespaced identity rejected")
	}
	for _, tc := range []struct {
		name, id, path string
		enabled        bool
	}{{"native-proof", id, path, true}, {"native-proof:native-proof", id + "-foreign", path, true}, {"native-proof:native-proof", id, path, false}, {"native-proof:native-proof", id, filepath.Join(root, "foreign", "SKILL.md"), true}} {
		if nativeSkillIdentity(tc.name, tc.id, tc.path, tc.enabled, id, root) {
			t.Fatalf("inexact native identity accepted: %+v", tc)
		}
	}
	if nativeHasPluginID(t, []byte(`{"data":[{"pluginId":"native-proof@agentplugins-test-foreign"}]}`), id) {
		t.Fatal("substring plugin identity accepted")
	}
}

func nativeTreeDigest(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(h, "%s\x00%o\x00", filepath.ToSlash(relative), info.Mode())
		if info.Mode().IsRegular() {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(body)
			h.Write(digest[:])
		} else if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			h.Write([]byte(target))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
func TestAgentpluginsNativeFixtureLifecycleResults(t *testing.T) {
	good := []byte(`{"schema_version":1,"command":"add","result":"success","data":{"result":{"mutated":false}}}`)
	if err := nativeUnchangedResult(good); err != nil {
		t.Fatal(err)
	}
	for _, b := range [][]byte{[]byte(`{"schema_version":1,"command":"add","result":"success","data":{"result":{}}}`), []byte(`{"schema_version":1,"command":"add","result":"success","data":{"result":{"mutated":true}}}`)} {
		if nativeUnchangedResult(b) == nil {
			t.Fatal("unknown/mutated add accepted as unchanged")
		}
	}
	guard := []byte(`{"result":"failure","data":{"status":"preflight_failed","targets":[{"output":{"result":{"mutated":false}}}]}}`)
	stderr := []byte("native identity ownership is indeterminate; refusing repair")
	if err := nativeRepairGuardResult(guard, stderr, fmt.Errorf("exit 1")); err != nil {
		t.Fatal(err)
	}
	if nativeRepairGuardResult(guard, []byte("network failure"), fmt.Errorf("exit 1")) == nil {
		t.Fatal("arbitrary refusal accepted as ownership guard")
	}
	root := t.TempDir()
	path := filepath.Join(root, "file")
	nativeWrite(t, path, []byte("original"), 0600)
	before := nativeTreeDigest(t, root)
	nativeWrite(t, path, []byte("damage"), 0600)
	if nativeTreeDigest(t, root) == before {
		t.Fatal("changed artifact digest unchanged")
	}
	nativeWrite(t, path, []byte("original"), 0600)
	if nativeTreeDigest(t, root) != before {
		t.Fatal("exact restore not reproducible")
	}
}

// Identity metadata describes the build input, never a made-up patch for a clean commit.
// The orchestrator verifies these values against the build checkout and hashes the binary.
func nativeSourceIdentity(commit, tree, patches string) (map[string]any, error) {
	valid := func(value string, n int) bool {
		b, err := hex.DecodeString(value)
		return err == nil && len(b) == n && value == strings.ToLower(value)
	}
	if !valid(commit, 20) || !valid(tree, 20) {
		return nil, fmt.Errorf("exact 40-character installer commit and tree required")
	}
	state := "committed"
	hashes := []string{}
	if patches != "" {
		state = "reviewed_uncommitted_patches"
		for _, h := range strings.Split(patches, ",") {
			if !valid(h, 32) {
				return nil, fmt.Errorf("installer patch must be an exact SHA256 digest")
			}
			hashes = append(hashes, h)
		}
	}
	return map[string]any{"installer_base_commit": commit, "installer_tree": tree, "installer_source_state": state, "installer_patch_sha256": hashes}, nil
}
func TestAgentpluginsNativeFixtureSourceIdentity(t *testing.T) {
	commit, tree := strings.Repeat("a", 40), strings.Repeat("b", 40)
	identity, err := nativeSourceIdentity(commit, tree, "")
	if err != nil || identity["installer_source_state"] != "committed" {
		t.Fatalf("clean source: %v %v", identity, err)
	}
	identity, err = nativeSourceIdentity(commit, tree, strings.Repeat("c", 64))
	if err != nil || identity["installer_source_state"] != "reviewed_uncommitted_patches" {
		t.Fatal("patch source identity lost")
	}
	for _, bad := range []string{"unknown", strings.Repeat("c", 63), strings.Repeat("c", 64) + ","} {
		if _, err := nativeSourceIdentity(commit, tree, bad); err == nil {
			t.Fatalf("invalid patch accepted: %q", bad)
		}
	}
	if _, err := nativeSourceIdentity("HEAD", tree, ""); err == nil {
		t.Fatal("symbolic commit accepted")
	}
}

// Uses the production exact-SHA acquirer with its supported transport injection.
// The CLI only admits GitHub identities, so this is acquisition evidence, not
// a claim that the CLI accepted a local Git URL or native Git registration.
func nativeImmutableAcquisition(t *testing.T, f *nativeFixture) map[string]any {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, git, args...)
		cmd.Dir = f.Package
		cmd.Env = append(f.env(filepath.Dir(git)), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("synthetic git: %v %s", err, b)
		}
		return strings.TrimSpace(string(b))
	}
	run("init", "--quiet")
	run("add", ".")
	run("commit", "--quiet", "-m", "test: immutable fixture")
	revision := run("rev-parse", "HEAD")
	original := nativeSHA(t, filepath.Join(f.Package, "plugin.json"))
	nativeWrite(t, filepath.Join(f.Package, "uncommitted-sentinel"), []byte("must not enter snapshot"), 0600)
	acquirer := sourceacquisition.Acquirer{TempRoot: filepath.Join(f.Root, "tmp"), URLForRepo: func(string) string { return f.Package }}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	snapshot, err := acquirer.AcquireGitHub(ctx, "fixture/native-proof", revision, "")
	if err != nil {
		t.Fatal(err)
	}
	defer packagedigest.Remove(snapshot)
	if snapshot.Source.ResolvedRevision != revision || nativeSHA(t, filepath.Join(snapshot.Root, "plugin.json")) != original {
		t.Fatal("immutable acquisition identity mismatch")
	}
	if _, err := os.Lstat(filepath.Join(snapshot.Root, "uncommitted-sentinel")); !os.IsNotExist(err) {
		t.Fatal("uncommitted bytes entered immutable acquisition")
	}
	if _, err := os.Lstat(filepath.Join(snapshot.Root, ".git")); !os.IsNotExist(err) {
		t.Fatal("Git metadata entered package")
	}
	return map[string]any{"revision": revision, "tree": run("rev-parse", "HEAD^{tree}"), "manifest_sha256": original, "source": snapshot.Source, "evidence_scope": "production acquirer with synthetic local Git transport; acquisition only"}
}
func TestAgentpluginsNativeFixtureImmutableAcquisition(t *testing.T) {
	f := newNativeFixture(t, false)
	f.writePackage(t, "A", "1.0.0")
	nativeImmutableAcquisition(t, f)
}

// Minimal Responses events match pinned codex-api/tests/sse_end_to_end.rs.
func nativeScriptedProvider(t *testing.T, f *nativeFixture) string {
	t.Helper()
	f.ProviderRequests = make(chan []byte, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/responses" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, (2<<20)+1))
		if err != nil || len(body) > 2<<20 {
			http.Error(w, "bounded fixture request", 400)
			return
		}
		select {
		case f.ProviderRequests <- body:
		default:
			http.Error(w, "fixture request limit", 429)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		events := []map[string]any{
			{"type": "response.output_item.done", "item": map[string]any{"type": "message", "id": "fixture-message", "role": "assistant", "content": []any{map[string]string{"type": "output_text", "text": "Fixture response."}}}},
			{"type": "response.completed", "response": map[string]any{"id": "fixture-response", "usage": map[string]int{"input_tokens": 1, "output_tokens": 1, "total_tokens": 2}}},
		}
		for _, event := range events {
			b, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event["type"], b)
		}
	}))
	t.Cleanup(server.Close)
	return server.URL + "/v1"
}

func TestAgentpluginsNativeFixtureForeignCollisionClassification(t *testing.T) {
	good := []byte(`{"result":"failure","data":{"status":"preflight_failed","targets":[{"output":{"result":{"mutated":false}}}]}}`)
	stderr := []byte("Resolving and validating each unique exact installed package revision once...\nagentplugins: group repair preflight failed; no target was changed: observe prepared identity for codex: native package has no recognized authoritative manifest\n")
	if err := nativeCollisionGuardResult(good, stderr, fmt.Errorf("exit 1")); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"network timeout", "agentplugins: unrelated preflight failure"} {
		if nativeCollisionGuardResult(good, []byte(bad), fmt.Errorf("exit 1")) == nil {
			t.Fatal("unrelated failure accepted")
		}
	}
	mutated := bytes.ReplaceAll(good, []byte("false"), []byte("true"))
	if nativeCollisionGuardResult(mutated, stderr, fmt.Errorf("exit 1")) == nil {
		t.Fatal("mutating collision accepted")
	}
	if nativeCollisionGuardResult(good, stderr, nil) == nil {
		t.Fatal("successful collision accepted")
	}
}

func TestAgentpluginsNativeFixtureStartupFailure(t *testing.T) {
	f := newNativeFixture(t, false)
	cmd := exec.Command(f.Probe, "--fail-startup")
	cmd.Dir = f.Project
	cmd.Env = append(f.env(filepath.Dir(f.Probe)), "UAP_TEST_ROOT="+f.Root, "UAP_TEST_EVENTS="+f.Events, "UAP_TEST_NONCE="+f.Nonce)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 42 || len(output) != 0 || !strings.Contains(stderr.String(), "intentional fixture startup failure") {
		t.Fatalf("startup failure contract: %v %s %s", err, output, stderr.String())
	}
	events, err := os.ReadFile(f.Events)
	if err != nil || !bytes.Contains(events, []byte(`"method":"startup-failed"`)) {
		t.Fatalf("startup failure not recorded: %v %s", err, events)
	}
}
