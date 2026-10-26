//go:build linux

package pluginkitairepo_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// A closed four-client research table, not a native client/plugin framework.
// Every first native session remains failed/unqualified until its genuine
// output and exact authentication/trust/protocol boundary have been reviewed.
type projectNativeCLISpec struct {
	client, binEnv, optin, version, archiveSHA, member, executableSHA, versionOutput string
	executableBytes                                                                  int64
	mixed                                                                            bool
}

func projectNativeCLISpecification(client string) (projectNativeCLISpec, error) {
	switch client {
	case "qoder":
		return projectNativeCLISpec{client, "AGENTPLUGINS_QODER_BIN", "AGENTPLUGINS_QODER_PROJECT_RUNTIME_ATTEMPT", "1.1.78", "5e3213d2d4256c86c5ad2c8bf73fd392f92ba80d0d38f822806542805a48a8ea", "qodercli", "0da0976e13fe00bc28f725d6fe7c2f0fe9fa9a8c37eeb2ee4e80ad4c0fc58953", "1.1.78\n", 182982112, false}, nil
	case "grok":
		return projectNativeCLISpec{client, "AGENTPLUGINS_GROK_BIN", "AGENTPLUGINS_GROK_PROJECT_RUNTIME_ATTEMPT", "1.0.70", "11d27a30adfb75c4b9b180072ab27412a1916f054ff5497ab70fb9a77d304bcc", "grok", "22d281c5964c5444147328096ba2a221f3f0874c6858e27c825656a80f6e58e9", "grok 1.0.70 (f50904eaee6c)\n", 195700192, false}, nil
	case "kiro":
		return projectNativeCLISpec{client, "AGENTPLUGINS_KIRO_BIN", "AGENTPLUGINS_KIRO_PROJECT_RUNTIME_ATTEMPT", "2.34.0", "11dce9286cf7401079cf4eac02a3d5982daac16a346d880109ec7b4a0b941040", "kirocli/bin/kiro-cli", "94cae7e349f76869a19eae335a5b392fba8515743b1c2822b9ea33128c067cc6", "kiro-cli 2.34.0\n", 43027744, true}, nil
	case "windsurf":
		return projectNativeCLISpec{client, "AGENTPLUGINS_WINDSURF_BIN", "AGENTPLUGINS_WINDSURF_PROJECT_RUNTIME_ATTEMPT", "3000.11.205", "2e65c8b93682258fc91f5e4cead92e3698338cebf1b31dbc32a3201cb63705c4", "bin/devin", "642c82e32cb3dec90cf5ab01975520b425307872c7a650c9fb5b0653f37681f4", "devin 3000.11.205 (6dea39440ae3)\n", 217494824, true}, nil
	default:
		return projectNativeCLISpec{}, fmt.Errorf("unregistered native CLI attempt")
	}
}
func projectNativeCLIArguments(client, executable, view, prompt string) ([]string, error) {
	switch client {
	case "qoder":
		if !filepath.IsAbs(view) || filepath.Clean(view) != view || filepath.Base(view) != ".agentplugins-qoder-native" {
			return nil, fmt.Errorf("receipt-selected closed Qoder view required")
		}
		return []string{executable, "--print", "--output-format", "stream-json", "--no-session-persistence", "--permission-mode", "default", "--tools", "", "--plugin-dir", view, prompt}, nil
	case "grok":
		// Source-guided candidate: unlike --tools "" (which parses as None), the
		// strict permission parser maps --deny "" to Deny/Any/no pattern.
		return []string{executable, "--single", prompt, "--output-format", "streaming-json", "--permission-mode", "default", "--deny", "", "--no-subagents", "--disable-web-search", "--max-turns", "1"}, nil
	case "kiro":
		return []string{executable, "chat", "--v2", "--no-interactive", prompt}, nil
	case "windsurf":
		return []string{executable, "--print", prompt, "--respect-workspace-trust", "true"}, nil
	default:
		return nil, fmt.Errorf("unregistered native CLI argv")
	}
}
func projectNativeCLIUpdater(client string) (string, []byte) {
	switch client {
	case "qoder":
		return ".qoder/settings.json", []byte("{\"general\":{\"enableAutoUpdate\":false}}\n")
	case "windsurf":
		return "devin/config.json", []byte("{\"auto_update\":false,\"permissions\":{\"allow\":[],\"deny\":[\"read\",\"edit\",\"write\",\"grep\",\"glob\",\"exec\",\"fetch\",\"web_fetch\",\"mcp__*\"],\"ask\":[]}}\n")
	default:
		return "", nil
	}
}

type projectNativeCLIAttempt struct {
	spec                                                                         projectNativeCLISpec
	base                                                                         *nativeFixture
	client, installer, git, other, outside, output, bin, name, skill, view, data string
	started                                                                      time.Time
	ctx                                                                          context.Context
	cancel                                                                       context.CancelFunc
	files                                                                        map[string][]byte
	immutable, protected                                                         map[string]string
	used                                                                         map[string]bool
	evidence                                                                     map[string]any
	binding                                                                      domain.ClientBinding
}

func (f *projectNativeCLIAttempt) keep(t *testing.T, name string, body []byte) {
	t.Helper()
	total := len(body)
	for _, b := range f.files {
		total += len(b)
	}
	if filepath.Base(name) != name || f.files[name] != nil || len(f.files) >= 80 || len(body) > 1<<20 || total > 16<<20 {
		t.Fatal("closed CLI evidence bound")
	}
	f.files[name] = bytes.Clone(body)
}
func (f *projectNativeCLIAttempt) check(t *testing.T) {
	t.Helper()
	for _, inventory := range []map[string]string{f.immutable, f.protected} {
		for path, want := range inventory {
			got, err := projectNativeRuntimeDigest(path)
			if err != nil || got != want {
				t.Fatalf("native CLI immutable/protected identity changed: %s (%v)", path, err)
			}
		}
	}
	if f.spec.client == "qoder" && f.bin != "" {
		path := filepath.Join(f.bin, "qoder")
		info, err := os.Lstat(path)
		target, linkErr := os.Readlink(path)
		if err != nil || linkErr != nil || info.Mode()&os.ModeSymlink == 0 || target != f.client {
			t.Fatal("recorded genuine Qoder command symlink changed")
		}
	}
}
func (f *projectNativeCLIAttempt) env() []string {
	env := append(f.base.env(f.bin), "NO_COLOR=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
	if f.spec.client == "grok" {
		env = append(env, "GROK_DISABLE_AUTOUPDATER=1")
	}
	return env
}
func (f *projectNativeCLIAttempt) selectedCommand(label, cwd string, argv []string, timeout time.Duration) error {
	expectedCWD, expectedTimeout := f.base.Project, 30*time.Second
	var expected []string
	switch label {
	case "git-a":
		expected = []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
	case "git-b":
		expectedCWD = f.other
		expected = []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
	case "version":
		expectedCWD = f.outside
		expected = []string{f.client, "--version"}
	case "add-a":
		expected = []string{f.installer, "add", f.base.Package, "--no-save", "--scope", "project", "--project-root", f.base.Project, "--target", f.spec.client, "--format", "json"}
	case "native-attempt":
		expectedTimeout = 60 * time.Second
		var err error
		expected, err = projectNativeCLIArguments(f.spec.client, f.client, f.view, "Describe only the installed "+f.name+" skill marker. Do not run any tool or change any file.")
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unregistered native CLI command label")
	}
	if cwd != expectedCWD || timeout != expectedTimeout || !reflect.DeepEqual(argv, expected) {
		return fmt.Errorf("fixed native CLI command/cwd/deadline differs")
	}
	return nil
}
func (f *projectNativeCLIAttempt) command(t *testing.T, label, cwd string, argv []string, timeout time.Duration) ports.CommandResult {
	t.Helper()
	allowed := map[string]bool{"git-a": true, "git-b": true, "version": true, "add-a": true, "native-attempt": true}
	if !allowed[label] || f.used[label] {
		t.Fatal("unregistered/repeated CLI attempt command")
	}
	f.used[label] = true
	if err := f.selectedCommand(label, cwd, argv, timeout); err != nil {
		t.Fatal(err)
	}
	f.check(t)
	ctx, cancel := context.WithTimeout(f.ctx, timeout)
	defer cancel()
	start := time.Now()
	r, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: argv, Env: f.env(), Dir: cwd, StdoutLimitBytes: 1 << 20}, 5*time.Second)
	f.keep(t, label+".stdout.log", r.Stdout)
	f.keep(t, label+".stderr.log", r.Stderr)
	meta, _ := json.Marshal(map[string]any{"argv": argv, "cwd": cwd, "exit": r.ExitCode, "error": err != nil, "deadline": ctx.Err() != nil, "elapsed_ms": time.Since(start).Milliseconds(), "cleanup": "full descendant supervision; forced cleanup fails"})
	f.keep(t, label+".command.json", meta)
	f.check(t)
	if err != nil || ctx.Err() != nil || len(r.Stdout) > 1<<20 || bytes.Contains(r.Stderr, []byte("stderr bytes omitted")) || r.ExitCode < 0 || (label != "native-attempt" && r.ExitCode != 0) {
		t.Fatalf("%s child/capture/cleanup failed; see bounded evidence", label)
	}
	return r
}
func projectNativeCLIHosted(goos, arch, actions, environment, optin string) bool {
	return goos == "linux" && arch == "amd64" && actions == "true" && environment == "github-hosted" && optin == "1"
}
func newProjectNativeCLIAttempt(t *testing.T, spec projectNativeCLISpec) *projectNativeCLIAttempt {
	t.Helper()
	started := time.Now()
	if !projectNativeCLIHosted(runtime.GOOS, runtime.GOARCH, os.Getenv("GITHUB_ACTIONS"), os.Getenv("RUNNER_ENVIRONMENT"), os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED")) {
		t.Fatal("native CLI attempts require actual opted-in GitHub-hosted Linux amd64")
	}
	f := &projectNativeCLIAttempt{spec: spec, started: started, files: map[string][]byte{}, immutable: map[string]string{}, protected: map[string]string{}, used: map[string]bool{}, evidence: map[string]any{"client": spec.client, "version": spec.version, "runtime_attempt_complete": false, "activation": "prepared", "vendor_authentication_qualified": false, "real_model_qualified": false, "qualification": map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}}}
	f.ctx, f.cancel = context.WithDeadline(context.Background(), started.Add(150*time.Second))
	f.output = os.Getenv("AGENTPLUGINS_PROJECT_NATIVE_OUTPUT")
	if err := projectNativeRuntimeParents(filepath.Join(f.output, "leaf")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(f.output)
	if err != nil || len(entries) != 0 {
		t.Fatal("existing empty closed output directory required")
	}
	f.client = nativeBinary(t, spec.binEnv)
	f.installer = nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	f.git, err = exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	f.git, err = filepath.EvalSymlinks(f.git)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := projectNativeRuntimeRead(os.Getenv("AGENTPLUGINS_PROJECT_NATIVE_IDENTITY"), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectNativeRuntimeJSON(raw); err != nil {
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
		Node         json.RawMessage `json:"node"`
		RuntimeFiles []struct {
			Path string `json:"path"`
			SHA  string `json:"sha256"`
		} `json:"runtime_files"`
	}
	if json.Unmarshal(raw, &identity) != nil || identity.Client != spec.client || !identity.Acquisition.Pin || identity.Acquisition.SHA != spec.archiveSHA || len(identity.Node) != 0 || len(identity.Acquisition.Members) > 4096 || len(identity.RuntimeFiles) == 0 || len(identity.RuntimeFiles) > 4096 {
		t.Fatal("fixed CLI acquisition receipt differs")
	}
	suffix := filepath.FromSlash(spec.member)
	if !strings.HasSuffix(f.client, string(filepath.Separator)+suffix) {
		t.Fatal("exact native entrypoint suffix differs")
	}
	root := strings.TrimSuffix(f.client, suffix)
	expected := map[string]string{}
	main := false
	for _, member := range identity.Acquisition.Members {
		if member.Kind == "directory" {
			continue
		}
		if member.Kind != "file" || member.Name == "" || member.Name == "." || member.Name == ".." || strings.ContainsAny(member.Name, "\\:\x00") || filepath.ToSlash(filepath.Clean(member.Name)) != member.Name || filepath.IsAbs(member.Name) || strings.HasPrefix(member.Name, "../") || len(member.SHA) != 64 || member.Bytes < 0 || member.Bytes > 512<<20 {
			t.Fatal("native archive member refused")
		}
		path := filepath.Join(root, filepath.FromSlash(member.Name))
		if expected[path] != "" {
			t.Fatal("duplicate native archive member")
		}
		expected[path] = member.SHA
		if member.Name == spec.member {
			if member.SHA != spec.executableSHA || member.Bytes != spec.executableBytes {
				t.Fatal("exact native executable pin differs")
			}
			main = true
		}
	}
	if !main || len(expected) != len(identity.RuntimeFiles) {
		t.Fatal("complete regular runtime closure required")
	}
	for _, file := range identity.RuntimeFiles {
		if expected[file.Path] == "" || expected[file.Path] != file.SHA || f.immutable[file.Path] != "" {
			t.Fatal("runtime closure path/hash differs")
		}
		f.immutable[file.Path] = file.SHA
	}
	for _, path := range []string{f.installer, f.git, nativeBinary(t, "AGENTPLUGINS_NATIVE_PROBE_BIN"), nativeBinary(t, "AGENTPLUGINS_LINTAI_BIN")} {
		digest, err := projectNativeRuntimeDigest(path)
		if err != nil {
			t.Fatal(err)
		}
		f.immutable[path] = digest
	}
	source, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	f.evidence["source_identity"], f.evidence["acquisition"] = source, identity
	f.check(t)
	f.base = newNativeFixture(t, false)
	f.other = filepath.Join(f.base.Root, "project-b")
	f.outside = filepath.Join(f.base.Root, "outside")
	for _, path := range []string{f.other, f.outside} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	f.bin = filepath.Dir(f.client)
	if spec.client == "qoder" {
		f.bin = filepath.Join(f.base.Root, "native-bin")
		if err := os.Mkdir(f.bin, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(f.client, filepath.Join(f.bin, "qoder")); err != nil {
			t.Fatal(err)
		}
	}
	f.base.ClientBinDir = f.bin
	f.immutable[f.base.Probe] = nativeSHA(t, f.base.Probe)
	f.evidence["scanner"] = nativeProvisionScanner(t, f.base)
	marker := filepath.Join(f.base.Home, "original-user-marker.txt")
	projectNativeRuntimeWrite(t, marker, []byte("original independent user authority\n"), 0600)
	f.protected[marker] = nativeSHA(t, marker)
	if relative, body := projectNativeCLIUpdater(spec.client); relative != "" {
		base := f.base.Home
		if spec.client == "windsurf" {
			base = filepath.Join(f.base.Root, "xdg-config")
		}
		path := filepath.Join(base, filepath.FromSlash(relative))
		projectNativeRuntimeWrite(t, path, body, 0600)
		f.protected[path] = nativeSHA(t, path)
		f.evidence["fixture_authored_updater_config"] = path
	}
	return f
}
func (f *projectNativeCLIAttempt) setup(t *testing.T) {
	t.Helper()
	for _, project := range []struct{ label, path string }{{"a", f.base.Project}, {"b", f.other}} {
		r := f.command(t, "git-"+project.label, project.path, []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}, 30*time.Second)
		if len(r.Stdout) != 0 || len(r.Stderr) != 0 {
			t.Fatal("real Git project setup diagnostics")
		}
	}
	r := f.command(t, "version", f.outside, []string{f.client, "--version"}, 30*time.Second)
	if string(r.Stdout) != f.spec.versionOutput || len(r.Stderr) != 0 {
		t.Fatal("exact frozen native version differs")
	}
	f.name = "uap-" + f.spec.client + "-attempt"
	manifest, _ := json.Marshal(map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": f.name, "version": "1.0.0", "description": "Disposable native project startup observation"})
	projectNativeRuntimeWrite(t, filepath.Join(f.base.Package, "plugin.json"), manifest, 0600)
	skill := []byte("---\nname: " + f.name + "\ndescription: Harmless installed startup marker\n---\nUAP_NATIVE_INSTALLED_" + f.base.Nonce + "\n")
	projectNativeRuntimeWrite(t, filepath.Join(f.base.Package, "skills", f.name, "SKILL.md"), skill, 0600)
	if f.spec.mixed {
		probe, err := projectNativeRuntimeRead(f.base.Probe, 32<<20)
		if err != nil {
			t.Fatal(err)
		}
		projectNativeRuntimeWrite(t, filepath.Join(f.base.Package, "bin", "probe"), probe, 0700)
		server := map[string]any{"type": "stdio", "command": "./bin/probe", "args": []string{"Owned " + f.spec.client, "${PLUGIN_ROOT}/argument"}, "cwd": "${PLUGIN_ROOT}", "env": map[string]string{"UAP_TEST_ROOT": f.base.Root, "UAP_TEST_EVENTS": f.base.Events, "UAP_TEST_NONCE": f.base.Nonce, "UAP_TEST_REVISION": "a-one"}}
		mcp, _ := json.Marshal(map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": map[string]any{f.name: server}})
		projectNativeRuntimeWrite(t, filepath.Join(f.base.Package, "mcp.json"), mcp, 0600)
	}
	f.evidence["attempt_stage"] = "public-project-install"
	r = f.command(t, "add-a", f.base.Project, []string{f.installer, "add", f.base.Package, "--no-save", "--scope", "project", "--project-root", f.base.Project, "--target", f.spec.client, "--format", "json"}, 30*time.Second)
	if _, err := projectNativeRuntimeJSON(r.Stdout); err != nil {
		t.Fatal(err)
	}
	if err := nativeValidateInstallerJSON(r.Stdout); err != nil {
		t.Fatal(err)
	}
	raw, err := projectNativeRuntimeRead(filepath.Join(f.base.Project, ".uap", "state-v2.json"), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectNativeRuntimeJSON(raw); err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if json.Unmarshal(raw, &state) != nil || len(state.Installations) != 1 {
		t.Fatal("exact public project installation missing")
	}
	installation := state.Installations[0]
	if installation.DeclaredName != f.name || installation.Source.CanonicalSource != f.base.Package || installation.Source.SourceBindingID == "" || installation.Source.TreeDigest == "" || installation.Package.DeclaredName != f.name || installation.Package.Version != "1.0.0" || installation.Package.ManifestDigest == "" || len(installation.Clients) != 1 {
		t.Fatal("installed source/package/receipt identity differs")
	}
	for _, binding := range installation.Clients {
		if binding.ClientID != f.spec.client || binding.Scope != "project" || binding.Activation != domain.ActivationPrepared || binding.PhysicalArtifact == "" || binding.TargetLocator == "" {
			t.Fatal("Prepared project binding differs")
		}
		f.binding = binding
	}
	b := f.binding
	wantVerification := domain.VerificationInstalled
	if f.spec.client == "grok" {
		wantVerification = domain.VerificationPackageValid
	}
	if b.Verification != wantVerification {
		t.Fatal("exact Prepared binding verification differs")
	}
	profile := filepath.Join(f.base.Project, "."+f.spec.client)
	if f.spec.client == "windsurf" {
		profile = filepath.Join(f.base.Project, ".devin")
	}
	if f.spec.client == "qoder" {
		profile = filepath.Join(f.base.Project, ".uap", "managed", "clients", "qoder")
	}
	if b.NativeProfileRoot != profile {
		t.Fatal("original profile versus selected project differs")
	}
	active := filepath.Join(f.base.Project, ".uap", "managed", "clients", f.spec.client, b.PhysicalArtifact)
	if f.spec.client == "grok" {
		active = filepath.Join(profile, "plugins", b.PhysicalArtifact)
	}
	if b.TargetLocator != active {
		t.Fatal("exact receipt package locator differs")
	}
	f.skill = filepath.Join(profile, "skills", f.name, "SKILL.md")
	if f.spec.client == "qoder" {
		f.view = filepath.Join(active, ".agentplugins-qoder-native")
		f.skill = filepath.Join(f.view, "skills", f.name, "SKILL.md")
		if err := projectNativeCLIClosedView(f.view, f.name, skill); err != nil {
			t.Fatal(err)
		}
	}
	if f.spec.client == "grok" {
		f.skill = filepath.Join(active, "skills", f.name, "SKILL.md")
	}
	delivered, err := projectNativeRuntimeRead(f.skill, 1<<20)
	if err != nil || !bytes.Equal(delivered, skill) {
		t.Fatal("receipt-selected delivered skill differs from independent source")
	}
	f.protected[f.skill] = nativeSHA(t, f.skill)
	packageDigest, err := projectNativeCLIPackageDigest(f.ctx, active)
	if err != nil {
		t.Fatal(err)
	}
	expectedOwned := map[string]domain.NativeObjectOwnership{
		"managed_package_directory": {ObjectID: "package:" + f.spec.client + ":" + b.PhysicalArtifact, Kind: "managed_package_directory", LogicalName: f.name, Path: active, ManagedDigest: packageDigest},
	}
	if f.spec.mixed {
		kind := "kiro_global_skill_directory"
		id := "kiro-skill:" + f.name
		if f.spec.client == "windsurf" {
			kind = "devin_project_skill_directory"
			id = kind + ":" + f.name
		}
		digest := sha256.Sum256(append(append([]byte("f\x00SKILL.md\x00-\x00"), skill...), 0))
		expectedOwned[kind] = domain.NativeObjectOwnership{ObjectID: id, Kind: kind, LogicalName: f.name, Path: filepath.Dir(f.skill), ManagedDigest: "sha256:" + hex.EncodeToString(digest[:])}
	}
	f.evidence["installation_source"], f.evidence["binding"], f.evidence["installed_skill"] = installation.Source, b, f.skill
	if f.spec.mixed {
		data := installation.DataReceipts[b.DataReceiptID]
		if data.DataReceiptID != b.DataReceiptID || data.State != domain.DataReceiptOwned || data.Scope != "project" || data.PhysicalBackend != b.PhysicalArtifact || data.OwnershipDigest == "" {
			t.Fatal("exact owned project data receipt differs")
		}
		nativeRequireContained(t, filepath.Join(f.base.Project, ".uap", "plugin-data"), data.Locator)
		f.data = data.Locator
		config := filepath.Join(profile, "settings", "mcp.json")
		if f.spec.client == "windsurf" {
			config = filepath.Join(profile, "mcp_config.json")
		}
		raw, err := projectNativeRuntimeRead(config, 1<<20)
		if err != nil {
			t.Fatal(err)
		}
		value, err := projectNativeRuntimeJSON(raw)
		if err != nil {
			t.Fatal(err)
		}
		document, ok := value.(map[string]any)
		if !ok {
			t.Fatal("native MCP object missing")
		}
		entries, ok := document["mcpServers"].(map[string]any)
		if !ok {
			t.Fatal("native MCP collection missing")
		}
		entry, ok := entries[f.name].(map[string]any)
		if !ok {
			t.Fatal("exact installed native server missing")
		}
		expectedArgs := []any{"--internal-stdio-v1", active, f.data, active, "plugin", "--", "./bin/probe", "Owned " + f.spec.client, active + "/argument"}
		expectedEnv := map[string]any{"PLUGIN_ROOT": active, "PLUGIN_DATA": f.data, "UAP_TEST_ROOT": f.base.Root, "UAP_TEST_EVENTS": f.base.Events, "UAP_TEST_NONCE": f.base.Nonce, "UAP_TEST_REVISION": "a-one"}
		if err := projectNativeCLIMixedEntry(entry, filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName), expectedArgs, expectedEnv); err != nil {
			t.Fatal(err)
		}
		mcpKind, mcpID := "kiro_global_mcp_server", "kiro-mcp:"+f.name
		canonical, _ := json.Marshal(entry)
		sum := sha256.Sum256(canonical)
		mcpDigest := "sha256:" + hex.EncodeToString(sum[:])
		if f.spec.client == "windsurf" {
			mcpKind = "devin_project_mcp_server"
			mcpID = mcpKind + ":" + f.name
			args := make([]string, len(expectedArgs))
			for i, value := range expectedArgs {
				args[i] = value.(string)
			}
			env := map[string]string{}
			for key, value := range expectedEnv {
				env[key] = value.(string)
			}
			receipt, err := nativeconfig.DesiredReceipt(config, nativeconfig.CodecDevin, f.name, nativeconfig.Server{Type: "stdio", Command: entry["command"].(string), Args: args, Env: env, StdioValuesResolved: true, CWDResolved: true}, nativeconfig.Placeholders{})
			if err != nil {
				t.Fatal(err)
			}
			mcpDigest = receipt.Digest
		}
		expectedOwned[mcpKind] = domain.NativeObjectOwnership{ObjectID: mcpID, Kind: mcpKind, LogicalName: f.name, Path: config, ManagedDigest: mcpDigest}
		f.protected[config] = nativeSHA(t, config)
		f.evidence["installed_mcp"] = entry
		f.evidence["data_locator"] = f.data
		for _, path := range []string{filepath.Join(active, "bin", "probe"), filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)} {
			digest, err := projectNativeRuntimeDigest(path)
			if err != nil {
				t.Fatal(err)
			}
			f.immutable[path] = digest
		}
	}
	if err := projectNativeCLIOwnership(b.NativeObjects, expectedOwned); err != nil {
		t.Fatal(err)
	}
	f.check(t)
}
func projectNativeCLIMixedEntry(actual map[string]any, command string, args []any, env map[string]any) error {
	if projectNativeRuntimeKeys(actual, "command args env", "") != nil || actual["command"] != command || !reflect.DeepEqual(actual["args"], args) || !reflect.DeepEqual(actual["env"], env) {
		return fmt.Errorf("native stdio command/args/env differs or includes an unqualified transport/cwd/field")
	}
	return nil
}
func projectNativeCLIOwnership(actual []domain.NativeObjectOwnership, expected map[string]domain.NativeObjectOwnership) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("installed native ownership count differs")
	}
	seen := map[string]bool{}
	for _, object := range actual {
		want, ok := expected[object.Kind]
		if !ok || seen[object.Kind] || object.ObjectID != want.ObjectID || object.LogicalName != want.LogicalName || object.Path != want.Path || object.ManagedDigest == "" || object.ManagedDigest != want.ManagedDigest {
			return fmt.Errorf("installed native ownership identity/path/digest differs")
		}
		seen[object.Kind] = true
	}
	return nil
}
func projectNativeCLIPackageDigest(ctx context.Context, root string) (string, error) {
	if err := projectNativeRuntimeParents(filepath.Join(root, "leaf")); err != nil {
		return "", err
	}
	captured := []packagedigest.CapturedEntry{}
	var total int64
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		if count > 4096 {
			return fmt.Errorf("owned package entry bound")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if len(strings.Split(rel, string(filepath.Separator))) > 32 {
			return fmt.Errorf("owned package depth bound")
		}
		if entry.IsDir() {
			if err := projectNativeRuntimeParents(filepath.Join(path, "leaf")); err != nil {
				return err
			}
			if rel != "." {
				captured = append(captured, packagedigest.CapturedEntry{Path: filepath.ToSlash(rel), Kind: "directory"})
			}
			return nil
		}
		before, err := os.Lstat(path)
		if err != nil {
			return err
		}
		total += before.Size()
		if total > 256<<20 {
			return fmt.Errorf("owned package byte bound")
		}
		body, err := projectNativeRuntimeRead(path, 64<<20)
		if err != nil {
			return err
		}
		captured = append(captured, packagedigest.CapturedEntry{Path: filepath.ToSlash(rel), Kind: "file", Executable: before.Mode().Perm()&0111 != 0, Content: body})
		return nil
	})
	if err != nil {
		return "", err
	}
	return packagedigest.DigestCaptured(ctx, captured)
}
func projectNativeCLIClosedView(root, name string, skill []byte) error {
	expected := map[string]bool{".": true, ".qoder-plugin": true, ".qoder-plugin/plugin.json": true, "skills": true, "skills/" + name: true, "skills/" + name + "/SKILL.md": true}
	seen := map[string]bool{}
	if err := projectNativeRuntimeParents(filepath.Join(root, "leaf")); err != nil {
		return err
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !expected[rel] || seen[rel] {
			return fmt.Errorf("Qoder view contains an unqualified native path")
		}
		seen[rel] = true
		if entry.IsDir() {
			return projectNativeRuntimeParents(filepath.Join(path, "leaf"))
		}
		body, err := projectNativeRuntimeRead(path, 1<<20)
		if err != nil {
			return err
		}
		if rel == ".qoder-plugin/plugin.json" {
			value, err := projectNativeRuntimeJSON(body)
			if err != nil {
				return err
			}
			m, ok := value.(map[string]any)
			if !ok || projectNativeRuntimeKeys(m, "name version description skills", "") != nil || m["name"] != name || m["version"] != "1.0.0" || m["skills"] != "./skills/" || m["description"] != "Disposable native project startup observation" {
				return fmt.Errorf("Qoder view manifest differs from selected source")
			}
		} else if !bytes.Equal(body, skill) {
			return fmt.Errorf("Qoder selected view skill differs")
		}
		return nil
	})
	if err != nil || !reflect.DeepEqual(seen, expected) {
		return fmt.Errorf("closed receipt-selected Qoder native view failed: %v", err)
	}
	return nil
}
func (f *projectNativeCLIAttempt) finish(t *testing.T) {
	t.Helper()
	defer f.cancel()
	if time.Since(f.started) > 180*time.Second {
		t.Error("original CLI attempt total deadline exceeded")
	}
	f.evidence["test_failed"], f.evidence["elapsed_ms"] = t.Failed(), time.Since(f.started).Milliseconds()
	if f.base != nil {
		if body, err := projectNativeRuntimeRead(f.base.Events, 1<<20); err == nil {
			f.keep(t, "probe-events.jsonl", body)
		}
	}
	raw, _ := json.MarshalIndent(f.evidence, "", "  ")
	f.keep(t, "runtime-attempt.json", append(raw, '\n'))
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
func projectNativeCLIRuntimeAttempt(t *testing.T, client string) {
	t.Helper()
	spec, err := projectNativeCLISpecification(client)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv(spec.optin) != "1" {
		t.Skip("opt-in genuine installed native startup research")
	}
	f := newProjectNativeCLIAttempt(t, spec)
	defer f.finish(t)
	f.setup(t)
	prompt := "Describe only the installed " + f.name + " skill marker. Do not run any tool or change any file."
	argv, err := projectNativeCLIArguments(client, f.client, f.view, prompt)
	if err != nil {
		t.Fatal(err)
	}
	f.evidence["attempt_stage"] = "native-installed-startup"
	result := f.command(t, "native-attempt", f.base.Project, argv, 60*time.Second)
	f.evidence["native_exit"], f.evidence["native_output_captured"] = result.ExitCode, true
	// Even zero exit is not proof of plugin selection, permissions or model use.
	// Raw genuine output must freeze an exact supported protocol/auth boundary.
	t.Fatal("genuine installed native startup captured; exact trust/authentication/session boundary requires independent raw review; runtime remains unqualified")
}
