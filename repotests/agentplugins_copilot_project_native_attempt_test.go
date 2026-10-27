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

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Root acquisition supplies a bounded JSON object through
// AGENTPLUGINS_PROJECT_NATIVE_IDENTITY. Its minimum typed shape is:
// {"client":"copilot","acquisition":{"sha256":"<archive>",
// "pin_verified":true,"members":[{"name":"copilot","kind":"file",
// "bytes":198445888,"sha256":"<executable>"}]},
// "runtime_files":[{"path":"<absolute/copilot>","sha256":"<executable>"}]}.
// Gemini additionally supplies node with the same acquisition/member shape and
// includes its exact Node file in runtime_files. Acquisition may retain its
// existing provenance fields; every regular member must have one runtime file.
type copilotProjectAttemptIdentity struct {
	Client       string                        `json:"client"`
	Acquisition  copilotProjectAttemptArchive  `json:"acquisition"`
	Node         *copilotProjectAttemptArchive `json:"node,omitempty"`
	RuntimeFiles []struct {
		Path string `json:"path"`
		SHA  string `json:"sha256"`
	} `json:"runtime_files"`
}
type copilotProjectAttemptArchive struct {
	SHA     string `json:"sha256"`
	Pin     bool   `json:"pin_verified"`
	Members []struct {
		Name  string `json:"name"`
		Kind  string `json:"kind"`
		Bytes int64  `json:"bytes"`
		SHA   string `json:"sha256"`
	} `json:"members"`
}
type copilotProjectAttempt struct {
	base                                           *nativeFixture
	client, installer, git, other, outside, output string
	ctx                                            context.Context
	cancel                                         context.CancelFunc
	started                                        time.Time
	nativeStarted, nativeEnded                     time.Time
	readinessObserved                              bool
	files                                          map[string][]byte
	immutable                                      map[string]string
	protected                                      map[string]string
	originalConfigPath                             string
	originalConfigBaseline                         []byte
	evidence                                       map[string]any
	binding                                        domain.ClientBinding
	data                                           string
	facts                                          nativeFacts
}

func (f *copilotProjectAttempt) keep(t *testing.T, name string, body []byte) {
	t.Helper()
	total := 0
	for _, b := range f.files {
		total += len(b)
	}
	if filepath.Base(name) != name || f.files[name] != nil || len(f.files) >= 80 || len(body) > 2<<20 || total+len(body) > 16<<20 {
		t.Fatal("closed attempt artifact bound")
	}
	f.files[name] = bytes.Clone(body)
}
func (f *copilotProjectAttempt) check(t *testing.T) {
	t.Helper()
	for path, want := range f.immutable {
		got, err := projectNativeRuntimeDigest(path)
		if err != nil || got != want {
			t.Fatalf("immutable identity changed: %s (%v)", path, err)
		}
	}
	for path, want := range f.protected {
		got, err := f.protectedDigest(path, want)
		if err != nil || got != want {
			t.Fatalf("original authority changed: %s (%v)", path, err)
		}
	}
}
func (f *copilotProjectAttempt) command(t *testing.T, label, cwd string, env, argv []string, timeout time.Duration) ports.CommandResult {
	t.Helper()
	f.check(t)
	ctx, cancel := context.WithTimeout(f.ctx, timeout)
	defer cancel()
	began := time.Now()
	if label == "native-attempt" {
		f.nativeStarted = began.UTC()
		f.evidence["native_command_started_at"] = f.nativeStarted.Format(time.RFC3339Nano)
	}
	result, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: argv, Env: env, Dir: cwd, StdoutLimitBytes: copilotProjectBodyLimit}, 5*time.Second)
	if label == "native-attempt" {
		f.nativeEnded = time.Now().UTC()
		f.evidence["native_command_ended_at"] = f.nativeEnded.Format(time.RFC3339Nano)
	}
	f.keep(t, label+".stdout.log", result.Stdout)
	f.keep(t, label+".stderr.log", result.Stderr)
	record := map[string]any{"argv": argv, "cwd": cwd, "exit": result.ExitCode, "elapsed_ms": time.Since(began).Milliseconds(), "error": err != nil, "context_error": ctx.Err() != nil, "cleanup": "ordinary full-descendant supervision; forced cleanup is a failure"}
	body, _ := json.Marshal(record)
	f.keep(t, label+".command.json", body)
	f.check(t)
	if err != nil || ctx.Err() != nil || result.ExitCode != 0 || len(result.Stdout) > copilotProjectBodyLimit || bytes.Contains(result.Stderr, []byte("stderr bytes omitted")) {
		t.Fatalf("%s process/capture/cleanup failed; see bounded evidence", label)
	}
	return result
}
func (f *copilotProjectAttempt) env() []string {
	return append(f.base.env(filepath.Dir(f.client)), "COPILOT_OFFLINE=true", "COPILOT_AUTO_UPDATE=false", "NO_COLOR=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
}
func newCopilotProjectAttempt(t *testing.T) *copilotProjectAttempt {
	t.Helper()
	started := time.Now()
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED") != "1" {
		t.Fatal("runtime attempt requires actual opted-in GitHub-hosted Linux amd64")
	}
	f := &copilotProjectAttempt{started: started, files: map[string][]byte{}, immutable: map[string]string{}, protected: map[string]string{}, evidence: map[string]any{"client": "copilot", "provider_kind": "scripted_loopback", "runtime_attempt_complete": false, "activation": "prepared", "vendor_authentication_qualified": false, "real_model_qualified": false, "qualification": map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}}}
	f.ctx, f.cancel = context.WithDeadline(context.Background(), started.Add(150*time.Second))
	f.evidence["mode"] = copilotReadinessMode
	f.evidence["provider_closed"] = false
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
	f.client = nativeBinary(t, "AGENTPLUGINS_COPILOT_BIN")
	f.installer = nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	f.git, err = exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	body, err := projectNativeRuntimeRead(os.Getenv("AGENTPLUGINS_PROJECT_NATIVE_IDENTITY"), copilotProjectBodyLimit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectNativeRuntimeJSON(body); err != nil {
		t.Fatal(err)
	}
	var identity copilotProjectAttemptIdentity
	if err := json.Unmarshal(body, &identity); err != nil || identity.Client != "copilot" || !identity.Acquisition.Pin || identity.Acquisition.SHA != "9aeb4430e9ac9523546db07b227c7aba179a4710a55f9ac29ad143b0f9e1b325" || identity.Node != nil || len(identity.RuntimeFiles) != 1 {
		t.Fatal("Copilot immutable acquisition identity differs")
	}
	if len(identity.Acquisition.Members) != 1 || identity.Acquisition.Members[0].Name != "copilot" || identity.Acquisition.Members[0].Kind != "file" || identity.Acquisition.Members[0].Bytes != 198445888 || identity.Acquisition.Members[0].SHA != "f19b3554a6b62605db3a26d5255ebe2a3ee39fcd6a357edb18301ea5037f6f0d" || identity.RuntimeFiles[0].Path != f.client || identity.RuntimeFiles[0].SHA != identity.Acquisition.Members[0].SHA {
		t.Fatal("Copilot exact regular executable identity differs")
	}
	f.immutable[f.client] = identity.RuntimeFiles[0].SHA
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
	f.evidence["acquisition"] = identity
	f.evidence["scanner"] = nativeProvisionScanner(t, f.base)

	return f
}
func (f *copilotProjectAttempt) authorOriginalConfig(t *testing.T) error {
	t.Helper()
	if f.started.IsZero() {
		return fmt.Errorf("held nonzero Copilot attempt start required")
	}
	body, err := json.Marshal(map[string]any{
		"firstLaunchAt":  f.started.UTC().Format(time.RFC3339Nano),
		"trustedFolders": []string{f.base.Project, f.other},
	})
	if err != nil {
		return fmt.Errorf("author original Copilot config: %w", err)
	}
	body = append(body, '\n')
	trust := filepath.Join(f.base.Home, ".copilot", "config.json")
	projectNativeRuntimeWrite(t, trust, body, 0600)
	f.originalConfigPath, f.originalConfigBaseline = trust, bytes.Clone(body)
	// Freeze the independently authored recipe, never a later observed rewrite.
	f.protected[trust] = copilotProjectConfigHash(body)
	return nil
}
func (f *copilotProjectAttempt) setup(t *testing.T) {
	t.Helper()
	for _, project := range []struct{ name, path string }{{"A", f.base.Project}, {"B", f.other}} {
		r := f.command(t, "git-"+project.name, project.path, f.env(), []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}, 30*time.Second)
		if len(r.Stdout) != 0 || len(r.Stderr) != 0 {
			t.Fatal("Git project setup emitted diagnostics")
		}
	}
	if err := f.authorOriginalConfig(t); err != nil {
		t.Fatal(err)
	}
	r := f.command(t, "version", f.outside, f.env(), []string{f.client, "--version"}, 30*time.Second)
	if string(r.Stdout) != "GitHub Copilot CLI 1.0.103.\nRun 'copilot update' to check for updates.\n" || len(r.Stderr) != 0 {
		t.Fatal("exact pinned Copilot version differs")
	}
	f.base.writePackage(t, "A-one", "1.0.0")
	// Keep the shared harmless probe, but use one distinct installed server and
	// one distinct skill. These are public package source bytes, never a fixture
	// replacement for the installed native entry.
	if err := os.RemoveAll(filepath.Join(f.base.Package, "skills")); err != nil {
		t.Fatal(err)
	}
	projectNativeRuntimeWrite(t, filepath.Join(f.base.Package, "skills", "uapcopilota", "SKILL.md"), []byte("---\nname: uapcopilota\ndescription: Disposable Copilot project proof\n---\nUAP_COPILOT_A_ONE_"+f.base.Nonce+"\n"), 0600)
	server := map[string]any{"type": "stdio", "command": "./bin/probe", "args": []string{"argument with spaces", "${PLUGIN_ROOT}/argument", "${UNKNOWN}"}, "cwd": "${PLUGIN_ROOT}", "env": map[string]string{"UAP_TEST_ROOT": f.base.Root, "UAP_TEST_EVENTS": f.base.Events, "UAP_TEST_NONCE": f.base.Nonce, "UAP_TEST_REVISION": "A-one", "UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": "${PLUGIN_ROOT}/${UNKNOWN}"}}
	nativeJSON(t, filepath.Join(f.base.Package, "mcp.json"), map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": map[string]any{"uapcopilota": server}})
	f.immutable[filepath.Join(f.base.Package, "bin", "probe")] = nativeSHA(t, filepath.Join(f.base.Package, "bin", "probe"))
	r = f.command(t, "add-A", f.base.Project, f.env(), []string{f.installer, "add", f.base.Package, "--no-save", "--scope", "project", "--project-root", f.base.Project, "--target", "copilot", "--format", "json"}, 30*time.Second)
	if string(r.Stderr) != "Resolving and validating one Agent Plugin package for every selected target...\nInstalling native-proof for 1 selected clients...\n" {
		t.Fatal("unexpected public installer diagnostic")
	}
	if _, err := projectNativeRuntimeJSON(r.Stdout); err != nil {
		t.Fatal(err)
	}
	if err := nativeValidateInstallerJSON(r.Stdout); err != nil {
		t.Fatal(err)
	}
	stateBody, err := projectNativeRuntimeRead(filepath.Join(f.base.Project, ".uap", "state-v2.json"), copilotProjectBodyLimit)
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
		if binding.ClientID != "copilot" || binding.Scope != "project" || binding.NativeProfileRoot != f.base.Project || binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || data.State != domain.DataReceiptOwned || data.Scope != "project" || data.DataReceiptID != binding.DataReceiptID || data.PhysicalBackend != binding.PhysicalArtifact || data.OwnershipDigest == "" {
			t.Fatal("installed Prepared/data authority differs")
		}
		nativeRequireContained(t, filepath.Join(f.base.Project, ".uap", "managed"), binding.TargetLocator)
		nativeRequireContained(t, filepath.Join(f.base.Project, ".uap", "plugin-data"), data.Locator)
		f.binding, f.data = binding, data.Locator
	}
	owned := map[string]string{}
	for _, object := range f.binding.NativeObjects {
		if object.Kind == "copilot_project_mcp_server" || object.Kind == "copilot_project_skill_directory" {
			if object.LogicalName != "uapcopilota" || object.ManagedDigest == "" || owned[object.Kind] != "" {
				t.Fatal("installed native ownership differs")
			}
			owned[object.Kind] = object.Path
		}
	}
	if owned["copilot_project_mcp_server"] != filepath.Join(f.base.Project, ".github", "mcp.json") || owned["copilot_project_skill_directory"] != filepath.Join(f.base.Project, ".github", "skills", "uapcopilota") {
		t.Fatal("installed native ownership paths differ")
	}
	f.captureInstalled(t, owned["copilot_project_mcp_server"], filepath.Join(owned["copilot_project_skill_directory"], "SKILL.md"))
	if err := f.installedReadiness(); err != nil {
		t.Fatal(err)
	}
	f.protected[owned["copilot_project_mcp_server"]] = copilotProjectConfigHash(f.files["mcp.json.installed"])
	f.protected[filepath.Join(owned["copilot_project_skill_directory"], "SKILL.md")] = copilotProjectConfigHash(f.files["uapcopilota-SKILL.md.installed"])
	f.evidence["binding"] = f.binding
	f.evidence["data_locator"] = f.data
	if _, err := os.Lstat(f.base.Events); !os.IsNotExist(err) {
		t.Fatal("probe started before native session")
	}
	f.check(t)
}
func (f *copilotProjectAttempt) validateFacts(body string) error {
	log, err := projectNativeRuntimeRead(f.base.Events, 2<<20)
	if err != nil {
		return err
	}
	return f.validateFactsLog(body, log)
}
func (f *copilotProjectAttempt) validateFactsLog(body string, log []byte) error {
	if len(log) > 2<<20 {
		return fmt.Errorf("Copilot probe event bound")
	}
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
	// Copilot expands its native env after UAP materializes the unchanged literal.
	// Bind that measured expansion to the held original HOME, never ambient env.
	if facts.Nonce != f.base.Nonce || facts.Revision != "A-one" || facts.Root != f.binding.TargetLocator || facts.Data != f.data || facts.CWD != f.binding.TargetLocator || !reflect.DeepEqual(facts.Argv, []string{"argument with spaces", f.binding.TargetLocator + "/argument", "${UNKNOWN}"}) || facts.Literal != "literal ${UNKNOWN} "+f.base.Home || facts.Once != f.binding.TargetLocator+"/${UNKNOWN}" || facts.Marker != "retained-"+f.base.Nonce || facts.PID <= 0 || facts.Session == "" {
		return fmt.Errorf("returned probe authority/nonce/revision/cwd/marker differs")
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
func (f *copilotProjectAttempt) finish(t *testing.T) {
	t.Helper()
	defer f.cancel()
	if time.Since(f.started) > 180*time.Second {
		t.Error("attempt total budget exceeded")
	}
	if f.base != nil {
		if body, err := projectNativeRuntimeRead(f.base.Events, 2<<20); err == nil {
			f.keep(t, "probe-events.jsonl", body)
		}
	}
	f.finalizeReadiness(t)
	f.evidence["test_failed"] = t.Failed()
	f.evidence["elapsed_ms"] = time.Since(f.started).Milliseconds()
	body, _ := json.MarshalIndent(f.evidence, "", "  ")
	f.keep(t, "runtime-attempt.json", append(body, '\n'))
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

func TestAgentpluginsCopilotNativeProjectRuntimeAttempt(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_COPILOT_PROJECT_RUNTIME_ATTEMPT") != "1" {
		t.Skip("opt-in current-source runtime research")
	}
	f := newCopilotProjectAttempt(t)
	defer f.finish(t)
	f.setup(t)
	p := newCopilotProjectProvider("uapcopilota-inspect_runtime", f.base.Nonce, f.validateFacts)
	if err := p.start(f.ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		p.server.Close()
		f.evidence["provider_closed"] = true
		p.mu.Lock()
		defer p.mu.Unlock()
		for i, body := range p.requests {
			f.keep(t, fmt.Sprintf("provider-request-%d.json", i+1), body)
		}
		f.evidence["provider_returned"] = p.returned
		if p.err != nil {
			f.evidence["provider_error"] = p.err.Error()
		}
	}()
	env := append(f.env(), "COPILOT_PROVIDER_TYPE=openai", "COPILOT_PROVIDER_BASE_URL="+p.server.URL+"/v1", "COPILOT_PROVIDER_WIRE_API=completions", "COPILOT_MODEL=uap-fixture-model")
	args := []string{f.client, "-p", copilotReadinessPrompt, "--output-format", "json", "--no-auto-update", "--disable-client-telemetry", "--disable-builtin-mcps", "--available-tools=uapcopilota-inspect_runtime", "--allow-tool=uapcopilota(inspect_runtime)", "--deny-tool=shell", "--deny-tool=write", "--deny-tool=read", "--deny-tool=url", "--deny-tool=memory"}
	result := f.command(t, "native-attempt", f.base.Project, env, args, 60*time.Second)
	p.server.Close()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil || !p.returned || len(p.requests) != 2 {
		t.Fatalf("native provider exchange incomplete: %v", p.err)
	}
	f.evidence["probe_facts"] = f.facts
	if len(result.Stderr) != 0 {
		t.Fatal("Copilot readiness native stderr is not empty")
	}
	session, err := f.readiness(result.Stdout, p)
	if err != nil {
		t.Fatal(err)
	}
	f.check(t)
	f.evidence["native_session_id"] = session
	f.evidence["provider_call_id"] = p.id
	f.readinessObserved = true
}
