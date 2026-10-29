//go:build linux

package pluginkitairepo_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
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

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Root acquisition supplies a bounded JSON object through
// AGENTPLUGINS_PROJECT_NATIVE_IDENTITY. Its minimum typed shape is:
// {"client":"gemini","acquisition":{"sha256":"<archive>",
// "pin_verified":true,"members":[{"name":"gemini.js","kind":"file",
// "bytes":5092,"sha256":"<entrypoint>"},<all regular support members>]},
// "runtime_files":[{"path":"<absolute/gemini.js>","sha256":"<entrypoint>"},
// <all support files and the selected Node executable>]}.
// Gemini additionally supplies node with the same acquisition/member shape and
// includes its exact Node file in runtime_files. Acquisition may retain its
// existing provenance fields; every regular member must have one runtime file.
type geminiProjectAttemptIdentity struct {
	Client       string                       `json:"client"`
	Acquisition  geminiProjectAttemptArchive  `json:"acquisition"`
	Node         *geminiProjectAttemptArchive `json:"node,omitempty"`
	RuntimeFiles []struct {
		Path string `json:"path"`
		SHA  string `json:"sha256"`
	} `json:"runtime_files"`
}
type geminiProjectAttemptArchive struct {
	SHA     string `json:"sha256"`
	Pin     bool   `json:"pin_verified"`
	Members []struct {
		Name  string `json:"name"`
		Kind  string `json:"kind"`
		Bytes int64  `json:"bytes"`
		SHA   string `json:"sha256"`
	} `json:"members"`
}
type geminiProjectAttempt struct {
	base                                           *nativeFixture
	client, installer, git, other, outside, output string
	node, detectBin, secret                        string
	ctx                                            context.Context
	cancel                                         context.CancelFunc
	started                                        time.Time
	files                                          map[string][]byte
	immutable                                      map[string]string
	protected                                      map[string]string
	evidence                                       map[string]any
	binding                                        domain.ClientBinding
	data                                           string
	facts                                          nativeFacts
}

func (f *geminiProjectAttempt) keep(t *testing.T, name string, body []byte) {
	t.Helper()
	total := 0
	for _, b := range f.files {
		total += len(b)
	}
	if filepath.Base(name) != name || f.files[name] != nil || len(f.files) >= 80 || len(body) > 2<<20 || total+len(body) > 16<<20 {
		t.Fatal("closed attempt artifact bound")
	}
	f.files[name] = bytes.ReplaceAll(body, []byte(f.secret), []byte("[redacted]"))
}
func (f *geminiProjectAttempt) check(t *testing.T) {
	t.Helper()
	if f.detectBin != "" {
		link := filepath.Join(f.detectBin, "gemini")
		if err := projectNativeRuntimeParents(link); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(link)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatal("Gemini recorded detection link changed")
		}
		target, err := os.Readlink(link)
		if err != nil || target != f.client {
			t.Fatal("Gemini recorded detection target changed")
		}
	}
	for path, want := range f.immutable {
		got, err := projectNativeRuntimeDigest(path)
		if err != nil || got != want {
			t.Fatalf("immutable identity changed: %s (%v)", path, err)
		}
	}
	for path, want := range f.protected {
		got, err := projectNativeRuntimeDigest(path)
		if err != nil || got != want {
			t.Fatalf("original authority changed: %s (%v)", path, err)
		}
	}
}
func (f *geminiProjectAttempt) command(t *testing.T, label, cwd string, env, argv []string, timeout time.Duration) ports.CommandResult {
	t.Helper()
	f.check(t)
	ctx, cancel := context.WithTimeout(f.ctx, timeout)
	defer cancel()
	began := time.Now()
	result, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: argv, Env: env, Dir: cwd, StdoutLimitBytes: projectNativeRuntimeBodyLimit}, 5*time.Second)
	f.keep(t, label+".stdout.log", result.Stdout)
	f.keep(t, label+".stderr.log", result.Stderr)
	record := map[string]any{"argv": argv, "cwd": cwd, "exit": result.ExitCode, "elapsed_ms": time.Since(began).Milliseconds(), "error": err != nil, "context_error": ctx.Err() != nil, "cleanup": "ordinary full-descendant supervision; forced cleanup is a failure"}
	body, _ := json.Marshal(record)
	f.keep(t, label+".command.json", body)
	f.check(t)
	if err != nil || ctx.Err() != nil || result.ExitCode != 0 || len(result.Stdout) > projectNativeRuntimeBodyLimit || bytes.Contains(result.Stderr, []byte("stderr bytes omitted")) {
		t.Fatalf("%s process/capture/cleanup failed; see bounded evidence", label)
	}
	return result
}
func (f *geminiProjectAttempt) env() []string {
	return append(nativePlatformEnvironment(f.base.Root, f.base.Home, filepath.Dir(f.node), f.detectBin, filepath.Dir(f.git)), "AGENTPLUGINS_HOME="+filepath.Join(f.base.Root, "installer-state"), "NO_COLOR=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
}
func newGeminiProjectAttempt(t *testing.T) *geminiProjectAttempt {
	t.Helper()
	started := time.Now()
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED") != "1" {
		t.Fatal("runtime attempt requires actual opted-in GitHub-hosted Linux amd64")
	}
	f := &geminiProjectAttempt{started: started, files: map[string][]byte{}, immutable: map[string]string{}, protected: map[string]string{}, evidence: map[string]any{"client": "gemini", "provider_kind": "scripted_loopback", "runtime_attempt_complete": false, "activation": "prepared", "vendor_authentication_qualified": false, "real_model_qualified": false, "qualification": map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}}}
	f.ctx, f.cancel = context.WithDeadline(context.Background(), started.Add(150*time.Second))
	f.output = os.Getenv("AGENTPLUGINS_PROJECT_NATIVE_OUTPUT")
	if !filepath.IsAbs(f.output) {
		t.Fatal("explicit attempt output directory required")
	}
	if err := projectNativeRuntimeParents(filepath.Join(f.output, "leaf")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(f.output)
	if err != nil || len(entries) != 0 {
		t.Fatal("attempt output must be an existing empty ordinary directory")
	}
	f.client = nativeBinary(t, "AGENTPLUGINS_PROJECT_NATIVE_GEMINI_EXECUTABLE")
	f.node = nativeBinary(t, "AGENTPLUGINS_NODE_BIN")
	var secret [24]byte
	if _, err := rand.Read(secret[:]); err != nil {
		t.Fatal(err)
	}
	f.secret = "uap-fixture-" + hex.EncodeToString(secret[:])
	f.installer = nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	f.git, err = exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	body, err := projectNativeRuntimeRead(os.Getenv("AGENTPLUGINS_PROJECT_NATIVE_IDENTITY"), projectNativeRuntimeBodyLimit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectNativeRuntimeJSON(body); err != nil {
		t.Fatal(err)
	}
	var identity geminiProjectAttemptIdentity
	if err := json.Unmarshal(body, &identity); err != nil || identity.Client != "gemini" || !identity.Acquisition.Pin || identity.Acquisition.SHA != "bb5e7e879608bd9b3fe261930d3886fa56aacbf471da1655f2978eb35b705a73" || identity.Node == nil || !identity.Node.Pin || identity.Node.SHA != "fd8e59d5a511510f6a298afb548f18c7d2b1be404d8b4a27d94fbe49f56cb2d6" {
		t.Fatal("Gemini immutable acquisition identity differs")
	}
	expected := map[string]string{}
	for _, member := range identity.Acquisition.Members {
		if member.Kind == "directory" {
			continue
		}
		if member.Kind != "file" || member.Bytes < 0 || len(member.SHA) != 64 || member.Name == "" || filepath.IsAbs(member.Name) || filepath.Clean(member.Name) != member.Name || strings.Contains(member.Name, "\\") || strings.HasPrefix(member.Name, "../") {
			t.Fatal("Gemini regular support member differs")
		}
		path := filepath.Join(filepath.Dir(f.client), member.Name)
		if expected[path] != "" {
			t.Fatal("duplicate Gemini member")
		}
		expected[path] = member.SHA
	}
	if expected[f.client] != "41847af2158ced805c4a3e098a05b9a2c26fb1a49b12583009b2eaf6a4059a59" || len(identity.Node.Members) != 1 || identity.Node.Members[0].Name != "node-v24.21.0-linux-x64/bin/node" || identity.Node.Members[0].Kind != "file" || identity.Node.Members[0].Bytes != 126595440 || identity.Node.Members[0].SHA != "7fde7b8afa198da66257f42ee2001d874c7355631e6d1579a5fb5ef1f246df4c" {
		t.Fatal("exact Gemini bundle/Node identity differs")
	}
	expected[f.node] = identity.Node.Members[0].SHA
	if len(identity.RuntimeFiles) != len(expected) {
		t.Fatal("Gemini complete runtime closure missing")
	}
	for _, file := range identity.RuntimeFiles {
		if expected[file.Path] == "" || expected[file.Path] != file.SHA || f.immutable[file.Path] != "" {
			t.Fatal("Gemini runtime closure identity differs")
		}
		f.immutable[file.Path] = file.SHA
	}
	actual := 0
	err = filepath.WalkDir(filepath.Dir(f.client), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if expected[path] == "" {
			return fmt.Errorf("unrecorded Gemini support member")
		}
		actual++
		return nil
	})
	if err != nil || actual != len(expected)-1 {
		t.Fatal("Gemini runtime member set differs")
	}
	for _, path := range []string{f.installer, f.git, nativeBinary(t, "AGENTPLUGINS_NATIVE_PROBE_BIN"), nativeBinary(t, "AGENTPLUGINS_LINTAI_BIN")} {
		digest, err := projectNativeRuntimeDigest(path)
		if err != nil {
			t.Fatal(err)
		}
		f.immutable[path] = digest
	}
	if source, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256")); err != nil {
		t.Fatal(err)
	} else {
		f.evidence["source_identity"] = source
	}
	f.check(t)
	f.base = newNativeFixture(t, false)
	f.other = filepath.Join(f.base.Root, "project-B")
	f.outside = filepath.Join(f.base.Root, "outside")
	for _, dir := range []string{f.other, f.outside} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	f.immutable[f.base.Probe] = nativeSHA(t, f.base.Probe)
	f.detectBin = filepath.Join(f.base.Root, "detection-bin")
	if err := os.Mkdir(f.detectBin, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(f.client, filepath.Join(f.detectBin, "gemini")); err != nil {
		t.Fatal(err)
	}
	f.evidence["gemini_detection_link"] = map[string]string{"path": filepath.Join(f.detectBin, "gemini"), "target": f.client}
	f.evidence["acquisition"] = identity
	f.evidence["client_source"] = map[string]string{"commit": "6006c14a4decfe4b891c6263ad6ad5992e404b5b", "tools_blob": "ab2870984aba77e77fc5bacb25009cd47698582e", "output_types_blob": "1d4159a1f8e545ec2da2b8df413cb6778b3f79dc", "mcp_tool_blob": "997f1957eed193f61bf6090b607d3ad402da7742", "genai_commit": "38cac5bbf4941ec5fa760238bd423c0ecc2c6f04", "genai_converters_blob": "509660270e9cfe2173850ba0a8e9e4ddbeb3b3af", "genai_types_blob": "24ed38f1aaace9165a0dfb4bcbfe794ca94a75de"}
	f.evidence["scanner"] = nativeProvisionScanner(t, f.base)

	return f
}
func (f *geminiProjectAttempt) setup(t *testing.T) {
	t.Helper()
	for _, project := range []struct{ name, path string }{{"A", f.base.Project}, {"B", f.other}} {
		r := f.command(t, "git-"+project.name, project.path, f.env(), []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}, 30*time.Second)
		if len(r.Stdout) != 0 || len(r.Stderr) != 0 {
			t.Fatal("Git project setup emitted diagnostics")
		}
	}
	settings := filepath.Join(f.base.Home, ".gemini", "settings.json")
	settingsBody, _ := json.Marshal(map[string]any{"security": map[string]any{"auth": map[string]any{"selectedType": "gemini-api-key"}, "folderTrust": map[string]any{"enabled": true}}, "privacy": map[string]any{"usageStatisticsEnabled": false}, "telemetry": map[string]any{"enabled": false}, "experimental": map[string]any{"adk": map[string]any{"agentSessionNoninteractiveEnabled": false}}, "general": map[string]any{"enableAutoUpdate": false, "enableAutoUpdateNotification": false}})
	trust := filepath.Join(f.base.Home, ".gemini", "trustedFolders.json")
	trustBody, _ := json.Marshal(map[string]string{f.base.Project: "TRUST_FOLDER", f.other: "TRUST_FOLDER"})
	policy := filepath.Join(f.base.Home, ".gemini", "policies", "uap-project-proof.toml")
	policyBody := []byte("[[rule]]\ntoolName = \"*\"\ndecision = \"deny\"\npriority = 100\n\n[[rule]]\nmcpName = \"uapgeminia\"\ntoolName = \"inspect_runtime\"\ndecision = \"allow\"\npriority = 200\n\n[[rule]]\nmcpName = \"uapgeminib\"\ntoolName = \"inspect_runtime\"\ndecision = \"allow\"\npriority = 200\n")
	for path, body := range map[string][]byte{settings: settingsBody, trust: trustBody, policy: policyBody} {
		projectNativeRuntimeWrite(t, path, append(body, '\n'), 0600)
		f.protected[path] = nativeSHA(t, path)
	}
	r := f.command(t, "version", f.outside, f.env(), []string{f.node, f.client, "--version"}, 30*time.Second)
	if string(r.Stdout) != "0.65.0\n" || len(r.Stderr) != 0 {
		t.Fatal("exact pinned Gemini version differs")
	}
	f.base.writePackage(t, "A-one", "1.0.0")
	// Keep the shared harmless probe, but use one distinct installed server and
	// one distinct skill. These are public package source bytes, never a fixture
	// replacement for the installed native entry.
	if err := os.RemoveAll(filepath.Join(f.base.Package, "skills")); err != nil {
		t.Fatal(err)
	}
	projectNativeRuntimeWrite(t, filepath.Join(f.base.Package, "skills", "uapgeminia", "SKILL.md"), []byte("---\nname: uapgeminia\ndescription: Disposable Gemini project proof\n---\nUAP_GEMINI_A_ONE_"+f.base.Nonce+"\n"), 0600)
	server := map[string]any{"type": "stdio", "command": "./bin/probe", "args": []string{"argument with spaces", "${PLUGIN_ROOT}/argument", "${UNKNOWN}"}, "cwd": "${PLUGIN_ROOT}", "env": map[string]string{"UAP_TEST_ROOT": f.base.Root, "UAP_TEST_EVENTS": f.base.Events, "UAP_TEST_NONCE": f.base.Nonce, "UAP_TEST_REVISION": "A-one", "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": "${PLUGIN_ROOT}/${UNKNOWN}"}}
	nativeJSON(t, filepath.Join(f.base.Package, "mcp.json"), map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": map[string]any{"uapgeminia": server}})
	f.immutable[filepath.Join(f.base.Package, "bin", "probe")] = nativeSHA(t, filepath.Join(f.base.Package, "bin", "probe"))
	r = f.command(t, "add-A", f.base.Project, f.env(), []string{f.installer, "add", f.base.Package, "--no-save", "--scope", "project", "--project-root", f.base.Project, "--target", "gemini", "--format", "json"}, 30*time.Second)
	if string(r.Stderr) != "Resolving and validating one Agent Plugin package for every selected target...\nInstalling native-proof for 1 selected clients...\n" {
		t.Fatal("unexpected public installer diagnostic")
	}
	if _, err := projectNativeRuntimeJSON(r.Stdout); err != nil {
		t.Fatal(err)
	}
	if err := nativeValidateInstallerJSON(r.Stdout); err != nil {
		t.Fatal(err)
	}
	stateBody, err := projectNativeRuntimeRead(filepath.Join(f.base.Project, ".uap", "state-v2.json"), projectNativeRuntimeBodyLimit)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(stateBody, &state); err != nil || len(state.Installations) != 1 || state.Installations[0].DeclaredName != "native-proof" || len(state.Installations[0].Clients) != 1 {
		t.Fatal("exact installed project receipt differs")
	}
	installation := state.Installations[0]
	if installation.Source.CanonicalSource != f.base.Package || installation.Source.SourceBindingID == "" || installation.Source.TreeDigest == "" || installation.Package.DeclaredName != "native-proof" || installation.Package.Version != "1.0.0" || installation.Package.ManifestDigest == "" {
		t.Fatal("selected local package identity differs")
	}
	f.evidence["installation_source"] = installation.Source
	for _, binding := range installation.Clients {
		data := installation.DataReceipts[binding.DataReceiptID]
		if binding.ClientID != "gemini" || binding.Scope != "project" || binding.NativeProfileRoot != filepath.Join(f.base.Project, ".gemini") || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || data.State != domain.DataReceiptOwned || data.Scope != "project" || data.DataReceiptID != binding.DataReceiptID || data.PhysicalBackend != binding.PhysicalArtifact || data.OwnershipDigest == "" {
			t.Fatal("installed Prepared/data authority differs")
		}
		nativeRequireContained(t, filepath.Join(f.base.Project, ".uap", "managed"), binding.TargetLocator)
		nativeRequireContained(t, filepath.Join(f.base.Project, ".uap", "plugin-data"), data.Locator)
		f.binding, f.data = binding, data.Locator
	}
	owned := map[string]string{}
	for _, object := range f.binding.NativeObjects {
		if object.Kind == "gemini_global_mcp_server" || object.Kind == "gemini_global_skill_directory" {
			if object.LogicalName != "uapgeminia" || object.ManagedDigest == "" || owned[object.Kind] != "" {
				t.Fatal("installed native ownership differs")
			}
			owned[object.Kind] = object.Path
		}
	}
	if owned["gemini_global_mcp_server"] != filepath.Join(f.base.Project, ".gemini", "settings.json") || owned["gemini_global_skill_directory"] != filepath.Join(f.base.Project, ".gemini", "skills", "uapgeminia") {
		t.Fatal("installed native ownership paths differ")
	}
	for _, path := range []string{owned["gemini_global_mcp_server"], filepath.Join(owned["gemini_global_skill_directory"], "SKILL.md")} {
		body, err := projectNativeRuntimeRead(path, projectNativeRuntimeBodyLimit)
		if err != nil {
			t.Fatal(err)
		}
		f.keep(t, filepath.Base(filepath.Dir(path))+"-"+filepath.Base(path)+".installed", body)
	}
	f.evidence["binding"] = f.binding
	f.evidence["data_locator"] = f.data
	if _, err := os.Lstat(f.base.Events); !os.IsNotExist(err) {
		t.Fatal("probe started before native session")
	}
	f.check(t)
}
func (f *geminiProjectAttempt) validateFacts(body string) error {
	value, err := projectNativeRuntimeJSON([]byte(body))
	if err != nil {
		return err
	}
	object, ok := value.(map[string]any)
	if !ok || projectNativeRuntimeKeys(object, "nonce revision cwd argv plugin_root plugin_data literal once marker pid session", "") != nil {
		return fmt.Errorf("probe facts fields differ")
	}
	var facts nativeFacts
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&facts); err != nil {
		return err
	}
	if facts.Nonce != f.base.Nonce || facts.Revision != "A-one" || facts.Root != f.binding.TargetLocator || facts.Data != f.data || facts.CWD != f.binding.TargetLocator || !reflect.DeepEqual(facts.Argv, []string{"argument with spaces", f.binding.TargetLocator + "/argument", "${UNKNOWN}"}) || facts.Literal != "literal  "+f.base.Home || facts.Once != f.binding.TargetLocator+"/" || facts.Marker != "retained-"+f.base.Nonce || facts.PID <= 0 || facts.Session == "" {
		return fmt.Errorf("returned probe authority/nonce/revision/cwd/marker differs")
	}
	log, err := projectNativeRuntimeRead(f.base.Events, 2<<20)
	if err != nil {
		return err
	}
	methods := map[string]int{}
	lastTime := time.Time{}
	for _, line := range bytes.Split(bytes.TrimSuffix(log, []byte{'\n'}), []byte{'\n'}) {
		v, err := projectNativeRuntimeJSON(line)
		if err != nil {
			return err
		}
		m, ok := v.(map[string]any)
		if !ok || projectNativeRuntimeKeys(m, "method pid session nonce timestamp facts", "") != nil {
			return fmt.Errorf("probe event shape")
		}
		if m["session"] != facts.Session || m["nonce"] != facts.Nonce || m["pid"] != json.Number(fmt.Sprint(facts.PID)) {
			return fmt.Errorf("probe event session identity differs")
		}
		method, ok := m["method"].(string)
		if !ok || (method != "initialize" && method != "tools/list" && method != "tools/call") {
			return fmt.Errorf("unexpected probe method")
		}
		timestamp, ok := m["timestamp"].(string)
		when, timeErr := time.Parse(time.RFC3339Nano, timestamp)
		if !ok || timeErr != nil || when.Before(lastTime) || (method != "initialize" && methods["initialize"] != 1) || (method == "tools/call" && methods["tools/list"] == 0) || methods["tools/call"] != 0 {
			return fmt.Errorf("probe events out of order")
		}
		lastTime = when
		if method != "tools/call" && m["facts"] != nil {
			return fmt.Errorf("unexpected probe facts")
		}
		methods[method]++
		if method == "tools/call" {
			object, ok := m["facts"].(map[string]any)
			if !ok || projectNativeRuntimeKeys(object, "nonce revision cwd argv plugin_root plugin_data literal once marker pid session", "") != nil {
				return fmt.Errorf("probe event facts fields differ")
			}
			encoded, _ := json.Marshal(m["facts"])
			var observed nativeFacts
			if json.Unmarshal(encoded, &observed) != nil || !reflect.DeepEqual(observed, facts) {
				return fmt.Errorf("probe event/result facts differ")
			}
		}
	}
	if methods["initialize"] != 1 || methods["tools/list"] < 1 || methods["tools/list"] > 4 || methods["tools/call"] != 1 {
		return fmt.Errorf("missing/duplicate native-driven MCP methods")
	}
	f.facts = facts
	return nil
}
func (f *geminiProjectAttempt) finish(t *testing.T) {
	t.Helper()
	defer f.cancel()
	if time.Since(f.started) > 180*time.Second {
		t.Error("attempt total budget exceeded")
	}
	f.evidence["test_failed"] = t.Failed()
	f.evidence["elapsed_ms"] = time.Since(f.started).Milliseconds()
	if f.base != nil {
		if body, err := projectNativeRuntimeRead(f.base.Events, 2<<20); err == nil {
			f.keep(t, "probe-events.jsonl", body)
		}
	}
	body, _ := json.MarshalIndent(f.evidence, "", "  ")
	f.keep(t, "runtime-attempt.json", append(body, '\n'))
	if err := geminiProjectSecretFree(f.files, []string{f.secret}); err != nil {
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
		entry, err := archive.Create(name)
		if err != nil {
			t.Error(err)
			break
		}
		if _, err := entry.Write(f.files[name]); err != nil {
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

func geminiProjectSecretFree(files map[string][]byte, secrets []string) error {
	for _, secret := range secrets {
		if len(secret) < 8 {
			return fmt.Errorf("unregistered Gemini fixture key")
		}
		for name, body := range files {
			if strings.Contains(name, secret) || bytes.Contains(body, []byte(secret)) {
				return fmt.Errorf("private fixture key refused at closed artifact admission")
			}
		}
	}
	return nil
}

func TestAgentpluginsGeminiNativeProjectRuntimeAttempt(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_GEMINI_PROJECT_RUNTIME_ATTEMPT") != "1" {
		t.Skip("opt-in current-source runtime research")
	}
	f := newGeminiProjectAttempt(t)
	defer f.finish(t)
	f.setup(t)
	p := newGeminiProjectProvider(f.base.Nonce, f.secret, f.validateFacts)
	if err := p.start(f.ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		p.server.Close()
		p.mu.Lock()
		defer p.mu.Unlock()
		for i, body := range p.requests {
			f.keep(t, fmt.Sprintf("provider-request-%d.json", i+1), body)
		}
		f.evidence["provider_routes"] = p.routes
		f.evidence["provider_returned"] = p.returned
		if p.err != nil {
			f.evidence["provider_error"] = p.err.Error()
		}
	}()
	prompt := "Call only the installed inspect_runtime tool once for the fixture challenge."
	env := append(f.env(), "GEMINI_API_KEY="+f.secret, "GOOGLE_GEMINI_BASE_URL="+p.server.URL, "GOOGLE_GENAI_API_VERSION=v1beta")
	result := f.command(t, "native-attempt", f.base.Project, env, []string{f.node, f.client, "--prompt", prompt, "--approval-mode", "default", "--output-format", "stream-json", "--model", geminiProjectModel}, 60*time.Second)
	if !geminiProjectNativeStderrAllowed(result.Stderr) {
		t.Fatal("unexpected Gemini native diagnostic")
	}
	p.server.Close()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil || !p.returned || p.streams != 2 {
		t.Fatalf("Gemini native provider exchange incomplete: %v", p.err)
	}
	providerFacts := f.facts
	session, err := geminiProjectStream(result.Stdout, p, prompt, f.validateFacts)
	if err != nil || !reflect.DeepEqual(providerFacts, f.facts) {
		t.Fatalf("Gemini native stream/session/result admission failed: %v", err)
	}
	f.evidence["native_session_id"] = session
	f.evidence["provider_call_id"] = p.id
	f.evidence["probe_facts"] = f.facts
	f.evidence["runtime_attempt_complete"] = true
}
