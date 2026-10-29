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
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const cursorProjectVersion = "2026.10.22-27dcf5f"
const cursorProjectArchive = "85296944e4600a83c4b764f578d1502bf9c416db200d33a593e8f3ecabfdc294"

type cursorProjectCommand struct {
	label, cwd string
	argv       []string
}
type cursorProjectHeld struct {
	info   os.FileInfo
	digest string
}
type cursorProjectAttempt struct {
	base                                           *nativeFixture
	client, installer, git, other, outside, output string
	ctx                                            context.Context
	cancel                                         context.CancelFunc
	started                                        time.Time
	files                                          map[string][]byte
	held                                           map[string]cursorProjectHeld
	runtimeRoot                                    string
	runtimeMembers                                 map[string]string
	absent                                         []string
	evidence                                       map[string]any
}

func (f *cursorProjectAttempt) commands() []cursorProjectCommand {
	a := f.base.Project
	return []cursorProjectCommand{
		{"git-a", a, []string{f.git, "init", "--quiet", "--initial-branch=main"}},
		{"git-b", f.other, []string{f.git, "init", "--quiet", "--initial-branch=main"}},
		{"version", f.outside, []string{f.client, "--version"}},
		{"empty-a-mcp", a, []string{f.client, "mcp", "list"}},
		{"empty-b-mcp", f.other, []string{f.client, "mcp", "list"}},
		{"empty-outside-mcp", f.outside, []string{f.client, "mcp", "list"}},
		{"add-a", a, []string{f.installer, "add", f.base.Package, "--no-save", "--scope", "project", "--project-root", a, "--target", "cursor", "--format", "json"}},
		{"native-attempt", a, []string{f.client, "mcp", "list"}},
		{"control-b-mcp", f.other, []string{f.client, "mcp", "list"}},
		{"control-outside-mcp", f.outside, []string{f.client, "mcp", "list"}},
	}
}
func (f *cursorProjectAttempt) hold(path string) error {
	digest, err := projectNativeRuntimeDigest(path)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	f.held[path] = cursorProjectHeld{info, digest}
	return nil
}
func (f *cursorProjectAttempt) check() error {
	if f.runtimeRoot != "" {
		if err := f.runtimeClosure(); err != nil {
			return err
		}
	}
	for path, want := range f.held {
		digest, err := projectNativeRuntimeDigest(path)
		info, statErr := os.Lstat(path)
		if err != nil || statErr != nil || digest != want.digest || !os.SameFile(info, want.info) || !projectNativeRuntimeStableStat(info.Sys(), want.info.Sys()) {
			return fmt.Errorf("Cursor held file identity changed: %s", path)
		}
	}
	for _, path := range f.absent {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return fmt.Errorf("Cursor other-source absence changed: %s", path)
		}
	}
	return nil
}
func (f *cursorProjectAttempt) keep(t *testing.T, name string, body []byte) {
	t.Helper()
	total := len(body)
	for _, previous := range f.files {
		total += len(previous)
	}
	_, exists := f.files[name]
	if filepath.Base(name) != name || exists || len(f.files) >= 32 || len(body) > 1<<20 || total > 16<<20 {
		t.Fatal("Cursor closed capture bound")
	}
	f.files[name] = body
}
func (f *cursorProjectAttempt) command(t *testing.T, command cursorProjectCommand) ports.CommandResult {
	t.Helper()
	if err := f.check(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(f.ctx, 30*time.Second)
	defer cancel()
	began := time.Now()
	env := append(nativePlatformEnvironment(f.base.Root, f.base.Home, filepath.Dir(f.client), filepath.Dir(f.git)), "AGENTPLUGINS_HOME="+filepath.Join(f.base.Root, "installer-state"), "NO_COLOR=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
	result, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: command.argv, Env: env, Dir: command.cwd, StdoutLimitBytes: 1 << 20}, 5*time.Second)
	receipt, _ := json.Marshal(map[string]any{"argv": command.argv, "cwd": command.cwd, "exit": result.ExitCode, "error": err != nil, "deadline": ctx.Err() != nil, "elapsed_ms": time.Since(began).Milliseconds(), "cleanup": "full descendant supervision; forced cleanup fails"})
	f.keep(t, command.label+".command.json", receipt)
	f.keep(t, command.label+".stdout.log", result.Stdout)
	f.keep(t, command.label+".stderr.log", result.Stderr)
	if err := f.check(); err != nil {
		t.Fatal(err)
	}
	if err != nil || ctx.Err() != nil || result.ExitCode != 0 || bytes.Contains(result.Stderr, []byte("stderr bytes omitted")) {
		t.Fatalf("Cursor %s process/capture/cleanup failed; see bounded capture", command.label)
	}
	return result
}
func newCursorProjectAttempt(t *testing.T) *cursorProjectAttempt {
	t.Helper()
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED") != "1" {
		t.Fatal("Cursor attempt requires actual opted-in disposable GitHub-hosted Linux amd64")
	}
	f := &cursorProjectAttempt{started: time.Now(), files: map[string][]byte{}, held: map[string]cursorProjectHeld{}, evidence: map[string]any{"client": "cursor", "version": cursorProjectVersion, "activation": "prepared", "runtime_attempt_complete": false, "vendor_authentication_qualified": false, "real_model_qualified": false, "qualification": map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}}}
	f.ctx, f.cancel = context.WithDeadline(context.Background(), f.started.Add(150*time.Second))
	f.output = os.Getenv("AGENTPLUGINS_PROJECT_NATIVE_OUTPUT")
	if err := projectNativeRuntimeParents(filepath.Join(f.output, "leaf")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(f.output)
	if err != nil || len(entries) != 0 {
		t.Fatal("Cursor output must be existing empty ordinary directory")
	}
	f.client = nativeBinary(t, "AGENTPLUGINS_CURSOR_BIN")
	f.installer = nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	f.git, err = exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	f.freezeAcquisition(t)
	for _, path := range []string{f.installer, f.git, nativeBinary(t, "AGENTPLUGINS_NATIVE_PROBE_BIN"), nativeBinary(t, "AGENTPLUGINS_LINTAI_BIN")} {
		if err := f.hold(path); err != nil {
			t.Fatal(err)
		}
	}
	source, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	f.evidence["source_identity"] = source
	f.base = newNativeFixture(t, false)
	f.other = filepath.Join(f.base.Root, "project-B")
	f.outside = filepath.Join(f.base.Root, "outside")
	for _, path := range []string{f.other, f.outside} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	f.absent = []string{filepath.Join(f.base.Home, ".cursor", "mcp.json"), filepath.Join(f.other, ".cursor", "mcp.json"), filepath.Join(f.outside, ".cursor", "mcp.json"), filepath.Join(f.base.Root, ".cursor", "mcp.json")}
	f.evidence["other_source_absent"] = f.absent
	scanner := nativeProvisionScanner(t, f.base)
	f.evidence["scanner"] = scanner
	if err := f.hold(filepath.Join(f.base.Root, "installer-state", "security", "lintai", "0.1.3", scanner["platform"], "lintai")); err != nil {
		t.Fatal(err)
	}
	if err := f.hold(f.base.Probe); err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *cursorProjectAttempt) finish(t *testing.T) {
	t.Helper()
	defer f.cancel()
	if err := f.check(); err != nil {
		t.Error(err)
	}
	if time.Since(f.started) > 180*time.Second {
		t.Error("Cursor total attempt budget exceeded")
	}
	events, err := projectNativeRuntimeRead(f.base.Events, 1<<20)
	if os.IsNotExist(err) {
		events = []byte{}
	} else if err != nil {
		t.Error(err)
	}
	f.keep(t, "probe-events.jsonl", events)
	f.evidence["test_failed"] = t.Failed()
	f.evidence["elapsed_ms"] = time.Since(f.started).Milliseconds()
	body, err := json.MarshalIndent(f.evidence, "", "  ")
	if err != nil {
		t.Error(err)
		return
	}
	f.keep(t, "runtime-attempt.json", append(body, '\n'))
	file, err := os.OpenFile(filepath.Join(f.output, "research.zip"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
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
		if _, err := writer.Write(f.files[name]); err != nil {
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
func TestAgentpluginsCursorNativeProjectRuntimeAttempt(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CURSOR_PROJECT_RUNTIME_ATTEMPT") != "1" {
		t.Skip("opt-in current-source Cursor component capture")
	}
	f := newCursorProjectAttempt(t)
	defer f.finish(t)
	f.packageSource(t)
	for _, command := range f.commands() {
		result := f.command(t, command)
		if command.label == "version" && (string(result.Stdout) != cursorProjectVersion+"\n" || len(result.Stderr) != 0) {
			t.Fatal("exact Cursor pinned version differs")
		}
		if command.label == "add-a" {
			f.installed(t, result.Stdout)
		}
	}
	t.Fatal("Cursor first installed MCP capture requires independent raw review; all runtime qualifications remain false")
}
