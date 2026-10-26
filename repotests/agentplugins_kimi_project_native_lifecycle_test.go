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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const kimiProjectNativeLimit = 1 << 20
const kimiProjectArchiveSHA = "4715ccd1a591b64683929a14b26b3d07ac4c484e4da3881b22a2ccca56304042"
const kimiProjectExecutableSHA = "1cc5979fca0ea8c01c0a4d53806df8e65573cfbe7daf17ccff86db392807e4fa"

type kimiProjectNativePackage struct {
	Source, Name, Revision, Skill, Data string
	Installation                        domain.Installation
	Binding                             domain.ClientBinding
}

type kimiProjectNativeFixture struct {
	base                                   *nativeFixture
	client, installer, python, git, output string
	projects                               map[string]string
	packages                               map[string]*kimiProjectNativePackage
	immutable, protected                   map[string]string
	files                                  map[string][]byte
	ctx                                    context.Context
	cancel                                 context.CancelFunc
	started                                time.Time
	evidence                               map[string]any
	serverIDs                              map[string]bool
	foreignMCP                             map[string]any
	protectedTrees                         map[string]map[string]string
}

func (f *kimiProjectNativeFixture) keep(t *testing.T, name string, body []byte) {
	t.Helper()
	total := len(body)
	for _, b := range f.files {
		total += len(b)
	}
	if filepath.Base(name) != name || f.files[name] != nil || len(f.files) >= 120 || len(body) > kimiProjectNativeLimit || total > 24<<20 {
		t.Fatal("Kimi closed evidence bounds refused")
	}
	f.files[name] = bytes.Clone(body)
}

func (f *kimiProjectNativeFixture) check(t *testing.T) {
	t.Helper()
	for _, inventory := range []map[string]string{f.immutable, f.protected} {
		for path, want := range inventory {
			got, err := projectNativeRuntimeDigest(path)
			if err != nil || got != want {
				t.Fatalf("Kimi authority identity changed: %s (%v)", path, err)
			}
		}
	}
	for root, expected := range f.protectedTrees {
		if !reflect.DeepEqual(kimiProjectTree(t, root), expected) {
			t.Fatal("Kimi unrelated managed package inventory changed")
		}
	}
	for path, expected := range f.foreignMCP {
		body, err := projectNativeRuntimeRead(path, kimiProjectNativeLimit)
		if err != nil {
			t.Fatal(err)
		}
		object, err := kimiProjectObject(body)
		if err != nil {
			t.Fatal(err)
		}
		entries, ok := object["mcpServers"].(map[string]any)
		if !ok || !reflect.DeepEqual(entries["foreign-preserve"], expected) {
			t.Fatal("Kimi unrelated native MCP member changed")
		}
	}
}

func (f *kimiProjectNativeFixture) env() []string {
	return append(f.base.env(filepath.Dir(f.client)), "NO_COLOR=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
}

func (f *kimiProjectNativeFixture) command(t *testing.T, label, cwd string, env, argv []string, timeout time.Duration, absent bool) ports.CommandResult {
	t.Helper()
	f.check(t)
	ctx, cancel := context.WithTimeout(f.ctx, timeout)
	defer cancel()
	r, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: argv, Env: env, Dir: cwd, StdoutLimitBytes: kimiProjectNativeLimit}, 5*time.Second)
	f.keep(t, label+".stdout.log", r.Stdout)
	f.keep(t, label+".stderr.log", r.Stderr)
	meta, _ := json.Marshal(map[string]any{"argv": argv, "cwd": cwd, "exit": r.ExitCode, "error": err != nil, "deadline": ctx.Err() != nil, "cleanup": "ordinary full-descendant supervision; forced termination fails"})
	f.keep(t, label+".command.json", meta)
	f.check(t)
	if ctx.Err() != nil || len(r.Stdout) > kimiProjectNativeLimit || bytes.Contains(r.Stderr, []byte("stderr bytes omitted")) || (!absent && (err != nil || r.ExitCode != 0)) || (absent && (err != nil || r.ExitCode != 1)) {
		t.Fatalf("Kimi command %s failed; bounded evidence retained", label)
	}
	return r
}

func newKimiProjectNativeFixture(t *testing.T) *kimiProjectNativeFixture {
	t.Helper()
	started := time.Now()
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED") != "1" {
		t.Fatal("Kimi native execution requires actual opted-in GitHub-hosted Linux amd64")
	}
	f := &kimiProjectNativeFixture{started: started, packages: map[string]*kimiProjectNativePackage{}, immutable: map[string]string{}, protected: map[string]string{}, files: map[string][]byte{}, serverIDs: map[string]bool{}, foreignMCP: map[string]any{}, protectedTrees: map[string]map[string]string{}, evidence: map[string]any{"client": "kimi", "activation": "prepared", "qualification": map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}, "vendor_authentication_qualified": false, "real_model_qualified": false}}
	f.ctx, f.cancel = context.WithDeadline(context.Background(), started.Add(840*time.Second))
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

func (f *kimiProjectNativeFixture) source(t *testing.T, scope, revision string) {
	t.Helper()
	name := "uap-kimi-native-" + scope
	root := filepath.Join(f.base.Root, "sources", scope)
	write := func(path string, body []byte, mode os.FileMode) {
		if _, err := os.Lstat(path); err == nil {
			if _, err := projectNativeRuntimeRead(path, 256<<20); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		projectNativeRuntimeWrite(t, path, body, mode)
	}
	version := "1.0.0"
	if revision == "a-two" {
		version = "1.0.1"
	}
	manifest, _ := json.Marshal(map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": name, "version": version})
	write(filepath.Join(root, "plugin.json"), manifest, 0600)
	skill := []byte(fmt.Sprintf("---\nname: %s\ndescription: Kimi installed %s\n---\nUAP_KIMI_%s_%s\n", name, revision, revision, f.base.Nonce))
	write(filepath.Join(root, "skills", name, "SKILL.md"), skill, 0600)
	if scope != "user" {
		probe, err := projectNativeRuntimeRead(f.base.Probe, 256<<20)
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(root, "bin", "probe"), probe, 0700)
		server := map[string]any{"type": "stdio", "command": "./bin/probe", "args": []string{"Kimi owned " + scope, "${PLUGIN_ROOT}/argument"}, "cwd": "${PLUGIN_ROOT}", "env": map[string]string{"UAP_TEST_ROOT": f.base.Root, "UAP_TEST_EVENTS": filepath.Join(f.base.Root, scope+"-events.jsonl"), "UAP_TEST_NONCE": f.base.Nonce + "-" + scope, "UAP_TEST_REVISION": revision}}
		mcp, _ := json.Marshal(map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": map[string]any{name: server}})
		write(filepath.Join(root, "mcp.json"), mcp, 0600)
	}
	f.packages[scope] = &kimiProjectNativePackage{Source: root, Name: name, Revision: revision}
}

func (f *kimiProjectNativeFixture) uap(t *testing.T, label, scope string, mutated, absent bool, args ...string) {
	t.Helper()
	argv := append([]string{f.installer}, args...)
	argv = append(argv, "--target", "kimi", "--format", "json")
	cwd := f.projects["user"]
	if scope == "user" {
		argv = append(argv, "--scope", "user")
	} else {
		cwd = f.projects[scope]
		argv = append(argv, "--scope", "project", "--project-root", cwd)
	}
	r := f.command(t, label, cwd, f.env(), argv, 30*time.Second, absent)
	if absent {
		first := "agentplugins: installation \"" + f.packages[scope].Name + "\" was not found\n"
		second := "agentplugins: plugin is not installed for target \"kimi\" in project scope; no target was changed\n"
		if len(r.Stdout) != 0 || (string(r.Stderr) != first && string(r.Stderr) != second) {
			t.Fatal("Kimi repeated removal is not the exact absent-installation refusal")
		}
		return
	}
	value, err := kimiProjectObject(r.Stdout)
	if err != nil {
		t.Fatal(err)
	}
	if err := nativeValidateInstallerJSON(r.Stdout); err != nil {
		t.Fatal(err)
	}
	commandsRequireMutated(t, label, value, mutated)
	var progress string
	switch args[0] {
	case "add":
		progress = "Resolving and validating one Agent Plugin package for every selected target...\n"
		if strings.HasPrefix(label, "repeat-") {
			progress += "Applying transactional client package...\n"
		} else {
			progress += "Installing " + f.packages[scope].Name + " for 1 selected clients...\n"
		}
	case "update":
		progress = "Resolving and validating one updated Agent Plugin package for every selected target...\n"
	case "repair":
		progress = "Resolving and validating each unique exact installed package revision once...\n"
	}
	if string(r.Stderr) != progress {
		t.Fatal("Kimi public CLI emitted unexpected diagnostics")
	}
}

func (f *kimiProjectNativeFixture) receipt(t *testing.T, scope string) {
	t.Helper()
	p := f.packages[scope]
	statePath := filepath.Join(f.projects[scope], ".uap", "state-v2.json")
	if scope == "user" {
		statePath = filepath.Join(f.base.Root, "installer-state", "state-v2.json")
	}
	body, err := projectNativeRuntimeRead(statePath, kimiProjectNativeLimit)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kimiProjectObject(body); err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if json.Unmarshal(body, &state) != nil {
		t.Fatal("Kimi installed state malformed")
	}
	var selected []domain.Installation
	for _, installation := range state.Installations {
		if installation.DeclaredName == p.Name {
			selected = append(selected, installation)
		}
	}
	if len(selected) != 1 || len(selected[0].Clients) != 1 {
		t.Fatal("Kimi exact package/binding receipt missing")
	}
	i := selected[0]
	if i.Source.CanonicalSource != p.Source || i.Source.SourceBindingID == "" || i.Source.TreeDigest == "" || i.Package.DeclaredName != p.Name || i.Package.ManifestDigest == "" {
		t.Fatal("Kimi selected source/package identity differs")
	}
	for _, binding := range i.Clients {
		if binding.ClientID != "kimi" || binding.Scope != map[bool]string{true: "user", false: "project"}[scope == "user"] || binding.TargetLocator == "" {
			t.Fatal("Kimi receipt scope differs")
		}
		if scope != "user" {
			if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || binding.NativeProfileRoot != filepath.Join(f.projects[scope], ".kimi-code") {
				t.Fatal("Kimi project authority/status changed")
			}
			data := i.DataReceipts[binding.DataReceiptID]
			if data.DataReceiptID == "" || data.State != domain.DataReceiptOwned || data.OwnershipDigest == "" || data.Scope != "project" || data.PhysicalBackend != binding.PhysicalArtifact {
				t.Fatal("Kimi persistent data receipt missing")
			}
			nativeRequireContained(t, filepath.Join(f.projects[scope], ".uap", "managed"), binding.TargetLocator)
			nativeRequireContained(t, filepath.Join(f.projects[scope], ".uap", "plugin-data"), data.Locator)
			p.Data = data.Locator
			owned := map[string]string{}
			for _, object := range binding.NativeObjects {
				if object.Kind == "kimi_project_skill_directory" || object.Kind == "kimi_project_mcp_server" {
					if object.LogicalName != p.Name || object.ManagedDigest == "" || owned[object.Kind] != "" {
						t.Fatal("Kimi native ownership differs")
					}
					owned[object.Kind] = object.Path
				}
			}
			if owned["kimi_project_skill_directory"] != filepath.Join(f.projects[scope], ".kimi-code", "skills", p.Name) || owned["kimi_project_mcp_server"] != filepath.Join(f.projects[scope], ".kimi-code", "mcp.json") {
				t.Fatal("Kimi native ownership paths differ")
			}
			p.Skill = filepath.Join(owned["kimi_project_skill_directory"], "SKILL.md")
		} else {
			p.Skill = filepath.Join(binding.TargetLocator, "skills", p.Name, "SKILL.md")
		}
		p.Binding = binding
	}
	p.Installation = i
	got, err := projectNativeRuntimeRead(p.Skill, kimiProjectNativeLimit)
	if err != nil {
		t.Fatal(err)
	}
	want, err := projectNativeRuntimeRead(filepath.Join(p.Source, "skills", p.Name, "SKILL.md"), kimiProjectNativeLimit)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("Kimi selected native skill bytes differ")
	}
}

func (f *kimiProjectNativeFixture) observe(t *testing.T, stage string, trust bool, installed ...string) {
	t.Helper()
	if time.Until(f.started.Add(840*time.Second)) < 75*time.Second {
		t.Fatal("Kimi cannot start native stage inside cleanup reserve")
	}
	selected := map[string]bool{}
	for _, scope := range installed {
		selected[scope] = true
	}
	skills, servers := map[string][]any{}, map[string][]any{}
	for _, scope := range []string{"a", "b", "c"} {
		skills[scope], servers[scope] = []any{}, []any{}
		for _, key := range []string{"user", scope} {
			if !selected[key] {
				continue
			}
			p := f.packages[key]
			f.receipt(t, key)
			source := "project"
			if key == "user" {
				source = "extra"
			}
			skills[scope] = append(skills[scope], map[string]any{"name": p.Name, "description": "Kimi installed " + p.Revision, "path": p.Skill, "source": source, "sha256": nativeSHA(t, p.Skill)})
		}
		if selected[scope] {
			p := f.packages[scope]
			origin := filepath.Join(f.projects[scope], ".kimi-code", "mcp.json")
			body, err := projectNativeRuntimeRead(origin, kimiProjectNativeLimit)
			if err != nil {
				t.Fatal(err)
			}
			object, err := kimiProjectObject(body)
			if err != nil {
				t.Fatal(err)
			}
			entries, ok := object["mcpServers"].(map[string]any)
			if !ok {
				t.Fatal("Kimi native MCP collection missing")
			}
			config, ok := entries[p.Name].(map[string]any)
			if !ok {
				t.Fatal("Kimi native owned MCP missing")
			}
			if err := kimiProjectValidateServer(config, p, scope, f.base.Root, f.base.Nonce); err != nil {
				t.Fatal(err)
			}
			servers[scope] = append(servers[scope], map[string]any{"name": p.Name, "origin": origin, "config": config, "sha256": nativeSHA(t, origin), "event": filepath.Join(f.base.Root, scope+"-events.jsonl"), "nonce": f.base.Nonce + "-" + scope, "cwd": f.projects[scope]})
		}
	}
	observerIDs := map[string]string{}
	for path, digest := range f.immutable {
		if path != f.python && path != f.git {
			observerIDs[path] = digest
		}
	}
	for _, scope := range installed {
		if scope != "user" {
			p := f.packages[scope]
			for _, path := range []string{filepath.Join(p.Binding.TargetLocator, "bin", "probe"), filepath.Join(p.Binding.TargetLocator, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)} {
				digest, err := projectNativeRuntimeDigest(path)
				if err != nil {
					t.Fatal(err)
				}
				observerIDs[path] = digest
			}
		}
	}
	request := map[string]any{"schema_version": 1, "root": f.base.Root, "home": f.base.Home, "projects": f.projects, "client": f.client, "identities": observerIDs, "observation": map[string]any{"stage": stage, "setup_trust": trust, "skills": skills, "servers": servers}}
	body, _ := json.Marshal(request)
	path := filepath.Join(f.base.Root, stage+"-private-request.json")
	projectNativeRuntimeWrite(t, path, body, 0600)
	env := append(f.env(), "GITHUB_ACTIONS="+os.Getenv("GITHUB_ACTIONS"), "RUNNER_ENVIRONMENT="+os.Getenv("RUNNER_ENVIRONMENT"), "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED="+os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED"))
	r := f.command(t, "observe-"+stage, f.projects["user"], env, []string{f.python, filepath.Join(RepoRoot(t), "scripts", "project_native_collectors", "kimi.py"), "--installed-request", path}, 70*time.Second, false)
	if len(r.Stderr) != 0 {
		t.Fatal("Kimi observer diagnostics refused")
	}
	value, err := kimiProjectObject(r.Stdout)
	if err != nil {
		t.Fatal(err)
	}
	files, ok := value["files"].(map[string]any)
	if !ok || value["schema_version"] != json.Number("1") {
		t.Fatal("Kimi closed observer envelope malformed")
	}
	summaryText, ok := files["research.json"].(string)
	if !ok {
		t.Fatal("Kimi admitted native summary missing")
	}
	summary, err := kimiProjectObject([]byte(summaryText))
	if err != nil {
		t.Fatal(err)
	}
	observation, ok := summary["observations"].(map[string]any)
	if !ok || observation["stage"] != stage || observation["tools_call_count"] != json.Number("0") || observation["provider_configured"] != false {
		t.Fatal("Kimi native stage outcome differs")
	}
	id, ok := observation["server_id"].(string)
	if !ok || id == "" || f.serverIDs[id] {
		t.Fatal("Kimi fresh native restart identity missing")
	}
	f.serverIDs[id] = true
	f.evidence[stage] = observation
}

func (f *kimiProjectNativeFixture) finish(t *testing.T) {
	t.Helper()
	defer f.cancel()
	if time.Since(f.started) > 900*time.Second {
		t.Error("Kimi original total deadline exceeded")
	}
	f.evidence["test_failed"], f.evidence["elapsed_ms"] = t.Failed(), time.Since(f.started).Milliseconds()
	if t.Failed() {
		for key := range f.evidence["qualification"].(map[string]bool) {
			f.evidence["qualification"].(map[string]bool)[key] = false
		}
	}
	body, _ := json.MarshalIndent(f.evidence, "", "  ")
	f.keep(t, "lifecycle.json", body)
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
		w, err := archive.Create(name)
		if err != nil {
			t.Error(err)
			break
		}
		if _, err = w.Write(f.files[name]); err != nil {
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

func TestAgentpluginsKimiNativeProjectComponentsLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_KIMI_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("opt-in current-source Kimi installed native lifecycle")
	}
	f := newKimiProjectNativeFixture(t)
	defer f.finish(t)
	for _, scope := range []string{"a", "b", "c"} {
		r := f.command(t, "git-"+scope, f.projects[scope], f.env(), []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}, 30*time.Second, false)
		if len(r.Stdout) != 0 || len(r.Stderr) != 0 {
			t.Fatal("Kimi Git boundary setup failed")
		}
	}
	r := f.command(t, "version", f.projects["user"], f.env(), []string{f.client, "--version"}, 30*time.Second, false)
	if string(r.Stdout) != "2.3.2\n" || len(r.Stderr) != 0 {
		t.Fatal("Kimi exact native version differs")
	}
	for _, scope := range []string{"user", "a", "b", "c"} {
		f.source(t, scope, scope+"-one")
	}
	for _, root := range []string{f.base.Home, f.projects["a"], f.projects["b"], f.projects["c"]} {
		path := filepath.Join(root, ".kimi-code", "foreign-keep.txt")
		projectNativeRuntimeWrite(t, path, []byte("original foreign authority\n"), 0600)
		f.protected[path] = nativeSHA(t, path)
		skill := filepath.Join(root, ".kimi-code", "skills", "foreign-preserve", "SKILL.md")
		projectNativeRuntimeWrite(t, skill, []byte("---\nname: foreign-preserve\ndescription: Retained foreign marker\n---\nRetain exact original bytes.\n"), 0600)
		f.protected[skill] = nativeSHA(t, skill)
		entry := map[string]any{"transport": "http", "url": "http://127.0.0.1:9/foreign", "enabled": false}
		mcp := filepath.Join(root, ".kimi-code", "mcp.json")
		body, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"foreign-preserve": entry}})
		projectNativeRuntimeWrite(t, mcp, body, 0600)
		f.foreignMCP[mcp] = entry
	}
	preferences := filepath.Join(f.base.Home, ".kimi-code", "config.toml")
	projectNativeRuntimeWrite(t, preferences, []byte("telemetry = false\n"), 0600)
	f.protected[preferences] = nativeSHA(t, preferences)
	userMCP := filepath.Join(f.base.Home, ".kimi-code", "mcp.json")
	f.protected[userMCP] = nativeSHA(t, userMCP)
	f.observe(t, "empty", true)
	for _, scope := range []string{"user", "a", "b", "c"} {
		f.uap(t, "add-"+scope, scope, true, false, "add", f.packages[scope].Source, "--no-save")
		f.receipt(t, scope)
	}
	for _, scope := range []string{"a", "b", "c"} {
		p := f.packages[scope]
		path := filepath.Join(p.Data, "keep.txt")
		projectNativeRuntimeWrite(t, path, []byte("retained-"+f.base.Nonce+"-"+scope), 0600)
		f.protected[path] = nativeSHA(t, path)
	}
	f.observe(t, "installed", false, "user", "a", "b", "c")
	userState := filepath.Join(f.base.Root, "installer-state", "state-v2.json")
	f.protected[userState] = nativeSHA(t, userState)
	for _, scope := range []string{"user", "b", "c"} {
		f.protected[f.packages[scope].Skill] = nativeSHA(t, f.packages[scope].Skill)
		f.protectedTrees[f.packages[scope].Binding.TargetLocator] = kimiProjectTree(t, f.packages[scope].Binding.TargetLocator)
		if scope != "user" {
			for _, path := range []string{filepath.Join(f.projects[scope], ".uap", "state-v2.json"), filepath.Join(f.projects[scope], ".kimi-code", "mcp.json")} {
				f.protected[path] = nativeSHA(t, path)
			}
		}
	}
	userRegistry := filepath.Join(f.base.Home, ".kimi-code", "plugins", "installed.json")
	f.protected[userRegistry] = nativeSHA(t, userRegistry)
	state := filepath.Join(f.projects["a"], ".uap", "state-v2.json")
	stable := nativeSHA(t, state)
	nativeStable := nativeSHA(t, f.packages["a"].Skill)
	aMCP := filepath.Join(f.projects["a"], ".kimi-code", "mcp.json")
	nativeMCPStable := nativeSHA(t, aMCP)
	f.uap(t, "repeat-add-a", "a", false, false, "add", f.packages["a"].Source, "--no-save")
	f.uap(t, "unchanged-update-a", "a", false, false, "update", f.packages["a"].Name)
	if nativeSHA(t, state) != stable || nativeSHA(t, f.packages["a"].Skill) != nativeStable || nativeSHA(t, aMCP) != nativeMCPStable {
		t.Fatal("Kimi unchanged reconciliation rewrote state/native bytes")
	}
	f.uap(t, "intact-repair-a", "a", true, false, "repair", f.packages["a"].Name)
	if nativeSHA(t, f.packages["a"].Skill) != nativeStable || nativeSHA(t, aMCP) != nativeMCPStable {
		t.Fatal("Kimi intact repair changed native bytes")
	}
	f.observe(t, "stable", false, "user", "a", "b", "c")
	old := f.packages["a"]
	f.source(t, "a", "a-two")
	f.uap(t, "update-a", "a", true, false, "update", f.packages["a"].Source)
	f.receipt(t, "a")
	if f.packages["a"].Installation.Source.TreeDigest == old.Installation.Source.TreeDigest || f.packages["a"].Data != old.Data {
		t.Fatal("Kimi update source/data identity differs")
	}
	f.observe(t, "updated", false, "user", "a", "b", "c")
	skillDir := filepath.Dir(f.packages["a"].Skill)
	if skillDir != filepath.Join(f.projects["a"], ".kimi-code", "skills", f.packages["a"].Name) {
		t.Fatal("Kimi missing repair removal is outside exact owned skill")
	}
	if _, err := projectNativeRuntimeRead(f.packages["a"].Skill, kimiProjectNativeLimit); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(f.packages["a"].Skill); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(skillDir); err != nil {
		t.Fatal(err)
	}
	f.uap(t, "missing-repair-a", "a", true, false, "repair", f.packages["a"].Name)
	f.receipt(t, "a")
	f.observe(t, "repaired", false, "user", "a", "b", "c")
	f.observe(t, "restart", false, "user", "a", "b", "c")
	beforeEvent, err := projectNativeRuntimeRead(filepath.Join(f.base.Root, "a-events.jsonl"), 2<<20)
	if err != nil {
		t.Fatal(err)
	}
	f.uap(t, "remove-a", "a", true, false, "remove", f.packages["a"].Name)
	if _, err := os.Lstat(skillDir); !os.IsNotExist(err) {
		t.Fatal("Kimi removed native skill survives")
	}
	f.observe(t, "removed", false, "user", "b", "c")
	afterEvent, err := projectNativeRuntimeRead(filepath.Join(f.base.Root, "a-events.jsonl"), 2<<20)
	if err != nil || !bytes.Equal(beforeEvent, afterEvent) {
		t.Fatal("Kimi removed A started native probe")
	}
	removedState := nativeSHA(t, state)
	retainedA := map[string]map[string]string{}
	for _, root := range []string{filepath.Join(f.projects["a"], ".uap"), filepath.Join(f.projects["a"], ".kimi-code")} {
		retainedA[root] = kimiProjectTree(t, root)
	}
	f.uap(t, "repeat-remove-a", "a", false, true, "remove", f.packages["a"].Name)
	if nativeSHA(t, state) != removedState {
		t.Fatal("Kimi repeat removal changed state")
	}
	for root, before := range retainedA {
		if !reflect.DeepEqual(before, kimiProjectTree(t, root)) {
			t.Fatal("Kimi recognized repeated removal changed retained A authority")
		}
	}
	f.check(t)
	qualification := f.evidence["qualification"].(map[string]bool)
	for _, key := range []string{"project-lifecycle", "native-restart-removal", "mcp-initialize", "mcp-tools-list"} {
		qualification[key] = true
	}
	f.evidence["boundary"] = "Native authenticated local API discovery and named initialize/tools-list only; no tools/call, model, vendor OAuth or released-client qualification"
}

func kimiProjectObject(body []byte) (map[string]any, error) {
	value, err := projectNativeRuntimeJSON(body)
	if err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Kimi JSON object required")
	}
	return object, nil
}

// The expected projection is fixed by this source package and the selected
// receipt, independently of the bytes that the native API is about to read.
func kimiProjectValidateServer(actual map[string]any, p *kimiProjectNativePackage, scope, root, nonce string) error {
	active := p.Binding.TargetLocator
	expected := map[string]any{
		"transport": "stdio",
		"command":   filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName),
		"args":      []any{"--internal-stdio-v1", active, p.Data, active, "plugin", "--", "./bin/probe", "Kimi owned " + scope, active + "/argument"},
		"env":       map[string]any{"PLUGIN_ROOT": active, "PLUGIN_DATA": p.Data, "UAP_TEST_ROOT": root, "UAP_TEST_EVENTS": filepath.Join(root, scope+"-events.jsonl"), "UAP_TEST_NONCE": nonce + "-" + scope, "UAP_TEST_REVISION": p.Revision},
	}
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("Kimi native stdio projection differs from selected receipt/package/data/cwd/source arguments")
	}
	return nil
}

func kimiProjectTree(t *testing.T, root string) map[string]string {
	t.Helper()
	if err := projectNativeRuntimeParents(filepath.Join(root, "leaf")); err != nil {
		t.Fatal(err)
	}
	inventory := map[string]string{}
	var total int64
	entries := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if len(strings.Split(rel, string(os.PathSeparator))) > 32 {
			return fmt.Errorf("owned package depth limit")
		}
		entries++
		if entries > 4096 {
			return fmt.Errorf("owned package entry count limit")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if err := projectNativeRuntimeParents(filepath.Join(path, "leaf")); err != nil {
				return err
			}
			inventory[rel] = fmt.Sprintf("%#o:directory", info.Mode())
			return nil
		}
		total += info.Size()
		if total > 256<<20 {
			return fmt.Errorf("owned package byte limit")
		}
		digest, err := projectNativeRuntimeDigest(path)
		if err != nil {
			return err
		}
		inventory[rel] = fmt.Sprintf("%#o:%s", info.Mode(), digest)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}
