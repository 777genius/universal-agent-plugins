package agentpluginscli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

// claudeCLIInterrupt models Ctrl+C or SIGTERM arriving while one native Claude
// command runs: the command context is canceled and the child is stopped
// either before its effect or just after the effect already landed.
type claudeCLIInterrupt struct {
	args   []string
	landed bool
	// unclosed reports a child whose process-tree cleanup was not proven.
	unclosed bool
	cancel   context.CancelFunc
}

func (i *claudeCLIInterrupt) matches(args []string) bool {
	return i != nil && len(args) >= len(i.args) && reflect.DeepEqual(args[:len(i.args)], i.args)
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

// A child whose cleanup is not proven may still be writing. Its attempt stays
// unresolved and blocks blind retry, exactly as before.
func TestProjectClaudeUnclosedInterruptedCommandKeepsAttempt(t *testing.T) {
	fixture, args := interruptedProjectClaudeCommand(t, "add", &claudeCLIInterrupt{args: []string{"plugin", "install"}, unclosed: true})
	attempt, _ := fixture.nativeAttempt(t)
	if attempt == "" {
		t.Fatal("unclosed native command was resolved without proof")
	}
	out, err := fixture.runProject(context.Background(), args...)
	if err == nil || !strings.Contains(out+err.Error(), attempt) {
		t.Fatalf("retry ignored the unresolved attempt: %v\n%s", err, out)
	}
	// Doctor reports the open attempt and suggests no command that refuses it.
	doctor, _, err := fixture.execute(false, "doctor", "--target", "claude", "--scope", "project", "--project-root", fixture.project, "--format", "json")
	if err != nil || !strings.Contains(doctor, `"open_operation_count":1`) || !strings.Contains(doctor, "native_attempt_unresolved") || strings.Contains(doctor, "rerun the same") {
		t.Fatalf("doctor misreported the unresolved attempt: %v\n%s", err, doctor)
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
