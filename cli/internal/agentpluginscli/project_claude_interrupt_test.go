package agentpluginscli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// claudeCLIInterrupt models Ctrl+C or SIGTERM arriving while one native Claude
// command runs: the command context is canceled and the child is stopped
// either before its effect or just after the effect already landed.
type claudeCLIInterrupt struct {
	args []string
	// after, when set, arms the interrupt only once a command with this
	// prefix ran, for example the listing that verifies a native install.
	after  []string
	landed bool
	// unclosed reports a child whose process-tree cleanup was not proven.
	unclosed bool
	cancel   context.CancelFunc
}

func (i *claudeCLIInterrupt) matches(args []string) bool {
	return i != nil && len(i.after) == 0 && hasArgsPrefix(args, i.args)
}

// ran arms an interrupt that waits for the command args.
func (i *claudeCLIInterrupt) ran(args []string) {
	if i != nil && len(i.after) != 0 && hasArgsPrefix(args, i.after) {
		i.after = nil
	}
}

func hasArgsPrefix(args, prefix []string) bool {
	return len(args) >= len(prefix) && reflect.DeepEqual(args[:len(prefix)], prefix)
}

func (i *claudeCLIInterrupt) fire(r *preparedClaudeCLIRunner, args []string) (legacyports.CommandResult, error) {
	if i.landed {
		if _, err := r.marketplaceCommand(args); err != nil {
			r.t.Fatal(err)
		}
	}
	i.cancel()
	return legacyports.CommandResult{CleanupComplete: !i.unclosed}, context.Canceled
}

func (fixture projectClaudeCLI) runProject(ctx context.Context, args ...string) (string, error) {
	args = append(args, "--target", "claude", "--scope", "project", "--project-root", fixture.project, "--yes", "--format", "json")
	stdout, stderr, err := fixture.executeContext(ctx, false, "", args...)
	return stdout + stderr, err
}

// nativeAttempt returns the retained native attempt marker of the only binding.
func (fixture projectClaudeCLI) nativeAttempt(t *testing.T) (string, bool) {
	t.Helper()
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			return binding.NativeActivationAttempt, true
		}
	}
	return "", false
}

func (fixture projectClaudeCLI) knownMarketplaces(t *testing.T) int {
	t.Helper()
	return len(fixture.runner.readObject(filepath.Join(fixture.client.ConfigRoot, "plugins", "known_marketplaces.json")))
}

// An interrupted native Claude command must not strand the project binding
// behind an unresolved attempt: the same command converges afterwards.
func TestProjectClaudeInterruptedNativeCommandResumesWithSameCommand(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		args          []string
		landed        bool
	}{
		{"add/marketplace-add", "add", []string{"plugin", "marketplace", "add"}, false},
		{"add/install-landed", "add", []string{"plugin", "install"}, true},
		{"update/uninstall-landed", "update", []string{"plugin", "uninstall"}, true},
		{"update/install", "update", []string{"plugin", "install"}, false},
		{"remove/uninstall", "remove", []string{"plugin", "uninstall"}, false},
		{"remove/uninstall-landed", "remove", []string{"plugin", "uninstall"}, true},
		{"remove/marketplace-remove-landed", "remove", []string{"plugin", "marketplace", "remove"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture, args := interruptedProjectClaudeCommand(t, tc.command, &claudeCLIInterrupt{args: tc.args, landed: tc.landed})
			if attempt, _ := fixture.nativeAttempt(t); attempt != "" {
				t.Fatalf("closed interrupted command retained native attempt %s", attempt)
			}
			if out, err := fixture.runProject(context.Background(), args...); err != nil {
				t.Fatalf("same command did not resume: %v\n%s", err, out)
			}
			listed := len(fixture.runner.marketplaceListing())
			attempt, bound := fixture.nativeAttempt(t)
			if tc.command == "remove" && (bound || listed != 0 || fixture.knownMarketplaces(t) != 0) ||
				tc.command != "remove" && (!bound || attempt != "" || listed != 1) {
				t.Fatalf("resumed %s did not converge: bound=%v attempt=%q listed=%d", tc.command, bound, attempt, listed)
			}
		})
	}
}

// A child whose cleanup is not proven may still be writing. While it lives its
// attempt stays unresolved and every retry refuses, including the confirming
// removal, whether or not its effect already landed; removal has no later
// verification to fall back on.
func TestProjectClaudeUnclosedInterruptedCommandKeepsAttempt(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		args          []string
		landed        bool
	}{
		{"add/install", "add", []string{"plugin", "install"}, false},
		{"remove/uninstall", "remove", []string{"plugin", "uninstall"}, false},
		{"remove/uninstall-landed", "remove", []string{"plugin", "uninstall"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture, args := interruptedProjectClaudeCommand(t, tc.command, &claudeCLIInterrupt{args: tc.args, landed: tc.landed, unclosed: true})
			attempt, _ := fixture.nativeAttempt(t)
			if attempt == "" {
				t.Fatal("unclosed native command was resolved without proof")
			}
			fixture.runner.surviving = []legacyports.LiveProcess{{PID: 4242, Argv: append([]string{fixture.client.ExecutablePath}, tc.args...)}}
			out, err := fixture.runProject(context.Background(), args...)
			if err == nil || !strings.Contains(out+err.Error(), attempt) {
				t.Fatalf("retry ignored the unresolved attempt: %v\n%s", err, out)
			}
			// Doctor reports the open attempt and names only the confirming repair
			// and removal, never a blind rerun of the same command.
			doctor, _, err := fixture.execute(false, "doctor", "--target", "claude", "--scope", "project", "--project-root", fixture.project, "--format", "json")
			if err != nil || !strings.Contains(doctor, `"open_operation_count":1`) || !strings.Contains(doctor, "native_attempt_unresolved") || strings.Contains(doctor, "rerun the same") {
				t.Fatalf("doctor misreported the unresolved attempt: %v\n%s", err, doctor)
			}
		})
	}
}

// The listing that verifies a landed install belongs to that install: an
// interrupt there is settled the same way, so the same add finishes the
// activation, while a listing whose cleanup is not proven keeps the attempt.
func TestProjectClaudeInterruptedInstallVerificationResumesWithSameCommand(t *testing.T) {
	for _, unclosed := range []bool{false, true} {
		t.Run(map[bool]string{false: "closed", true: "unclosed"}[unclosed], func(t *testing.T) {
			interrupt := &claudeCLIInterrupt{args: []string{"plugin", "list", "--json"}, after: []string{"plugin", "install"}, unclosed: unclosed}
			fixture, args := interruptedProjectClaudeCommand(t, "add", interrupt)
			attempt, _ := fixture.nativeAttempt(t)
			if (attempt != "") != unclosed {
				t.Fatalf("interrupted install verification left attempt %q", attempt)
			}
			out, err := fixture.runProject(context.Background(), args...)
			if unclosed {
				if err == nil || !strings.Contains(out+err.Error(), attempt) {
					t.Fatalf("retry ignored the unresolved attempt: %v\n%s", err, out)
				}
				return
			}
			if err != nil {
				t.Fatalf("same command did not resume: %v\n%s", err, out)
			}
			state, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			for _, binding := range state.Installations[0].Clients {
				if binding.NativeActivationAttempt != "" || binding.Activation != domain.ActivationActive || len(fixture.runner.marketplaceListing()) != 1 {
					t.Fatalf("resumed add did not converge: %+v", binding)
				}
			}
		})
	}
}

// interruptedProjectClaudeCommand prepares the binding for command, then runs
// it once with interrupt armed and returns the command for the retry.
func interruptedProjectClaudeCommand(t *testing.T, command string, interrupt *claudeCLIInterrupt) (projectClaudeCLI, []string) {
	t.Helper()
	fixture := newProjectClaudeCLI(t)
	pkg := writeCLIPlugin(t)
	skill := filepath.Join(pkg, "skills", "guide", "SKILL.md")
	writeSkill := func(marker string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(skill), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(skill, []byte("---\nname: guide\ndescription: Inert marker\n---\n"+marker+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeSkill("version one")
	args := map[string][]string{"add": {"add", pkg, "--no-save"}, "update": {"update", "demo"}, "remove": {"remove", "demo"}}[command]
	if command != "add" {
		if out, err := fixture.runProject(context.Background(), "add", pkg, "--no-save"); err != nil {
			t.Fatalf("initial add: %v\n%s", err, out)
		}
		writeSkill("version two")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interrupt.cancel = cancel
	fixture.runner.interrupt = interrupt
	if out, err := fixture.runProject(ctx, args...); err == nil || fixture.runner.interrupt != nil {
		t.Fatalf("interrupted %s was not interrupted: %v\n%s", command, err, out)
	}
	return fixture, args
}
