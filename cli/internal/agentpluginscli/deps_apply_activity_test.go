package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Stop at the existing application readiness boundary before any lock, file
// publication or native operation, while observing what the person has seen.
type savedDependencyApplyBoundary struct {
	statev2.Store
	calls   int
	observe func()
	failure error
}

func (store *savedDependencyApplyBoundary) RequireMutationReady() error {
	store.calls++
	if store.observe != nil {
		store.observe()
	}
	return store.failure
}

func TestSavedDependencyConfirmationPrecedesActivityAndApply(t *testing.T) {
	applyFailure := errors.New("controlled application boundary")
	for _, test := range []struct {
		name, input, format               string
		terminal, yes, unchanged          bool
		wantPrompt, wantACK, wantActivity bool
		wantApply                         bool
		wantCause                         error
		wantError                         string
	}{
		{name: "accepted", input: "y\n", format: "human", terminal: true, wantPrompt: true, wantACK: true, wantActivity: true, wantApply: true},
		{name: "declined", input: "n\n", format: "human", terminal: true, wantPrompt: true},
		{name: "default-no", input: "\n", format: "human", terminal: true, wantPrompt: true},
		{name: "input-closed", format: "human", terminal: true, wantPrompt: true, wantCause: prompt.ErrPromptInputClosed},
		{name: "yes", format: "human", terminal: true, yes: true, wantActivity: true, wantApply: true},
		{name: "json-yes", format: "json", terminal: true, yes: true, wantApply: true},
		{name: "json-unconfirmed", format: "json", terminal: true, wantError: "requires --yes"},
		{name: "nonterminal-yes", format: "human", yes: true, wantApply: true},
		{name: "nonterminal-unconfirmed", format: "human", wantError: "requires --yes"},
		{name: "unchanged", input: "y\n", format: "human", terminal: true, unchanged: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			cmd.SetIn(strings.NewReader(test.input))
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			store := &savedDependencyApplyBoundary{failure: applyFailure}
			store.observe = func() {
				view := output.String()
				if strings.Contains(view, "Changes confirmed.") != test.wantACK || strings.Contains(view, "Applying changes") != test.wantActivity {
					t.Fatalf("Apply preceded its expected human output: %q", view)
				}
				if test.wantACK && strings.Index(view, "Changes confirmed.") > strings.Index(view, "Applying changes") {
					t.Fatalf("activity preceded accepted confirmation: %q", view)
				}
			}
			app := App{Terminal: test.terminal, StateStore: store, Output: &output, ErrorOutput: &output}
			action := environment.ActionRemove
			if test.unchanged {
				action = environment.ActionUnchanged
			}
			request := dependencySyncRequest{prepared: []dependencyAction{{Alias: "demo", ClientID: "codex", Action: action}}}
			err := applyDependencySync(cmd, app, &options{format: test.format, plain: true}, request, test.yes)
			wantCause := test.wantCause
			if test.wantApply {
				wantCause = applyFailure
			}
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("missing explicit confirmation refusal: %v", err)
				}
			} else if !errors.Is(err, wantCause) {
				t.Fatalf("original confirmation/application error changed: %v", err)
			}
			if (store.calls == 1) != test.wantApply || store.calls > 1 {
				t.Fatalf("unexpected Apply boundary calls: %d", store.calls)
			}
			view := output.String()
			if strings.Contains(view, "Apply these changes? [y/N]") != test.wantPrompt || strings.Contains(view, "Changes confirmed.") != test.wantACK || strings.Contains(view, "Applying changes") != test.wantActivity {
				t.Fatalf("unexpected prompt/acknowledgment/activity: %q", view)
			}
			if test.wantACK && strings.Count(view, "Changes confirmed.") != 1 {
				t.Fatalf("confirmation acknowledgment was repeated: %q", view)
			}
			if test.wantPrompt && strings.Index(view, "remove demo/codex") > strings.Index(view, "Apply these changes?") {
				t.Fatalf("confirmation preceded its review: %q", view)
			}
		})
	}
}

type savedDependencyACKWriter struct {
	bytes.Buffer
	failure  error
	short    bool
	attempts int
}

func (writer *savedDependencyACKWriter) Write(value []byte) (int, error) {
	if strings.Contains(string(value), "Changes confirmed.") {
		writer.attempts++
		if writer.short {
			return len(value) - 1, nil
		}
		return 0, writer.failure
	}
	return writer.Buffer.Write(value)
}

func TestSavedDependencyAcknowledgmentWriteFailureStopsBeforeApply(t *testing.T) {
	writeFailure := errors.New("confirmation output failed")
	for _, short := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "short"}[short], func(t *testing.T) {
			output := &savedDependencyACKWriter{failure: writeFailure, short: short}
			var progress bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			cmd.SetIn(strings.NewReader("y\n"))
			cmd.SetOut(output)
			cmd.SetErr(&progress)
			store := &savedDependencyApplyBoundary{failure: errors.New("must not reach Apply")}
			app := App{Terminal: true, StateStore: store, ErrorOutput: &progress}
			request := dependencySyncRequest{prepared: []dependencyAction{{Alias: "demo", ClientID: "codex", Action: environment.ActionRemove}}}
			err := applyDependencySync(cmd, app, &options{format: "human", plain: true}, request, false)
			want := writeFailure
			if short {
				want = io.ErrShortWrite
			}
			if !errors.Is(err, want) || output.attempts != 1 || store.calls != 0 || progress.Len() != 0 {
				t.Fatalf("failed acknowledgment reached Apply/activity: err=%v attempts=%d calls=%d progress=%q", err, output.attempts, store.calls, progress.String())
			}
			if !strings.Contains(output.String(), "Apply these changes? [y/N]") {
				t.Fatalf("write failed before actual confirmation: %q", output.String())
			}
		})
	}
}

func TestSavedDependencyDryRunHasNoAcknowledgmentOrApplyActivity(t *testing.T) {
	client := fixtureClient(t, domain.ClientCodex)
	client.ExecutablePath = filepath.Join(t.TempDir(), "codex")
	runner := &dependencyClaudeListing{root: client.ConfigRoot}
	f, _ := newDependencyHarness(t, "", []domain.DetectedClient{client}, installer.Config{Runner: runner})
	pin := seedDependencyCheckPackage(t, f, "demo", "")
	writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
	before, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	out, stderr, err := f.executeInput(true, "y\n", "deps", "sync", "--locked", "--dry-run", "--format", "human")
	if err != nil || !strings.Contains(out, "install demo/codex") {
		t.Fatalf("dry run did not render the actionable plan: %v\n%s\n%s", err, out, stderr)
	}
	for _, unexpected := range []string{"Apply these changes?", "Changes confirmed.", "Applying changes"} {
		if strings.Contains(out+stderr, unexpected) {
			t.Fatalf("dry run reached confirmation/application: %q", out+stderr)
		}
	}
	after, err := f.store.Load()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("dry run changed installer state: %v", err)
	}
}
