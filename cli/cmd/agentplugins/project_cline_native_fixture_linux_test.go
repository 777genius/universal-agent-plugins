//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

const clineGatePin = "1ff8841d17d4d83763134c193d414e4f7fcf2ef56af759e5ebbe96727fc2c8f7"
const clineGateStream = 128 << 10
const clineGateEnvelope = 32 << 20

type clineGateBuffer struct {
	mu       sync.Mutex
	body     []byte
	overflow bool
	cancel   context.CancelFunc
}

func (b *clineGateBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := min(len(p), clineGateStream-len(b.body))
	b.body = append(b.body, p[:n]...)
	if n != len(p) {
		b.overflow = true
		b.cancel()
		return n, fmt.Errorf("cline gate stream limit")
	}
	return n, nil
}
func (b *clineGateBuffer) snapshot() ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.body...), b.overflow
}

type clineGateCapture struct {
	out, stderr []byte
	exit        int
	err         error
	overflow    bool
	elapsed     time.Duration
	argv        []string
}

func captureClineGate(parent context.Context, executable, cwd string, env, args []string, timeout time.Duration) clineGateCapture {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	out, stderr := &clineGateBuffer{cancel: cancel}, &clineGateBuffer{cancel: cancel}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir, cmd.Env, cmd.Stdin = cwd, append([]string(nil), env...), nil
	cmd.Stdout, cmd.Stderr, cmd.WaitDelay = out, stderr, 250*time.Millisecond
	start := time.Now()
	err := cmd.Run()
	result := clineGateCapture{exit: -1, err: err, elapsed: time.Since(start), argv: append([]string{executable}, args...)}
	var a, b bool
	result.out, a = out.snapshot()
	result.stderr, b = stderr.snapshot()
	result.overflow = a || b
	if cmd.ProcessState != nil {
		result.exit = cmd.ProcessState.ExitCode()
	}
	if ctx.Err() != nil {
		result.err = fmt.Errorf("child deadline/cancellation: %w", ctx.Err())
	}
	return result
}

type clineGate struct {
	t                                             *testing.T
	ctx                                           context.Context
	cancel                                        context.CancelFunc
	root, home, user, A, B, cache, binary, output string
	journey                                       projectJourney
	runner                                        *projectClineNoNative
	files                                         map[string][]byte
	bytes                                         int
	commands                                      []map[string]any
	used                                          map[string]bool
	stages                                        map[string]map[string]string
	evidence                                      map[string]any
	current                                       string
	rootIdentity, outputIdentity                  os.FileInfo
}

func requireClineGate(t *testing.T) {
	t.Helper()
	if os.Getenv("AGENTPLUGINS_CLINE_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("opt-in current-source Cline read-only project config gate")
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || os.Getuid() <= 0 || os.Getuid() != os.Geteuid() || os.Getgid() <= 0 || os.Getgid() != os.Getegid() {
		t.Fatal("original nonroot Linux amd64 identity required")
	}
	for key, want := range map[string]string{"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1"} {
		if os.Getenv(key) != want {
			t.Fatal("genuine disposable hosted admission required")
		}
	}
}
func newClineGate(t *testing.T) *clineGate {
	t.Helper()
	parent := os.Getenv("AGENTPLUGINS_CLINE_FIXTURE_BASE")
	if parent == "" {
		parent = t.TempDir()
	}
	root, err := os.MkdirTemp(parent, "uap-cline-project-")
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	f := &clineGate{t: t, root: root, files: map[string][]byte{}, used: map[string]bool{}, stages: map[string]map[string]string{}}
	f.rootIdentity, err = os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}
	work := 360
	if raw := os.Getenv("AGENTPLUGINS_CLINE_WORK_SECONDS"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 || parsed > 360 {
			t.Fatal("bounded Cline work deadline required")
		}
		work = parsed
	}
	f.ctx, f.cancel = context.WithTimeout(context.Background(), time.Duration(work)*time.Second)
	f.home, f.user, f.A, f.B = filepath.Join(root, "home"), filepath.Join(root, "user-cwd"), filepath.Join(root, "A"), filepath.Join(root, "B")
	f.cache = filepath.Join(root, "installer-state")
	f.binary, f.output = os.Getenv("AGENTPLUGINS_CLINE_BIN"), os.Getenv("AGENTPLUGINS_CLINE_EVIDENCE")
	if f.output != "" {
		resolved, e := filepath.EvalSymlinks(f.output)
		if e != nil || resolved != f.output {
			t.Fatal("exact evidence output required")
		}
		f.outputIdentity, err = os.Lstat(f.output)
		if err != nil || !f.outputIdentity.IsDir() {
			t.Fatal("ordinary evidence output required")
		}
	}
	for _, p := range []string{f.home, f.user, f.A, f.B, f.cache, filepath.Join(root, "tmp"), filepath.Join(root, "sources")} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"HOME", "USERPROFILE"} {
		t.Setenv(key, f.home)
	}
	for key, sub := range map[string]string{"XDG_CONFIG_HOME": "config", "XDG_CACHE_HOME": "cache", "XDG_DATA_HOME": "data", "XDG_STATE_HOME": "state"} {
		t.Setenv(key, filepath.Join(f.home, sub))
	}
	t.Setenv("PATH", "/usr/bin:/bin")
	for _, key := range []string{"CLINE_MCP_SETTINGS_PATH", "CLINE_DATA_DIR", "CLINE_DIR"} {
		t.Setenv(key, "")
	}
	t.Setenv("CLINE_NO_AUTO_UPDATE", "1")
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(f.home, f.cache, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	f.runner = &projectClineNoNative{}
	app = configureClineGate(t, app, filepath.Join(f.home, ".cline"), f.runner)
	f.journey = projectJourney{app: clineGateSecurity(app), home: f.home, cache: f.cache, a: f.A, b: f.B}
	f.evidence = map[string]any{"schema_version": 1, "client": "cline", "version": "3.0.73", "source_commit": os.Getenv("EXPECTED_COMMIT"), "source_tree": os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), "collection_complete": false,
		"original_uid": os.Getuid(), "original_gid": os.Getgid(), "task_home": f.home, "fixture_root": f.root,
		"source_composition": "public Cobra and production scoped installer/file transactions; explicit disposable client authority and inert security admission; not automatic IDE detection",
		"network":            "not isolated; only source-established read-only config dispatch; no connection-attempt or initialization claim"}
	for _, key := range []string{"native_project_activation_qualified", "native_mcp_initialization_qualified", "native_runtime_qualified", "native_model_use_qualified", "native_ide_qualified", "native_trust_qualified", "same_name_active_isolation_qualified", "native_skill_execution_qualified", "native_authentication_qualified", "native_session_policy_qualified"} {
		f.evidence[key] = false
	}
	for _, label := range []string{"source_install", "unique_discovery", "stable_lifecycle", "update", "missing_repair", "remove", "same_name_precedence", "foreign_preservation"} {
		f.stages[label] = map[string]string{"status": "not_evaluated", "reason": "not reached"}
	}
	return f
}
func configureClineGate(t *testing.T, app agentpluginscli.App, user string, runner *projectClineNoNative) agentpluginscli.App {
	t.Helper()
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{StateRoot: filepath.Dir(app.ManagedRoot), ManagedRoot: app.ManagedRoot, ProjectRoot: app.ProjectRoot, Registry: app.ClientRegistry, Runner: runner, EnableNativeObserver: true, HelperExecutable: helper})
	if err != nil {
		t.Fatal(err)
	}
	// Preserve the production kernel, stager and transition objects.
	activator, ok := app.Lifecycle.Activator.(providers.Activator)
	if !ok || activator.NativeConfig == nil {
		t.Fatal("production activator kernel missing")
	}
	observer, ok := app.Lifecycle.NativeObserver.(providers.NativeIdentityObserver)
	if !ok || observer.NativeConfig == nil {
		t.Fatal("production observer kernel missing")
	}
	activator.Runner, observer.Runner = runner, runner
	app.Installer, app.Detector = engine, projectClineDetector{user}
	app.Lifecycle.Activator, app.Lifecycle.NativeObserver = activator, observer
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return configureClineGate(t, selected, user, runner), nil
	}
	return app
}
func clineGateSecurity(app agentpluginscli.App) agentpluginscli.App {
	app.SecurityIndex, app.DirectoryClient, app.DiscoveryClient = nil, nil, nil
	app.SecurityEvaluator = projectJourneySecurity{}
	app.SecurityEvaluatorForDryRun = func(string) ports.PackageSecurityEvaluator { return projectJourneySecurity{} }
	compose := app.ForScope
	app.ForScope = func(scope domain.InstallScope, root string) (agentpluginscli.App, error) {
		selected, err := compose(scope, root)
		if err != nil {
			return selected, err
		}
		return clineGateSecurity(selected), nil
	}
	return app
}
func (f *clineGate) env(override string) []string {
	env := []string{"HOME=" + f.home, "USERPROFILE=" + f.home, "PATH=/usr/bin:/bin", "LANG=C.UTF-8", "TMPDIR=" + filepath.Join(f.root, "tmp"),
		"XDG_CONFIG_HOME=" + filepath.Join(f.home, "config"), "XDG_CACHE_HOME=" + filepath.Join(f.home, "cache"), "XDG_DATA_HOME=" + filepath.Join(f.home, "data"), "XDG_STATE_HOME=" + filepath.Join(f.home, "state"),
		"CLINE_NO_AUTO_UPDATE=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + filepath.Join(f.home, "git-config"), "GIT_TERMINAL_PROMPT=0"}
	if override != "" {
		if override != f.A && override != f.B {
			f.t.Fatal("unreviewed MCP override")
		}
		env = append(env, "CLINE_MCP_SETTINGS_PATH="+f.projectMCP(override))
	}
	return env
}
func (f *clineGate) keep(name string, body []byte) {
	f.t.Helper()
	_, exists := f.files[name]
	if filepath.Base(name) != name || name == "" || exists || len(f.files) >= 120 || len(body) > 1<<20 || f.bytes+len(body) > clineGateEnvelope-(1<<20) {
		f.t.Fatal("bounded evidence admission refused")
	}
	f.files[name] = append([]byte(nil), body...)
	f.bytes += len(body)
}
func clineGateSHA(body []byte) string { h := sha256.Sum256(body); return hex.EncodeToString(h[:]) }
func clineGateError(err error) string {
	if err == nil {
		return ""
	}
	if len(err.Error()) > 512 {
		return err.Error()[:512]
	}
	return err.Error()
}
func (f *clineGate) record(label, class, cwd string, argv []string, r clineGateCapture) {
	f.t.Helper()
	f.admit(label, class)
	f.used[label] = true
	f.keep(label+".stdout.log", r.out)
	f.keep(label+".stderr.log", r.stderr)
	if len(r.argv) != 0 {
		argv = r.argv
	}
	f.commands = append(f.commands, map[string]any{"label": label, "class": class, "in_process": class == "public_uap", "argv": argv, "cwd": cwd, "exit": r.exit, "error": clineGateError(r.err), "overflow": r.overflow, "elapsed_ms": r.elapsed.Milliseconds(),
		"stdout_file": label + ".stdout.log", "stderr_file": label + ".stderr.log", "stdout_bytes": len(r.out), "stderr_bytes": len(r.stderr), "stdout_sha256": clineGateSHA(r.out), "stderr_sha256": clineGateSHA(r.stderr)})
}
func (f *clineGate) uap(label, project string, args ...string) map[string]any {
	f.t.Helper()
	f.admit(label, "public_uap")
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()
	stdout, stderr := &clineGateBuffer{cancel: cancel}, &clineGateBuffer{cancel: cancel}
	app := f.journey.app
	app.Input, app.Output, app.ErrorOutput, app.Terminal = strings.NewReader(""), stdout, stderr, false
	root := agentpluginscli.NewRoot(app)
	root.SetOut(stdout)
	root.SetErr(stderr)
	args = append(args, "--target", "cline", "--format", "json")
	if project != "" {
		args = append(args, "--scope", "project", "--project-root", project)
	}
	root.SetArgs(args)
	start := time.Now()
	err := root.ExecuteContext(ctx)
	out, a := stdout.snapshot()
	diag, b := stderr.snapshot()
	r := clineGateCapture{out: out, stderr: diag, exit: 0, err: err, overflow: a || b, elapsed: time.Since(start)}
	if err != nil {
		r.exit = 1
	}
	f.record(label, "public_uap", project, args, r)
	if err != nil || r.overflow || !clineGateAcceptProgress(label, diag) || f.runner.calls != 0 {
		f.t.Fatalf("public UAP stage %s failed: error=%v stdout=%q stderr=%q", label, err, out, diag)
	}
	value := clineGateObject(f.t, out)
	if value["result"] != "success" {
		f.t.Fatal("public UAP did not report success")
	}
	if project != "" && args[0] != "remove" {
		f.assertPrepared(project, "")
	}
	return value
}
func (f *clineGate) binding(project, name string) domain.ClientBinding {
	f.t.Helper()
	for _, installation := range f.journey.state(f.t, project).Installations {
		if name != "" && installation.DeclaredName != name {
			continue
		}
		for _, binding := range installation.Clients {
			if binding.ClientID == "cline" && binding.Materialization != domain.MaterializationAbsent {
				return binding
			}
		}
	}
	f.t.Fatal("exact binding not found")
	return domain.ClientBinding{}
}
func (f *clineGate) assertPrepared(project, name string) {
	b := f.binding(project, name)
	if b.Scope != "project" || b.NativeProfileRoot != project || b.Activation != domain.ActivationPrepared || b.Verification != domain.VerificationInstalled || b.NativeActivationAttempt != "" || b.PendingNativeIntent != nil || b.PackageRevision == nil || b.PackageRevision.TreeDigest == "" || b.ClientBindingID == "" || len(b.NativeObjects) == 0 {
		f.t.Fatalf("project lifecycle/receipt not Prepared: %+v", b)
	}
}
func (f *clineGate) requireSnapshot(want map[string]string, roots ...string) {
	f.t.Helper()
	if got := f.snapshot(roots...); !reflect.DeepEqual(got, want) {
		f.t.Fatal("owned/foreign bytes changed across read-only or other-project stage")
	}
}
