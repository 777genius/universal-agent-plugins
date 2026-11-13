package hermes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	plan  clients.SessionPlan
	mode  string
	calls [][]string
	edit  func()
}

func (runner *sessionInventoryRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	runner.calls = append(runner.calls, append([]string(nil), command.Argv...))
	if command.Dir != runner.plan.Directory || command.StdoutLimitBytes != 1<<20 {
		return legacyports.CommandResult{}, errors.New("incorrect observation boundary")
	}
	homes := 0
	for _, value := range command.Env {
		if strings.HasPrefix(strings.ToUpper(value), "HERMES_HOME=") {
			homes++
			if value != "HERMES_HOME="+projectSessionRoot(runner.plan.Directory) {
				return legacyports.CommandResult{}, errors.New("wrong selected native home")
			}
		}
	}
	if homes != 1 {
		return legacyports.CommandResult{}, errors.New("ambiguous selected native home")
	}
	result := legacyports.CommandResult{CleanupComplete: true}
	if len(runner.calls) == 1 {
		result.Stdout = []byte(filepath.Join(projectSessionRoot(runner.plan.Directory), "config.yaml") + "\n")
	} else {
		result.Stdout = []byte(`{"owned":{"url":"http://127.0.0.1:1234/mcp"}}`)
		if runner.edit != nil {
			runner.edit()
		}
	}
	switch runner.mode {
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
		if len(runner.calls) == 2 {
			result.Stdout = []byte(`{"owned":{"url":"http://foreign.test/mcp"}}`)
		}
	case "duplicate":
		if len(runner.calls) == 2 {
			result.Stdout = []byte(`{"owned":{},"owned":{}}`)
		}
	}
	return result, nil
}

func TestProjectSessionNativeInventoryUsesOfficialFiniteSelectedCommands(t *testing.T) {
	req, owner := newProjectSessionFixture(t)
	plan, err := prepareProjectSession(t.Context(), req, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERMES_HOME", "/original-user-profile")
	runner := &sessionInventoryRunner{plan: plan}
	if err := verifySessionNativeSource(t.Context(), sessionInventoryEnvironment(t, plan, runner), plan); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{plan.Executable, "config", "path"}, {plan.Executable, "config", "get", "mcp_servers", "--json"}}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("native observation started different commands: %v", runner.calls)
	}
}

func TestProjectSessionNativeInventoryRejectsForeignIncompleteAndChangedSource(t *testing.T) {
	for _, mode := range []string{"path", "cleanup", "no-child", "failed", "bound", "mapping", "duplicate", "changed"} {
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
			if len(runner.calls) > 2 {
				t.Fatal("native observation exceeded finite command budget")
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
