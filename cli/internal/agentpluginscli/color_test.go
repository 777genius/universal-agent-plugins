package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func TestColorFlagsBeforeMutation(t *testing.T) {
	for _, flags := range [][]string{{"--color=wrong"}, {"--color=always", "--no-color"}, {"--no-color", "--color=always"}, {"--color=never", "--color=auto"}, {"--no-color=false"}} {
		f := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
		_, _, err := f.execute(false, append([]string{"add", writeCLIPlugin(t), "--target=cursor"}, flags...)...)
		if err == nil {
			t.Fatalf("accepted %v", flags)
		}
		state, _ := f.store.Load()
		if len(state.Installations) != 0 {
			t.Fatal("mutated state")
		}
	}
}
func TestColorCLIOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	for _, tc := range []struct {
		flags []string
		want  bool
	}{
		{[]string{"--color=always"}, true}, {[]string{"--plain", "--color=always"}, true},
		{[]string{"--color=never", "--no-color"}, false}, {[]string{"--color=always", "--color=always"}, true},
		{nil, false}, {[]string{"--color=auto"}, false}, {[]string{"--color=always", "--format=json"}, false},
	} {
		f := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
		out, stderr, err := f.execute(false, append([]string{"add", writeCLIPlugin(t), "--target=cursor", "--dry-run"}, tc.flags...)...)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, "\x1b[") != tc.want {
			t.Fatalf("flags %v color mismatch: %.150q", tc.flags, out)
		}
		if !tc.want && strings.Contains(stderr, "\x1b") {
			t.Fatal("ANSI stderr")
		}
	}
}
func TestJSONHelpAndErrorsHaveNoANSI(t *testing.T) {
	for _, args := range [][]string{{"--color=always", "--format=json", "--help"}, {"add", "--color=always", "--format=json"}, {"add", "--color=always", "--unknown", "--format=json"}} {
		var out bytes.Buffer
		root := NewRoot(App{Output: &out, ErrorOutput: &out})
		root.SetArgs(args)
		err := root.Execute()
		if err != nil {
			out.WriteString(ErrorText(args, &out, err))
		}
		if strings.Contains(out.String(), "\x1b") {
			t.Fatalf("ANSI: %q", out.String())
		}
	}
	if got := ErrorText([]string{"add", "--color=always"}, io.Discard, errors.New("fixture\x1b[31m\nerror")); !strings.HasPrefix(got, "\x1b[31m") || !strings.Contains(got, "\n") {
		t.Fatalf("error: %q", got)
	}
}
func TestColoredPlanShortWritePreventsConsent(t *testing.T) {
	format := "human"
	policy := terminaltheme.Policy{Mode: "always", Explicit: true}
	cmd := NewRoot(App{})
	app := App{reviewOutput: terminaltheme.Wrap(shortPromptWriter{}, &policy, &format), Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		t.Fatal("consent after failed output")
		return prompt.ConfirmationResult{}, nil
	}}}
	accepted, err := confirmInstall(context.Background(), cmd, app, loadedPackage{}, []usecase.AddResult{{}})
	if accepted || !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(accepted, err)
	}
}
func TestColoredPlanValuesAndBindingSanitation(t *testing.T) {
	var out bytes.Buffer
	format := "human"
	p := terminaltheme.Policy{Mode: "always", Explicit: true}
	w := terminaltheme.Wrap(&out, &p, &format)
	result := usecase.AddResult{Plan: domain.DeliveryPlan{ClientID: "cursor", Warnings: []string{"synthetic warning"}}}
	if err := renderHumanPlan(w, domain.PackageEnvelope{}, result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\x1b[36mTarget\x1b[m: cursor\n") || !strings.Contains(out.String(), "\x1b[33mWarning\x1b[m: synthetic warning") {
		t.Fatalf("%q", out.String())
	}
	out.Reset()
	if err := renderHumanBindingPlan(w, usecase.BindingChangePlan{Blockers: []string{"fixture\x1b\nforged"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "\x1b[31mBlocker\x1b[m: fixtureforged\n") {
		t.Fatalf("%q", out.String())
	}
}

// Verbose execution is a deterministic presentation demo using only in-memory data.
func TestSemanticColorDemo(t *testing.T) {
	for _, mode := range []string{"always", "never"} {
		t.Run(mode, func(t *testing.T) {
			var out bytes.Buffer
			format := "human"
			policy := terminaltheme.Policy{Mode: mode, Explicit: true}
			w := terminaltheme.Wrap(&out, &policy, &format)
			result := usecase.AddResult{Plan: domain.DeliveryPlan{ClientID: "cursor", Warnings: []string{"Synthetic review warning; no real scanner was run."}}}
			if err := renderHumanPlan(w, domain.PackageEnvelope{}, result); err != nil {
				t.Fatal(err)
			}
			if err := renderBindingChange(w, "human", "rebind", usecase.BindingChangeResult{}, false); err != nil {
				t.Fatal(err)
			}
			out.WriteString(terminaltheme.For(w).Text(terminaltheme.Success, "Synthetic success") + "\n")
			out.WriteString(terminaltheme.For(w).Text(terminaltheme.Muted, "Synthetic secondary detail") + "\n")
			out.WriteString(ErrorText([]string{"add", "--color=" + mode}, &out, errors.New("synthetic helpful error: choose --target=cursor")) + "\n")
			if strings.Contains(out.String(), "\x1b[") != (mode == "always") {
				t.Fatal("demo policy mismatch")
			}
			t.Log("Synthetic presentation only; no mutation, profile, authentication or agent launch.\n" + out.String())
		})
	}
}

func TestNoColorTypedFlagCompatibility(t *testing.T) {
	root := NewRoot(App{})
	if err := root.ParseFlags([]string{"--no-color"}); err != nil {
		t.Fatal(err)
	}
	value, err := root.PersistentFlags().GetBool("no-color")
	if err != nil || !value {
		t.Fatal("inherited typed bool adapter lost --no-color", value, err)
	}
}
