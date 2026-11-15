package hermes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// Unit fixtures bind physical identities; the SDK suite separately exercises
// the actual durable standalone receipt around both native observations.
type sessionInventoryAuthority struct {
	project      string
	root, parent os.FileInfo
}

func (authority sessionInventoryAuthority) PublishProjectRoot(context.Context, domain.ActivationRequest, ports.NativeProjectRoot) error {
	return errors.New("native observation cannot publish a root")
}

func (authority sessionInventoryAuthority) VerifyProjectRoot(ctx context.Context, path string, owner domain.NativeObjectOwnership) error {
	if path != projectSessionRoot(authority.project) || owner != sessionRootObject(authority.project) {
		return errors.New("different fixture root")
	}
	root, err := os.Lstat(path)
	if err != nil || !os.SameFile(authority.root, root) {
		return errors.New("fixture root replaced")
	}
	parent, err := os.Lstat(filepath.Dir(path))
	if err != nil || !os.SameFile(authority.parent, parent) {
		return errors.New("fixture parent replaced")
	}
	return verifyProjectSessionRoot(ctx, authority.project, owner)
}

func sessionInventoryEnvironment(t *testing.T, plan clients.SessionPlan, runner *sessionInventoryRunner) clients.Env {
	t.Helper()
	root, err := os.Lstat(projectSessionRoot(plan.Directory))
	if err != nil {
		t.Fatal(err)
	}
	parent, err := os.Lstat(filepath.Dir(projectSessionRoot(plan.Directory)))
	if err != nil {
		t.Fatal(err)
	}
	return clients.Env{Runner: runner, ProjectRoots: sessionInventoryAuthority{plan.Directory, root, parent}}
}

type sessionInventoryRunner struct {
	plan         clients.SessionPlan
	mode         string
	calls        [][]string
	environments [][]string
	edit         func()
	editAt       int
}

func (runner *sessionInventoryRunner) verifyHome(environment []string) error {
	homes := 0
	for _, value := range environment {
		if strings.HasPrefix(strings.ToUpper(value), "HERMES_HOME=") {
			homes++
			if value != "HERMES_HOME="+projectSessionRoot(runner.plan.Directory) {
				return errors.New("wrong selected native home")
			}
		}
	}
	if homes != 1 {
		return errors.New("ambiguous selected native home")
	}
	return nil
}

func (runner *sessionInventoryRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls = append(runner.calls, append([]string(nil), command.Argv...))
	runner.environments = append(runner.environments, append([]string{}, command.Env...))
	if command.Dir != runner.plan.Directory || command.StdoutLimitBytes != 1<<20 {
		return legacyports.CommandResult{}, errors.New("incorrect observation boundary")
	}
	if err := runner.verifyHome(command.Env); err != nil {
		return legacyports.CommandResult{}, err
	}
	result := legacyports.CommandResult{CleanupComplete: true}
	if len(runner.calls) == 1 {
		result.Stdout = []byte("Hermes Agent vunknown (2026.9.24)\nInstall directory: " + filepath.Join(filepath.Dir(filepath.Dir(runner.plan.Executable)), "hermes-agent") + "\n")
	} else if len(runner.calls) == 2 {
		result.Stdout = []byte(filepath.Join(projectSessionRoot(runner.plan.Directory), "config.yaml") + "\n")
	} else {
		result.Stdout = []byte(`{"owned":{"url":"http://127.0.0.1:1234/mcp"}}`)
	}
	if runner.edit != nil && (len(runner.calls) == runner.editAt || runner.editAt == 0 && len(runner.calls) == 3) {
		runner.edit()
	}
	switch runner.mode {
	case "version":
		result.Stdout = []byte(strings.ReplaceAll(string(result.Stdout), "vunknown (2026.9.24)", "v999.0 (unqualified)"))
	case "missing-version":
		_, tail, _ := strings.Cut(string(result.Stdout), "\n")
		result.Stdout = []byte(tail)
	case "duplicate-version":
		result.Stdout = append(result.Stdout, []byte("Hermes Agent vunknown (2026.9.24)\n")...)
	case "crlf":
		result.Stdout = []byte(strings.ReplaceAll(string(result.Stdout), "\n", "\r\n"))
	case "path":
		result.Stdout = []byte("/foreign/config.yaml\n")
	case "cleanup":
		result.CleanupComplete = false
	case "no-child":
		result.NoChildStarted = true
	case "failed":
		result.ExitCode = 1
	case "bound":
		result.Stdout = make([]byte, 1<<20)
	case "mapping":
		if len(runner.calls) == 3 {
			result.Stdout = []byte(`{"owned":{"url":"http://foreign.test/mcp"}}`)
		}
	case "duplicate":
		if len(runner.calls) == 3 {
			result.Stdout = []byte(`{"owned":{},"owned":{}}`)
		}
	}
	return result, nil
}

func TestProjectSessionNativeInventoryUsesOfficialFiniteSelectedCommands(t *testing.T) {
	for _, mode := range []string{"lf", "crlf"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HERMES_CONFIG_BACKEND", " FILE ")
			req, owner := newProjectSessionFixture(t)
			plan, err := prepareProjectSession(t.Context(), req, owner)
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("HERMES_HOME", "/original-user-profile")
			runner := &sessionInventoryRunner{plan: plan, mode: mode}
			if err := verifySessionNativeSource(t.Context(), sessionInventoryEnvironment(t, plan, runner), plan); err != nil {
				t.Fatal(err)
			}
			want := [][]string{{plan.Executable, "--version"}, {plan.Executable, "config", "path"}, {plan.Executable, "config", "get", "mcp_servers", "--json"}}
			if !reflect.DeepEqual(runner.calls, want) {
				t.Fatalf("native observation started different commands: %v", runner.calls)
			}
		})
	}
}

func TestProjectSessionNativeInventoryRejectsForeignIncompleteAndChangedSource(t *testing.T) {
	for _, mode := range []string{"path", "cleanup", "no-child", "failed", "bound", "mapping", "duplicate", "changed", "version", "missing-version", "duplicate-version"} {
		t.Run(mode, func(t *testing.T) {
			req, owner := newProjectSessionFixture(t)
			plan, err := prepareProjectSession(t.Context(), req, owner)
			if err != nil {
				t.Fatal(err)
			}
			runner := &sessionInventoryRunner{plan: plan, mode: mode}
			if mode == "changed" {
				runner.edit = func() {
					path := filepath.Join(projectSessionRoot(req.ProjectRoot), "config.yaml")
					if err := os.WriteFile(path, []byte("mcp_servers: {owned: {url: 'http://foreign.test/mcp'}}\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := verifySessionNativeSource(t.Context(), sessionInventoryEnvironment(t, plan, runner), plan); err == nil {
				t.Fatal("native observation admitted an unproven selected source")
			}
			if len(runner.calls) > 3 {
				t.Fatal("native observation exceeded finite command budget")
			}
			if strings.Contains(mode, "version") && len(runner.calls) != 1 {
				t.Fatal("unqualified version reached another native command")
			}
		})
	}
}

func TestProjectSessionBadPrimaryOrSourceNeverStartsNativeDespiteBackup(t *testing.T) {
	for _, change := range []string{"missing", "malformed", "tagged", "plugins", "active-profile", "manifest-ref", "native-default", "backend", "managed", "bundle-env"} {
		t.Run(change, func(t *testing.T) {
			req, owner := newProjectSessionFixture(t, "hermes")
			sessionBootstrapFixture(t, req.ProjectRoot)
			plan, err := prepareProjectSession(t.Context(), req, owner)
			if err != nil {
				t.Fatal(err)
			}
			runner := &sessionInventoryRunner{plan: plan}
			env := sessionInventoryEnvironment(t, plan, runner)
			path := filepath.Join(projectSessionRoot(req.ProjectRoot), "config.yaml")
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "malformed":
				body = []byte("mcp_servers: [")
			case "tagged":
				body = []byte("mcp_servers: {owned: {url: !unqualified http://127.0.0.1:1234/mcp}}\n")
			case "plugins":
				body = append(body, []byte("plugins:\n  enabled: [foreign]\n")...)
			case "active-profile":
				path, body = filepath.Join(filepath.Dir(path), "active_profile"), []byte("foreign")
			case "manifest-ref":
				path = filepath.Join(filepath.Dir(filepath.Dir(plan.Executable)), "manifest.json")
				body, err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				body = []byte(strings.ReplaceAll(string(body), hermesBundleRef, strings.Repeat("0", 40)))
			case "native-default":
				t.Setenv("HERMES_DATA_DIR_SUFFIX", "")
				t.Setenv("HOME", req.ProjectRoot)
				if runtime.GOOS == "windows" {
					t.Setenv("LOCALAPPDATA", filepath.Dir(req.ProjectRoot))
				}
			case "backend":
				t.Setenv("HERMES_CONFIG_BACKEND", " remote ")
			case "managed":
				t.Setenv("HERMES_MANAGED_DIR", t.TempDir())
			case "bundle-env":
				path, body = filepath.Join(filepath.Dir(filepath.Dir(plan.Executable)), "hermes-agent", ".env"), []byte("HERMES_CONFIG_BACKEND=remote\n")
			}
			if change != "missing" && change != "native-default" && change != "backend" && change != "managed" {
				if err := os.WriteFile(path, body, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := New().PlanSession(t.Context(), env, req); err == nil || len(runner.calls) != 0 {
				t.Fatal("bad current source reached native planning", err)
			}
			if err := New().VerifySession(t.Context(), env, plan); err == nil || len(runner.calls) != 0 {
				t.Fatal("bad current source started a native child", err)
			}
		})
	}
}

func TestProjectSessionStopsBeforeNextChildWhenSourceChanges(t *testing.T) {
	for _, change := range []string{"primary-first", "primary-second", "bundle-env"} {
		t.Run(change, func(t *testing.T) {
			at := 1
			if change == "primary-second" {
				at = 2
			}
			req, owner := newProjectSessionFixture(t)
			sessionBootstrapFixture(t, req.ProjectRoot)
			plan, err := prepareProjectSession(t.Context(), req, owner)
			if err != nil {
				t.Fatal(err)
			}
			runner := &sessionInventoryRunner{plan: plan, editAt: at, edit: func() {
				path := filepath.Join(projectSessionRoot(req.ProjectRoot), "config.yaml")
				if change == "bundle-env" {
					path = filepath.Join(filepath.Dir(filepath.Dir(plan.Executable)), "hermes-agent", ".env")
				}
				if err := os.WriteFile(path, []byte("mcp_servers: ["), 0o600); err != nil {
					t.Fatal(err)
				}
			}}
			if err := New().VerifySession(t.Context(), sessionInventoryEnvironment(t, plan, runner), plan); err == nil || len(runner.calls) != at {
				t.Fatal("changed source reached another native command", runner.calls, err)
			}
		})
	}
}

func TestProjectSessionNativeInventoryKeepsOriginalInheritedEnvironment(t *testing.T) {
	for _, backend := range []string{"", " FILE "} {
		t.Run(backend, func(t *testing.T) {
			t.Setenv("HERMES_CONFIG_BACKEND", backend)
			req, owner := newProjectSessionFixture(t)
			plan, err := prepareProjectSession(t.Context(), req, owner)
			if err != nil {
				t.Fatal(err)
			}
			runner := &sessionInventoryRunner{plan: plan, editAt: 1, edit: func() {
				t.Setenv("HERMES_CONFIG_BACKEND", "remote")
			}}
			env := sessionInventoryEnvironment(t, plan, runner)
			env.SessionEnvironment = sessionEnvironment(nil)
			if err := New().VerifySession(t.Context(), env, plan); err != nil || len(runner.calls) != 3 {
				t.Fatal("original inherited source was not retained", err)
			}
			for _, actual := range runner.environments {
				found := 0
				for _, item := range actual {
					key, value, _ := strings.Cut(item, "=")
					if strings.EqualFold(key, "HERMES_CONFIG_BACKEND") {
						found++
						if value != backend {
							t.Fatal("observer adopted later ambient backend")
						}
					}
				}
				if found != 1 {
					t.Fatal("observer lost original supplied backend")
				}
			}
		})
	}
}

func TestProjectSessionRefusesLiteralHomeExpansionBeforeNative(t *testing.T) {
	names := []string{"$HOME"}
	if runtime.GOOS == "windows" {
		names = append(names, "%USERPROFILE%")
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			req, _ := newProjectSessionFixture(t, name)
			runner := &sessionInventoryRunner{}
			env := sessionInventoryEnvironment(t, clients.SessionPlan{Directory: req.ProjectRoot}, runner)
			if _, err := New().PlanSession(t.Context(), env, req); err == nil || len(runner.calls) != 0 {
				t.Fatal("native-expandable home reached child startup", err)
			}
		})
	}
}

func TestProjectSessionNativeInventoryCancellationStartsNoProcess(t *testing.T) {
	req, owner := newProjectSessionFixture(t)
	plan, err := prepareProjectSession(t.Context(), req, owner)
	if err != nil {
		t.Fatal(err)
	}
	runner := &sessionInventoryRunner{plan: plan}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := verifySessionNativeSource(canceled, sessionInventoryEnvironment(t, plan, runner), plan); !errors.Is(err, context.Canceled) || len(runner.calls) != 0 {
		t.Fatalf("pre-canceled observation started process: %v", err)
	}
}

func TestProjectSessionPassiveStateParentKeepsOriginalAuthority(t *testing.T) {
	f := newSessionHistoryFixture(t)
	parent := filepath.Join(f.state, "original-parent")
	selected := filepath.Join(parent, "state")
	if err := os.MkdirAll(selected, 0o700); err != nil {
		t.Fatal(err)
	}
	f.env.SessionEnvironment = clients.SessionPlan{Environment: map[string]string{"XDG_STATE_HOME": selected}}.ChildEnvironment(f.env.SessionEnvironment)
	bundle, err := qualifyHermesBundle(f.req.Client.ExecutablePath)
	if err != nil {
		t.Fatal(err)
	}
	state, err := captureSessionPassive(t.Context(), bundle, f.env.SessionEnvironment)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if state != nil {
			if err := state.close(); err != nil {
				t.Error(err)
			}
		}
	})
	original := state.directories[1]
	moved := parent + "-retained"
	if err := os.Rename(parent, moved); err != nil {
		if runtime.GOOS != "windows" || !errors.Is(err, syscall.Errno(32)) {
			t.Fatal(err)
		}
		if err := state.rejoin(t.Context()); err != nil {
			t.Fatal("denied Windows rename changed original authority", err)
		}
		if _, err := os.Lstat(moved); !os.IsNotExist(err) {
			t.Fatal("denied rename created its destination", err)
		}
		if err := state.close(); err != nil {
			t.Fatal(err)
		}
		state = nil
		if err := os.Rename(parent, moved); err != nil {
			t.Fatal("held parent was not released", err)
		}
		return
	}
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(moved, "state"), selected); err != nil {
		t.Fatal(err)
	}
	named, err := os.Stat(selected)
	if err != nil || !os.SameFile(original.info, named) {
		t.Fatal("fixture did not retain original state directory", err)
	}
	if err := state.rejoin(t.Context()); err == nil {
		t.Fatal("replacement parent was adopted around original missing component")
	}
}
