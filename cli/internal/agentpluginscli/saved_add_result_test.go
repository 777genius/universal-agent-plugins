package agentpluginscli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environmentapp"
)

func TestSavedPublicationNoticeUsesObservedEffects(t *testing.T) {
	for _, test := range []struct {
		name    string
		applied environmentapp.Result
		want    string
	}{
		{name: "no publication", want: ""},
		{name: "changed or unreadable", applied: environmentapp.Result{Published: true}, want: "intent may have changed"},
		{name: "binding failure", applied: environmentapp.Result{Published: true, Outcomes: []environment.BindingOutcome{{Status: environment.BindingFailed}}}, want: "requested targets, not successful installations"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{Use: "add"}
			cmd.SetOut(&out)
			request := dependencySyncRequest{intent: environmentapp.Intent{ManifestPath: "/selected project/uap.toml", LockPath: "/selected project/uap.lock"}}
			opts := &options{format: "human", scope: "project", environmentOutput: map[string]any{"saved": true}}
			app := App{Scope: "project", ProjectRoot: "/selected project"}
			if err := renderDependencyAppliedResults(cmd, app, opts, request, test.applied, errors.New("failed")); err != nil {
				t.Fatal(err)
			}
			text := out.String()
			if test.want == "" && text != "" || test.want != "" && !strings.Contains(text, test.want) {
				t.Fatalf("notice %q, want %q", text, test.want)
			}
			if test.want != "" && (!strings.Contains(text, "Scope: project") || !strings.Contains(text, request.intent.ManifestPath) || !strings.Contains(text, request.intent.LockPath)) {
				t.Fatalf("selected environment context lost: %s", text)
			}
			for _, forbidden := range []string{"nothing changed", "intent saved", "install --locked", "deps sync --target"} {
				if strings.Contains(text, forbidden) {
					t.Fatalf("unsupported publication claim or retry: %s", text)
				}
			}
		})
	}
}

func TestRetryHintDistinguishesSourceInstallFromEnvironmentConvergence(t *testing.T) {
	reports := reportByAgent([]agentItem{failedItem("pdf", "chatgpt", errors.New("native command failed"))})
	for _, test := range []struct {
		verb string
		args []string
		want string
	}{
		{verb: "add", args: []string{"source"}, want: "--target chatgptweb"},
		{verb: "install", args: []string{"source"}, want: "--target chatgptweb"},
		{verb: "install", want: "complete saved environment"},
		{verb: "sync", want: "complete saved environment"},
	} {
		cmd := &cobra.Command{Use: test.verb}
		if err := cmd.ParseFlags(test.args); err != nil {
			t.Fatal(err)
		}
		hint := retryHintFor(cmd, reports)
		if !strings.Contains(hint, test.want) {
			t.Fatalf("%s %v: %q lacks %q", test.verb, test.args, hint, test.want)
		}
		if len(test.args) == 0 && strings.Contains(hint, "--target") {
			t.Fatalf("environment convergence acquired unsupported target flag: %s", hint)
		}
	}
}
