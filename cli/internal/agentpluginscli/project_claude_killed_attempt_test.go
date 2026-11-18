package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (r *preparedClaudeCLIRunner) SurvivingCommands(context.Context) ([]legacyports.LiveProcess, error) {
	return r.surviving, r.censusErr
}

// commandOnlyRunner runs commands but cannot list the processes that survive.
type commandOnlyRunner struct{ runner *preparedClaudeCLIRunner }

func (r commandOnlyRunner) Run(ctx context.Context, cmd legacyports.Command) (legacyports.CommandResult, error) {
	return r.runner.Run(ctx, cmd)
}

// killedProjectClaudeCommand leaves the binding as a CLI killed during a native
// Claude command does: the attempt persisted without an intent and the
// command's effect landed or not. In process this is an interrupt whose
// process cleanup is unproven, which keeps the attempt the same way.
func killedProjectClaudeCommand(t *testing.T, command string, args []string, landed bool) (projectClaudeCLI, string) {
	t.Helper()
	fixture, _ := interruptedProjectClaudeCommand(t, command, &claudeCLIInterrupt{args: args, landed: landed, unclosed: true})
	attempt, _ := fixture.nativeAttempt(t)
	if attempt == "" {
		t.Fatal("the killed native command did not leave its attempt")
	}
	return fixture, attempt
}

func (fixture projectClaudeCLI) runProjectWithout(args ...string) (string, error) {
	args = append(args, "--target", "claude", "--scope", "project", "--project-root", fixture.project, "--format", "json")
	stdout, stderr, err := fixture.executeContext(context.Background(), false, "", args...)
	return stdout + stderr, err
}

// A killed run left every later command refused for good. Repair or remove
// with --yes now confirms the attempt from Claude's own state and continues;
// --dry-run previews the same check without writing.
func TestProjectClaudeKilledCommandIsConfirmedByRepairOrRemove(t *testing.T) {
	for _, tc := range []struct {
		name, command, confirm string
		args                   []string
		landed                 bool
	}{
		{"add/install-landed", "add", "repair", []string{"plugin", "install"}, true},
		{"add/install", "add", "repair", []string{"plugin", "install"}, false},
		{"update/uninstall-landed", "update", "repair", []string{"plugin", "uninstall"}, true},
		{"remove/uninstall-landed", "remove", "remove", []string{"plugin", "uninstall"}, true},
		{"remove/marketplace-remove-landed", "remove", "remove", []string{"plugin", "marketplace", "remove"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture, attempt := killedProjectClaudeCommand(t, tc.command, tc.args, tc.landed)
			before, err := os.ReadFile(fixture.store.Path)
			if err != nil {
				t.Fatal(err)
			}
			out, err := fixture.runProjectWithout(tc.confirm, "demo", "--dry-run")
			after, _ := os.ReadFile(fixture.store.Path)
			if err != nil || !strings.Contains(out, `"status":"confirmable"`) || !strings.Contains(out, attempt) || string(after) != string(before) {
				t.Fatalf("dry run did not preview the confirmation without writing: %v\n%s", err, out)
			}
			if out, err := fixture.runProject(context.Background(), tc.confirm, "demo"); err != nil {
				t.Fatalf("%s did not confirm the killed attempt: %v\n%s", tc.confirm, err, out)
			}
			current, bound := fixture.nativeAttempt(t)
			listed := len(fixture.runner.marketplaceListing())
			if tc.confirm == "remove" && (bound || listed != 0 || fixture.knownMarketplaces(t) != 0) ||
				tc.confirm == "repair" && (!bound || current != "" || listed != 1) {
				t.Fatalf("%s did not converge: bound=%v attempt=%q listed=%d", tc.confirm, bound, current, listed)
			}
			doctor, _, err := fixture.execute(false, "doctor", "--target", "claude", "--scope", "project", "--project-root", fixture.project, "--format", "json")
			if err != nil || !strings.Contains(doctor, `"open_operation_count":0`) {
				t.Fatalf("doctor still reports an open operation: %v\n%s", err, doctor)
			}
		})
	}
}

// The confirmation must never close an attempt whose effect may still change:
// a surviving Claude command, a process listing that failed or is missing, an
// unrecognized client state or a missing --yes keep it, and doctor names the
// confirming commands.
func TestProjectClaudeKilledCommandConfirmationKeepsUncertainAttempt(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		prepare    func(*testing.T, *projectClaudeCLI)
		args       []string
	}{
		{"surviving-plugin-command", "pid 4242", func(_ *testing.T, f *projectClaudeCLI) {
			f.runner.surviving = []legacyports.LiveProcess{{PID: 4242, Argv: []string{"/bin/sh", f.client.ExecutablePath, "plugin", "install", "demo@x", "--scope", "project"}}}
		}, []string{"repair", "demo", "--yes"}},
		{"surviving-profile-process", "pid 4243", func(_ *testing.T, f *projectClaudeCLI) {
			f.runner.surviving = []legacyports.LiveProcess{{PID: 4243, Executable: "claude", Argv: []string{"claude"}, Env: []string{"CLAUDE_CONFIG_DIR=" + f.client.ConfigRoot}}}
		}, []string{"remove", "demo", "--dry-run"}},
		{"unrecognized-cache", "does not match the recorded ownership", func(t *testing.T, f *projectClaudeCLI) {
			caches, err := filepath.Glob(filepath.Join(f.client.ConfigRoot, "plugins", "cache", "*", "demo", "*"))
			if err != nil || len(caches) != 1 {
				t.Fatalf("fixture cache: %v %v", caches, err)
			}
			dependencyWrite(t, filepath.Join(caches[0], "foreign.txt"), "foreign\n")
		}, []string{"repair", "demo", "--yes"}},
		{"confirmation-required", "must be confirmed first; rerun with --yes", func(*testing.T, *projectClaudeCLI) {}, []string{"repair", "demo"}},
		{"process-listing-failed", "cannot verify that no Claude Code command", func(_ *testing.T, f *projectClaudeCLI) {
			f.runner.censusErr = errors.New("process table unavailable")
		}, []string{"repair", "demo", "--yes"}},
		{"runner-without-process-listing", "cannot verify that no Claude Code command", func(_ *testing.T, f *projectClaudeCLI) {
			f.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{Runner: commandOnlyRunner{f.runner}})
		}, []string{"remove", "demo", "--yes"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture, attempt := killedProjectClaudeCommand(t, "add", []string{"plugin", "install"}, true)
			tc.prepare(t, &fixture)
			out, err := fixture.runProjectWithout(tc.args...)
			if err == nil || !strings.Contains(out+err.Error(), tc.want) {
				t.Fatalf("uncertain attempt was not refused with %q: %v\n%s", tc.want, err, out)
			}
			if current, _ := fixture.nativeAttempt(t); current != attempt {
				t.Fatalf("refused confirmation changed the attempt %q to %q", attempt, current)
			}
			doctor, _, _ := fixture.execute(false, "doctor", "--target", "claude", "--scope", "project", "--project-root", fixture.project, "--format", "json")
			if !strings.Contains(doctor, "agentplugins repair") || !strings.Contains(doctor, "agentplugins remove") {
				t.Fatalf("doctor does not name the confirming commands:\n%s", doctor)
			}
		})
	}
}

// The confirmation writes only under the mutation lock, and it records the
// attempt closed with the binding failed: it never claims activation.
func TestProjectClaudeKilledCommandConfirmationHoldsLockAndLeavesBindingFailed(t *testing.T) {
	fixture, attempt := killedProjectClaudeCommand(t, "add", []string{"plugin", "install"}, true)
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	service := lifecycleService(fixture.app, map[domain.ClientID]domain.DetectedClient{domain.ClientClaude: fixture.client})
	input := usecase.InterruptedAttemptInput{InstallationID: state.Installations[0].InstallationID, Client: fixture.client, Scope: domain.ScopeProject,
		BackendExecutable: fixture.client.ExecutablePath, Confirmed: true}
	release, err := fixture.app.Lifecycle.Lock.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmInterruptedNativeAttempt(context.Background(), input); !errors.Is(err, processlock.ErrActive) {
		t.Fatalf("confirmation ignored the held mutation lock: %v", err)
	}
	if current, _ := fixture.nativeAttempt(t); current != attempt {
		t.Fatalf("confirmation under a held lock changed the attempt %q to %q", attempt, current)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmInterruptedNativeAttempt(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	state, err = fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.NativeActivationAttempt != "" || binding.Activation != domain.ActivationFailed || binding.Verification != domain.VerificationFailed {
			t.Fatalf("confirmed binding = attempt %q activation %q verification %q, want closed and failed", binding.NativeActivationAttempt, binding.Activation, binding.Verification)
		}
	}
}
