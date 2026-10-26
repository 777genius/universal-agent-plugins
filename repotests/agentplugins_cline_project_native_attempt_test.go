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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const clineAttemptIntegrity = "sha512-K+T94s0kPHLoMSANOJJ7KHb/etzYXZGQUbarKDKUPoZsvy4+oD1MAqYnZlfbKFM8tpnRQWouFngD9TB3mFqqeA=="
const clineAttemptExecutableSHA = "1ff8841d17d4d83763134c193d414e4f7fcf2ef56af759e5ebbe96727fc2c8f7"
const clineAttemptTool = "uapclinea__inspect_runtime"
const clineAttemptModel = "uap-cline-model"
const clineAttemptProviderID = "uap-scripted"

func clineAttemptSHA(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// This suite is a source-guided, initially unqualified native ACP attempt.
// It never substitutes desktop approval identity: the pinned CLI drops the
// first-turn request session ID before activeCliSession has been registered.
type clineProjectAttempt struct {
	base                                                                            *nativeFixture
	client, installer, git, output, other, outside, config, active, data, key, unit string
	ctx                                                                             context.Context
	cancel                                                                          context.CancelFunc
	origin, workEnd, totalEnd                                                       int64
	immutable, protected                                                            map[string]string
	trees                                                                           map[string]map[string]string
	files                                                                           map[string][]byte
	used                                                                            map[string]bool
	evidence                                                                        map[string]any
	provider                                                                        *clineProjectProvider
	nativeFacts                                                                     map[string]any
	authored                                                                        map[string][]byte
	sourceDigest                                                                    string
	nativeStart, nativeEnd                                                          time.Time
}

func clineAttemptRemaining(origin, now int64) (time.Duration, time.Duration, error) {
	if origin <= 0 || now < origin || now-origin >= int64(150*time.Second) {
		return 0, 0, fmt.Errorf("original monotonic work deadline exhausted or invalid")
	}
	return 150*time.Second - time.Duration(now-origin), 180*time.Second - time.Duration(now-origin), nil
}
func clineAttemptMonotonic() int64 {
	var ts unix.Timespec
	if unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts) != nil {
		return 0
	}
	return ts.Nano()
}
func clineAttemptUnit(unit, membership string, uid, gid, expectedUID, expectedGID int) error {
	if !regexp.MustCompile(`^uap-cline-project-[0-9a-f]{32}\.service$`).MatchString(unit) || membership != "0::/system.slice/"+unit+"\n" || uid <= 0 || gid <= 0 || uid != expectedUID || gid != expectedGID {
		return fmt.Errorf("actual ordinary delegated service identity differs")
	}
	return nil
}
func (f *clineProjectAttempt) keep(t *testing.T, name string, body []byte) {
	t.Helper()
	if f.key != "" {
		body = bytes.ReplaceAll(body, []byte(f.key), []byte("[REDACTED_FIXTURE_KEY]"))
	}
	total := len(body)
	for _, b := range f.files {
		total += len(b)
	}
	if filepath.Base(name) != name || f.files[name] != nil || len(f.files) >= 64 || len(body) > 1<<20 || total > 12<<20 {
		t.Fatal("Cline closed evidence bound")
	}
	f.files[name] = bytes.Clone(body)
}
func clineAttemptTree(root string) (map[string]string, error) {
	if err := projectNativeRuntimeParents(filepath.Join(root, "leaf")); err != nil {
		return nil, err
	}
	result := map[string]string{}
	var total int64
	err := filepath.WalkDir(root, func(path string, e os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || len(strings.Split(rel, string(filepath.Separator))) > 32 || len(result) >= 4096 {
			return fmt.Errorf("Cline owned tree bound")
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if e.IsDir() {
			if err := projectNativeRuntimeParents(filepath.Join(path, "leaf")); err != nil {
				return err
			}
			result[rel] = fmt.Sprintf("directory:%o", info.Mode())
			return nil
		}
		total += info.Size()
		if total > 256<<20 {
			return fmt.Errorf("Cline owned bytes bound")
		}
		body, err := projectNativeRuntimeRead(path, 64<<20)
		if err != nil {
			return err
		}
		result[rel] = fmt.Sprintf("file:%o:%s", info.Mode(), clineAttemptSHA(body))
		return nil
	})
	return result, err
}
func (f *clineProjectAttempt) check(t *testing.T) {
	t.Helper()
	for _, inventory := range []map[string]string{f.immutable, f.protected} {
		for path, want := range inventory {
			got, err := projectNativeRuntimeDigest(path)
			if err != nil || got != want {
				t.Fatalf("Cline held identity changed: %s (%v)", path, err)
			}
		}
	}
	for root, want := range f.trees {
		got, err := clineAttemptTree(root)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("Cline original owned tree changed: %s (%v)", root, err)
		}
	}
}
func (f *clineProjectAttempt) env(native bool) []string {
	env := append(f.base.env(filepath.Dir(f.client)), "NO_COLOR=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "CLINE_NO_AUTO_UPDATE=1")
	if native {
		env = append(env, "CLINE_MCP_SETTINGS_PATH="+f.config, "CLINE_PROVIDER="+clineAttemptProviderID, "CLINE_MODEL="+clineAttemptModel, "CLINE_API_KEY="+f.key)
	}
	return env
}
func (f *clineProjectAttempt) command(t *testing.T, label string) ports.CommandResult {
	t.Helper()
	cwd := f.base.Project
	var argv []string
	switch label {
	case "git-a", "git-b":
		argv = []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
		if label == "git-b" {
			cwd = f.other
		}
	case "version":
		cwd = f.outside
		argv = []string{f.client, "--auto-approve", "false", "--version"}
	case "add-a":
		argv = []string{f.installer, "add", f.base.Package, "--no-save", "--scope", "project", "--project-root", f.base.Project, "--target", "cline", "--format", "json"}
	default:
		t.Fatal("unregistered Cline public command")
	}
	if f.used[label] {
		t.Fatal("repeated Cline command")
	}
	f.used[label] = true
	f.check(t)
	ctx, cancel := context.WithTimeout(f.ctx, 30*time.Second)
	defer cancel()
	r, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: argv, Env: f.env(false), Dir: cwd, StdoutLimitBytes: 1 << 20}, 5*time.Second)
	f.keep(t, label+".stdout.log", r.Stdout)
	f.keep(t, label+".stderr.log", r.Stderr)
	meta, _ := json.Marshal(map[string]any{"argv": argv, "cwd": cwd, "exit": r.ExitCode, "error": err != nil, "deadline": ctx.Err() != nil})
	f.keep(t, label+".command.json", meta)
	f.check(t)
	if err != nil || ctx.Err() != nil || r.ExitCode != 0 || len(r.Stdout) > 1<<20 || bytes.Contains(r.Stderr, []byte("stderr bytes omitted")) {
		t.Fatalf("Cline %s child/capture/cleanup failed", label)
	}
	return r
}
func newClineProjectAttempt(t *testing.T) *clineProjectAttempt {
	t.Helper()
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED") != "1" {
		t.Fatal("Cline requires actual opted-in GitHub-hosted Linux amd64")
	}
	f := &clineProjectAttempt{immutable: map[string]string{}, protected: map[string]string{}, trees: map[string]map[string]string{}, files: map[string][]byte{}, used: map[string]bool{}, evidence: map[string]any{"client": "cline", "version": "3.0.73", "activation": "prepared", "runtime_attempt_complete": false, "vendor_authentication_qualified": false, "real_model_qualified": false, "qualification": map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}}}
	var err error
	f.origin, err = strconv.ParseInt(os.Getenv("AGENTPLUGINS_PROJECT_NATIVE_STARTED_NS"), 10, 64)
	if err != nil {
		t.Fatal("original monotonic frontend origin required")
	}
	work, total, err := clineAttemptRemaining(f.origin, clineAttemptMonotonic())
	if err != nil {
		t.Fatal(err)
	}
	f.workEnd = f.origin + int64(150*time.Second)
	f.totalEnd = f.origin + int64(180*time.Second)
	f.ctx, f.cancel = context.WithTimeout(context.Background(), work)
	f.output = os.Getenv("AGENTPLUGINS_PROJECT_NATIVE_OUTPUT")
	if err := projectNativeRuntimeParents(filepath.Join(f.output, "leaf")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(f.output)
	if err != nil || len(entries) != 0 {
		t.Fatal("existing empty Cline output directory required")
	}
	f.unit = os.Getenv("AGENTPLUGINS_CLINE_SYSTEMD_UNIT")
	uid, e1 := strconv.Atoi(os.Getenv("AGENTPLUGINS_CLINE_ORIGINAL_UID"))
	gid, e2 := strconv.Atoi(os.Getenv("AGENTPLUGINS_CLINE_ORIGINAL_GID"))
	membership, err := os.ReadFile("/proc/self/cgroup")
	if e1 != nil || e2 != nil || err != nil || clineAttemptUnit(f.unit, string(membership), os.Getuid(), os.Getgid(), uid, gid) != nil {
		t.Fatal("genuine original ordinary delegated Cline membership required")
	}
	status, err := os.ReadFile("/proc/self/status")
	if err != nil || !bytes.Contains(status, []byte("NoNewPrivs:\t1\n")) {
		t.Fatal("NoNewPrivileges must be active")
	}
	cgroup := filepath.Join("/sys/fs/cgroup/system.slice", f.unit)
	info, err := os.Lstat(cgroup)
	var fs syscall.Statfs_t
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(uid) || syscall.Statfs(cgroup, &fs) != nil || uint64(fs.Type) != 0x63677270 {
		t.Fatal("real original-user delegated cgroup2 required")
	}
	f.evidence["delegation"] = map[string]any{"unit": f.unit, "membership": string(membership), "uid": uid, "gid": gid, "no_new_privileges": true, "original_total_remaining_ms": total.Milliseconds()}
	f.client = nativeBinary(t, "AGENTPLUGINS_CLINE_BIN")
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
			Integrity string `json:"integrity"`
			Pin       bool   `json:"pin_verified"`
			SHA       string `json:"sha256"`
			Bytes     int64  `json:"bytes"`
			Members   []struct {
				Name, Kind string
				Bytes      int64
				SHA        string `json:"sha256"`
			}
		} `json:"acquisition"`
		Node         json.RawMessage `json:"node"`
		RuntimeFiles []struct {
			Path string `json:"path"`
			SHA  string `json:"sha256"`
		} `json:"runtime_files"`
	}
	if json.Unmarshal(raw, &identity) != nil || identity.Client != "cline" || !identity.Acquisition.Pin || identity.Acquisition.Integrity != clineAttemptIntegrity || identity.Acquisition.Bytes != 50817241 || len(identity.Acquisition.SHA) != 64 || len(identity.Node) != 0 || len(identity.Acquisition.Members) == 0 || len(identity.Acquisition.Members) > 4096 || len(identity.RuntimeFiles) == 0 || len(identity.RuntimeFiles) > 4096 {
		t.Fatal("exact pinned Cline acquisition required")
	}
	if !strings.HasSuffix(f.client, "/package/bin/cline") {
		t.Fatal("exact Cline native archive entrypoint required")
	}
	root := strings.TrimSuffix(f.client, "package/bin/cline")
	expected := map[string]string{}
	main := false
	for _, m := range identity.Acquisition.Members {
		if m.Kind == "directory" {
			continue
		}
		if m.Kind != "file" || m.Name == "" || m.Name == "." || m.Name == ".." || strings.ContainsAny(m.Name, "\\:\x00") || filepath.ToSlash(filepath.Clean(m.Name)) != m.Name || filepath.IsAbs(m.Name) || strings.HasPrefix(m.Name, "../") || m.Bytes < 0 || m.Bytes > 512<<20 || len(m.SHA) != 64 {
			t.Fatal("Cline archive member refused")
		}
		path := filepath.Join(root, filepath.FromSlash(m.Name))
		if expected[path] != "" {
			t.Fatal("duplicate Cline archive member")
		}
		expected[path] = m.SHA
		if m.Name == "package/bin/cline" {
			if m.SHA != clineAttemptExecutableSHA {
				t.Fatal("Cline executable pin differs")
			}
			main = true
		}
	}
	if !main || len(expected) != len(identity.RuntimeFiles) {
		t.Fatal("full Cline regular runtime closure required")
	}
	for _, r := range identity.RuntimeFiles {
		if expected[r.Path] == "" || expected[r.Path] != r.SHA || f.immutable[r.Path] != "" {
			t.Fatal("Cline runtime closure differs")
		}
		f.immutable[r.Path] = r.SHA
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
	if err := (process.OS{}).DuplexCapability(); err != nil {
		t.Fatalf("genuine existing strict duplex admission failed: %v", err)
	}
	f.check(t)
	f.base = newNativeFixture(t, false)
	f.other = filepath.Join(f.base.Root, "project-b")
	f.outside = filepath.Join(f.base.Root, "outside")
	for _, path := range []string{f.other, f.outside} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	f.key = "uap-cline-private-" + f.base.Nonce // Registered before any fixture/capture can contain it.
	f.immutable[f.base.Probe] = nativeSHA(t, f.base.Probe)
	f.evidence["scanner"] = nativeProvisionScanner(t, f.base)
	return f
}
func (f *clineProjectAttempt) setup(t *testing.T) {
	t.Helper()
	for _, label := range []string{"git-a", "git-b"} {
		r := f.command(t, label)
		if len(r.Stdout) != 0 || len(r.Stderr) != 0 {
			t.Fatal("Git setup diagnostics")
		}
	}
	r := f.command(t, "version")
	if string(r.Stdout) != "3.0.73\n" || len(r.Stderr) != 0 {
		t.Fatal("frozen exact Cline version differs")
	}
	manifest, _ := json.Marshal(map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": "uap-cline-a", "version": "1.0.0", "description": "Disposable Cline installed ACP probe"})
	projectNativeRuntimeWrite(t, filepath.Join(f.base.Package, "plugin.json"), manifest, 0600)
	skill := []byte("---\nname: uap-cline-a\ndescription: Harmless installed Cline marker\n---\nUAP_CLINE_SKILL_" + f.base.Nonce + "\n")
	projectNativeRuntimeWrite(t, filepath.Join(f.base.Package, "skills", "uap-cline-a", "SKILL.md"), skill, 0600)
	probe, err := projectNativeRuntimeRead(f.base.Probe, 32<<20)
	if err != nil {
		t.Fatal(err)
	}
	projectNativeRuntimeWrite(t, filepath.Join(f.base.Package, "bin", "probe"), probe, 0700)
	server := map[string]any{"type": "stdio", "command": "./bin/probe", "args": []string{"Owned Cline A", "${PLUGIN_ROOT}/argument"}, "cwd": "${PLUGIN_ROOT}", "env": map[string]string{"UAP_TEST_ROOT": f.base.Root, "UAP_TEST_EVENTS": f.base.Events, "UAP_TEST_NONCE": f.base.Nonce, "UAP_TEST_REVISION": "a-one"}}
	mcp, _ := json.Marshal(map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": map[string]any{"uapclinea": server}})
	projectNativeRuntimeWrite(t, filepath.Join(f.base.Package, "mcp.json"), mcp, 0600)
	f.authored = map[string][]byte{"plugin.json": bytes.Clone(manifest), "mcp.json": bytes.Clone(mcp), "skills/uap-cline-a/SKILL.md": bytes.Clone(skill), "bin/probe": bytes.Clone(probe)}
	frozen, err := clineAttemptTree(f.base.Package)
	if err != nil {
		t.Fatal(err)
	}
	f.trees[f.base.Package] = frozen
	sourceEntries, err := clineAttemptPayload(f.base.Package)
	if err != nil {
		t.Fatal(err)
	}
	f.sourceDigest, err = packagedigest.DigestCaptured(f.ctx, sourceEntries)
	if err != nil {
		t.Fatal(err)
	}
	userMCP := filepath.Join(f.base.Home, ".cline", "data", "settings", "cline_mcp_settings.json")
	projectNativeRuntimeWrite(t, userMCP, []byte("{\"mcpServers\":{\"foreign-user\":{\"command\":\"/never-run\",\"args\":[],\"disabled\":true}}}\n"), 0600)
	f.protected[userMCP] = nativeSHA(t, userMCP)
	for _, path := range []string{filepath.Join(f.base.Home, ".cline", "skills", "foreign-user", "SKILL.md"), filepath.Join(f.other, ".cline", "skills", "foreign-b", "SKILL.md")} {
		projectNativeRuntimeWrite(t, path, []byte("---\nname: independent-marker\ndescription: Independent original authority\n---\nFOREIGN_UNSELECTED\n"), 0600)
		f.protected[path] = nativeSHA(t, path)
	}
	f.evidence["attempt_stage"] = "public-project-install"
	r = f.command(t, "add-a")
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
		t.Fatal("single installed Cline receipt required")
	}
	i := state.Installations[0]
	if err := clineAttemptReceipt(i, f.base.Package, f.sourceDigest, f.authored["plugin.json"]); err != nil {
		t.Fatal(err)
	}
	var b domain.ClientBinding
	for _, selected := range i.Clients {
		b = selected
	}
	f.active = filepath.Join(f.base.Project, ".uap", "managed", "clients", "cline", b.PhysicalArtifact)
	if b.ClientID != "cline" || b.Scope != "project" || b.Activation != domain.ActivationPrepared || b.Verification != domain.VerificationInstalled || b.NativeProfileRoot != f.base.Project || b.PhysicalArtifact == "" || b.TargetLocator != f.active {
		t.Fatal("exact Prepared Cline project binding differs")
	}
	wantRevision := &domain.ClientPackageRevision{Version: "1.0.0", TreeDigest: f.sourceDigest, ManifestDigest: "sha256:" + clineAttemptSHA(f.authored["plugin.json"])}
	if b.PhysicalArtifact != "uap-cline-a-"+clineAttemptSHA([]byte(i.InstallationID))[:12] || b.ClientBindingID != "client_"+clineAttemptSHA([]byte(strings.Join([]string{i.InstallationID, "cline", "project", f.active}, "\x00")))[:24] || !reflect.DeepEqual(b.PackageRevision, wantRevision) {
		t.Fatal("Cline exact physical/client/revision identity differs")
	}
	d := i.DataReceipts[b.DataReceiptID]
	if b.DataReceiptID == "" || d.DataReceiptID != b.DataReceiptID || d.State != domain.DataReceiptOwned || d.Scope != "project" || d.PhysicalBackend != b.PhysicalArtifact || d.OwnershipDigest == "" {
		t.Fatal("Cline owned project data receipt differs")
	}
	nativeRequireContained(t, filepath.Join(f.base.Project, ".uap", "plugin-data"), d.Locator)
	f.data = d.Locator
	f.config = filepath.Join(f.base.Project, ".uap", "clients", "cline", "mcp.json")
	raw, err = projectNativeRuntimeRead(f.config, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	value, err := projectNativeRuntimeJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	doc, ok := value.(map[string]any)
	if !ok || projectNativeRuntimeKeys(doc, "mcpServers", "") != nil {
		t.Fatal("Cline exact native MCP source required")
	}
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok || len(servers) != 1 {
		t.Fatal("only installed A MCP may be selected")
	}
	entry, ok := servers["uapclinea"].(map[string]any)
	if !ok {
		t.Fatal("installed A MCP absent")
	}
	args := []any{"Owned Cline A", f.active + "/argument"}
	env := map[string]any{"PLUGIN_ROOT": f.active, "PLUGIN_DATA": f.data, "UAP_TEST_ROOT": f.base.Root, "UAP_TEST_EVENTS": f.base.Events, "UAP_TEST_NONCE": f.base.Nonce, "UAP_TEST_REVISION": "a-one"}
	if err := clineAttemptServer(entry, filepath.Join(f.active, "bin", "probe"), f.active, args, env); err != nil {
		t.Fatal(err)
	}
	delivered := filepath.Join(f.base.Project, ".cline", "skills", "uap-cline-a", "SKILL.md")
	body, err := projectNativeRuntimeRead(delivered, 1<<20)
	if err != nil || !bytes.Equal(body, skill) {
		t.Fatal("installed independent Cline skill differs")
	}
	for _, path := range []string{f.config, delivered, filepath.Join(f.base.Project, ".uap", "state-v2.json")} {
		f.protected[path] = nativeSHA(t, path)
	}
	projection := map[string]any{"servers": map[string]any{"uapclinea": entry["transport"]}}
	if err := clineAttemptInstalled(f.active, f.authored, f.immutable[f.installer], projection); err != nil {
		t.Fatal(err)
	}
	installedEntries, err := clineAttemptPayload(f.active)
	if err != nil {
		t.Fatal(err)
	}
	packageDigest, err := packagedigest.DigestCaptured(f.ctx, installedEntries)
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := json.Marshal(entry)
	mcpDigest := "sha256:" + clineAttemptSHA(append([]byte("agentplugins-native-mcp-v1\x00cline-mcpServers\x00uapclinea\x00"), canonical...))
	skillDigest := "sha256:" + clineAttemptSHA(append(append([]byte("f\x00SKILL.md\x00-\x00"), skill...), 0))
	expectedObjects := map[string]domain.NativeObjectOwnership{
		"managed_package_directory":     {ObjectID: "package:cline:" + b.PhysicalArtifact, Kind: "managed_package_directory", LogicalName: "uap-cline-a", Path: f.active, ManagedDigest: packageDigest, ProtectionClass: "managed"},
		"cline_project_skill_directory": {ObjectID: "cline_project_skill_directory:uap-cline-a", Kind: "cline_project_skill_directory", LogicalName: "uap-cline-a", Path: filepath.Dir(delivered), SourceRelative: "skills/uap-cline-a", ManagedDigest: skillDigest, ProtectionClass: "managed"},
		"cline_project_mcp_server":      {ObjectID: "cline_project_mcp_server:uapclinea", Kind: "cline_project_mcp_server", LogicalName: "uapclinea", Path: f.config, ManagedDigest: mcpDigest, ProtectionClass: "managed"},
	}
	if err := clineAttemptOwnership(b.NativeObjects, expectedObjects); err != nil {
		t.Fatal(err)
	}
	tree, err := clineAttemptTree(f.active)
	if err != nil {
		t.Fatal(err)
	}
	f.trees[f.active] = tree
	f.evidence["binding"], f.evidence["installation_source"], f.evidence["installed_mcp"], f.evidence["data_locator"] = b, i.Source, entry, f.data
	f.check(t)
}
func clineAttemptReceipt(i domain.Installation, source, tree string, manifest []byte) error {
	sourceID := "src_" + clineAttemptSHA([]byte(source + "\x00\x00"))[:24]
	wantSource := domain.SourceBinding{SourceBindingID: sourceID, RequestedSource: source, CanonicalSource: source, TreeDigest: tree}
	wantPackage := domain.PackageBinding{LoaderKind: domain.LoaderKindAgentPlugins, FormatID: domain.FormatIDAgentPluginsV1, SchemaURI: domain.PluginSchemaV1, DeclaredName: "uap-cline-a", Version: "1.0.0", ManifestDigest: "sha256:" + clineAttemptSHA(manifest), Inventory: domain.ComponentInventory{MCPPresent: true, MCPEnabled: true, MCPServers: []string{"uapclinea"}, Skills: []string{"uap-cline-a"}}}
	if i.InstallationID == "" || i.DeclaredName != "uap-cline-a" || !reflect.DeepEqual(i.Source, wantSource) || !reflect.DeepEqual(i.Package, wantPackage) || len(i.Clients) != 1 {
		return fmt.Errorf("Cline recorded source/tree/package/manifest identity differs from frozen authored bytes")
	}
	return nil
}
func clineAttemptOwnership(actual []domain.NativeObjectOwnership, expected map[string]domain.NativeObjectOwnership) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("Cline native ownership count differs")
	}
	seen := map[string]bool{}
	for _, object := range actual {
		want, ok := expected[object.Kind]
		if !ok || seen[object.Kind] || !reflect.DeepEqual(object, want) {
			return fmt.Errorf("Cline native object identity/path/content digest differs")
		}
		seen[object.Kind] = true
	}
	return nil
}
func clineAttemptPayload(root string) ([]packagedigest.CapturedEntry, error) {
	if _, err := clineAttemptTree(root); err != nil {
		return nil, err
	}
	captured := []packagedigest.CapturedEntry{}
	err := filepath.WalkDir(root, func(path string, e os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if e.IsDir() {
			captured = append(captured, packagedigest.CapturedEntry{Path: rel, Kind: "directory"})
			return nil
		}
		before, err := os.Lstat(path)
		if err != nil {
			return err
		}
		body, err := projectNativeRuntimeRead(path, 64<<20)
		if err != nil {
			return err
		}
		captured = append(captured, packagedigest.CapturedEntry{Path: rel, Kind: "file", Executable: before.Mode().Perm()&0111 != 0, Content: body})
		return nil
	})
	return captured, err
}
func clineAttemptInstalled(root string, source map[string][]byte, installerDigest string, projection map[string]any) error {
	entries, err := clineAttemptPayload(root)
	if err != nil {
		return err
	}
	files := map[string]bool{"plugin.json": false, "mcp.json": false, "skills/uap-cline-a/SKILL.md": false, "bin/probe": true, ".agentplugins-cline-native.json": false, "io.github.777genius.agentplugins/managed-stdio-v1/agentplugins": true, "io.github.777genius.agentplugins/managed-stdio-v1/metadata.json": false}
	directories := map[string]bool{}
	for path := range files {
		for parent := filepath.ToSlash(filepath.Dir(path)); parent != "."; parent = filepath.ToSlash(filepath.Dir(parent)) {
			directories[parent] = true
		}
	}
	seenFiles, seenDirs := map[string]bool{}, map[string]bool{}
	for _, entry := range entries {
		if entry.Kind == "directory" {
			if !directories[entry.Path] || seenDirs[entry.Path] {
				return fmt.Errorf("unrecorded installed Cline directory")
			}
			seenDirs[entry.Path] = true
			continue
		}
		executable, ok := files[entry.Path]
		if !ok || entry.Kind != "file" || seenFiles[entry.Path] || entry.Executable != executable {
			return fmt.Errorf("unrecorded installed Cline file/type/executable mode")
		}
		seenFiles[entry.Path] = true
		switch entry.Path {
		case "mcp.json":
			actual, err := projectNativeRuntimeJSON(entry.Content)
			if err != nil {
				return err
			}
			want, err := projectNativeRuntimeJSON(source[entry.Path])
			if err != nil || !reflect.DeepEqual(actual, want) {
				return fmt.Errorf("sanitized MCP differs from frozen authored server")
			}
		case ".agentplugins-cline-native.json":
			actual, err := projectNativeRuntimeJSON(entry.Content)
			if err != nil || !reflect.DeepEqual(actual, projection) {
				return fmt.Errorf("stored Cline projection differs from independently resolved native transport")
			}
		case "io.github.777genius.agentplugins/managed-stdio-v1/agentplugins":
			if clineAttemptSHA(entry.Content) != installerDigest {
				return fmt.Errorf("delivered launcher differs from exact public installer")
			}
		case "io.github.777genius.agentplugins/managed-stdio-v1/metadata.json":
			actual, err := projectNativeRuntimeJSON(entry.Content)
			want := map[string]any{"protocol": json.Number("1"), "sha256": installerDigest, "cliVersion": "0.1.0-development"}
			if err != nil || !reflect.DeepEqual(actual, want) {
				return fmt.Errorf("known source-build launcher metadata differs")
			}
		default:
			if !bytes.Equal(entry.Content, source[entry.Path]) {
				return fmt.Errorf("installed payload differs from frozen authored bytes: %s", entry.Path)
			}
		}
	}
	if len(seenFiles) != len(files) || len(seenDirs) != len(directories) {
		return fmt.Errorf("closed installed Cline payload missing member")
	}
	return nil
}
func clineAttemptServer(actual map[string]any, command, cwd string, args []any, env map[string]any) error {
	if projectNativeRuntimeKeys(actual, "transport", "") != nil {
		return fmt.Errorf("Cline exact nested native transport required")
	}
	transport, ok := actual["transport"].(map[string]any)
	if !ok || projectNativeRuntimeKeys(transport, "type command args env cwd", "") != nil || transport["type"] != "stdio" || transport["command"] != command || transport["cwd"] != cwd || !reflect.DeepEqual(transport["args"], args) || !reflect.DeepEqual(transport["env"], env) {
		return fmt.Errorf("Cline installed native stdio command/args/env/cwd differs or has extra approval fields")
	}
	return nil
}
func (f *clineProjectAttempt) finish(t *testing.T) {
	t.Helper()
	defer f.cancel()
	if f.provider != nil {
		f.provider.close(t)
		f.provider.capture(t, f)
	}
	if clineAttemptMonotonic() > f.totalEnd {
		t.Error("original Cline total deadline exhausted")
	}
	f.evidence["test_failed"], f.evidence["elapsed_ms"] = t.Failed(), time.Duration(clineAttemptMonotonic()-f.origin).Milliseconds()
	if f.base != nil {
		if raw, err := projectNativeRuntimeRead(f.base.Events, 1<<20); err == nil {
			f.keep(t, "probe-events.jsonl", raw)
		}
	}
	raw, _ := json.MarshalIndent(f.evidence, "", "  ")
	f.keep(t, "runtime-attempt.json", append(raw, '\n'))
	file, err := os.OpenFile(filepath.Join(f.output, "research.zip"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Error(err)
		return
	}
	z := zip.NewWriter(file)
	names := []string{}
	for name := range f.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := z.Create(name)
		if err != nil {
			t.Error(err)
			break
		}
		if _, err = w.Write(f.files[name]); err != nil {
			t.Error(err)
			break
		}
	}
	if err := z.Close(); err != nil {
		t.Error(err)
	}
	if err := file.Close(); err != nil {
		t.Error(err)
	}
}
func TestAgentpluginsClineNativeProjectRuntimeAttempt(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CLINE_PROJECT_RUNTIME_ATTEMPT") != "1" {
		t.Skip("opt-in genuine installed Cline ACP runtime attempt")
	}
	f := newClineProjectAttempt(t)
	defer f.finish(t)
	f.setup(t)
	f.provider = newClineProjectProvider(t, f)
	f.provider.configure(t, f)
	f.evidence["attempt_stage"] = "native-installed-acp"
	f.check(t)
	var transcript bytes.Buffer
	argv := []string{f.client, "--auto-approve", "false", "--cwd", f.base.Project, "--acp"}
	start := time.Now()
	f.nativeStart = start
	err := (process.OS{}).RunDuplexWithPlannedShutdown(f.ctx, ports.Command{Argv: argv, Env: f.env(true), Dir: f.base.Project, StdoutLimitBytes: 1 << 20}, func(w io.Writer, r io.Reader) error { return f.acp(w, r, &transcript) })
	f.nativeEnd = time.Now()
	f.evidence["native_interval"] = map[string]string{"started_at": f.nativeStart.UTC().Format(time.RFC3339Nano), "ended_at": f.nativeEnd.UTC().Format(time.RFC3339Nano)}
	f.keep(t, "native-acp.jsonl", transcript.Bytes())
	meta, _ := json.Marshal(map[string]any{"argv": argv, "cwd": f.base.Project, "mcp_settings_override": f.config, "auto_approve": false, "injected_mcp_servers": []any{}, "error": err != nil, "elapsed_ms": time.Since(start).Milliseconds(), "cleanup": "existing OS strict planned duplex shutdown; entire delegated descendant cgroup", "stderr_contract": "existing OS returns bounded 32KiB diagnostic on error; successful planned shutdown exposes no diagnostic stream"})
	f.keep(t, "native-acp.command.json", meta)
	if err != nil {
		f.keep(t, "native-acp.error.log", []byte(err.Error()))
	}
	f.check(t)
	if err != nil || f.ctx.Err() != nil {
		t.Fatal("Cline ACP exchange/provider/cleanup failed; see bounded evidence")
	}
	if err := f.provider.verify(f); err != nil {
		t.Fatal(err)
	}
	f.evidence["native_exchange_observed"] = true
	t.Fatal("source-guided genuine Cline ACP attempt captured; independently review native/provider/probe bytes before freezing success grammar; runtime remains unqualified")
}
