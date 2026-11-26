package agentpluginscli

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminalprompts"
)

// App.Terminal describes input capability. The production factory remains the
// authority for whether any output stream can present an interactive question.
// Real stdin-TTY/both-output-files coverage belongs to the process PTY suite.
func TestSkillsUnavailableFactoryRefusesHiddenPromptWithoutReadingInput(t *testing.T) {
	for _, test := range []struct {
		name  string
		flags []string
	}{
		{"plain catalog", []string{"--plain"}},
		{"terminal catalog", []string{"--no-ui"}},
		{"target selection", []string{"--path", "engineering/review", "--yes", "--plain"}},
		{"review", []string{"--path", "engineering/review", "--target", "codex", "--plain"}},
		{"catalog despite yes", []string{"--target", "codex", "--yes", "--plain"}},
		{"explicit browser", []string{"--ui"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			journey, _, _ := skillsFullFlowFixture(t)
			app := journey.cli.app
			input := strings.NewReader("1\ndone\nyes\n")
			var stdout, stderr bytes.Buffer
			app.Terminal, app.Input, app.Output, app.ErrorOutput = true, input, &stdout, &stderr
			app.Prompter = nil
			factories := 0
			app.PromptFactory = func(in io.Reader, out, errOut io.Writer, plain, noColor bool) (prompt.Prompter, io.Writer, error) {
				factories++
				return terminalprompts.New(in, out, errOut, plain, noColor)
			}
			before := savedJourneyTree(t, app.DependenciesRoot)
			command := NewRoot(app)
			command.SetArgs(append([]string{"skills", "add", "acme/skills"}, test.flags...))
			err := command.ExecuteContext(t.Context())
			if !errors.Is(err, prompt.ErrPromptUnavailable) || factories != 1 || input.Len() != len("1\ndone\nyes\n") {
				t.Fatalf("hidden prompt or repeated factory: factories=%d unread=%d err=%v", factories, input.Len(), err)
			}
			if strings.Contains(stdout.String(), "Select:") || strings.Contains(stdout.String(), "Available Skills") || strings.Contains(stdout.String(), "Install these Skills?") {
				t.Fatalf("hidden interaction was rendered: %q", stdout.String())
			}
			if journey.runner.calls != 0 || !reflect.DeepEqual(before, savedJourneyTree(t, app.DependenciesRoot)) {
				t.Fatal("unavailable interaction changed native or durable state")
			}
		})
	}
}

func TestSkillsUnavailableFactoryAllowsExplicitNonPromptOperations(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		name := "yes"
		if dryRun {
			name = "dry-run"
		}
		t.Run(name, func(t *testing.T) {
			journey, _, _ := skillsFullFlowFixture(t)
			app := journey.cli.app
			input := strings.NewReader("next-owner\n")
			var stdout, stderr bytes.Buffer
			app.Terminal, app.Input, app.Output, app.ErrorOutput = true, input, &stdout, &stderr
			app.Prompter = nil
			factories := 0
			app.PromptFactory = func(in io.Reader, out, errOut io.Writer, plain, noColor bool) (prompt.Prompter, io.Writer, error) {
				factories++
				return terminalprompts.New(in, out, errOut, plain, noColor)
			}
			before := savedJourneyTree(t, app.DependenciesRoot)
			command := NewRoot(app)
			args := []string{"skills", "add", "acme/skills", "--path", "engineering/review", "--target", "codex", "--plain"}
			if dryRun {
				args = append(args, "--dry-run")
			} else {
				args = append(args, "--yes")
			}
			command.SetArgs(args)
			if err := command.ExecuteContext(t.Context()); err != nil {
				t.Fatalf("explicit non-prompt operation failed: %v\n%s\n%s", err, stdout.String(), stderr.String())
			}
			if factories != 1 || input.Len() != len("next-owner\n") {
				t.Fatalf("explicit operation acquired another prompt: factories=%d unread=%d", factories, input.Len())
			}
			if dryRun && (!strings.Contains(stdout.String(), "review") || !reflect.DeepEqual(before, savedJourneyTree(t, app.DependenciesRoot)) || journey.runner.calls != 0) {
				t.Fatal("dry-run failed to retain stdout authority without effects")
			}
		})
	}
}

func TestSkillsInjectedRichCapabilityMayFallbackWithoutAnotherFactory(t *testing.T) {
	journey, _, _ := skillsFullFlowFixture(t)
	app := journey.cli.app
	input := strings.NewReader("1\ndone\nnext-owner\n")
	var stdout, stderr bytes.Buffer
	app.Terminal, app.Input, app.Output, app.ErrorOutput = true, input, &stdout, &stderr
	calls := 0
	app.Prompter = skillsSelectionPrompter{
		Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) { return prompt.ConfirmationResult{Accepted: false}, nil }},
		selectSkills: func(prompt.SkillsSelectionRequest) (prompt.SkillsSelectionResult, error) {
			calls++
			return prompt.SkillsSelectionResult{}, prompt.ErrPromptUnavailable
		},
	}
	app.PromptFactory = func(io.Reader, io.Writer, io.Writer, bool, bool) (prompt.Prompter, io.Writer, error) {
		t.Fatal("injected host was replaced")
		return nil, nil, prompt.ErrPromptUnavailable
	}
	before := savedJourneyTree(t, app.DependenciesRoot)
	command := NewRoot(app)
	command.SetArgs([]string{"skills", "add", "acme/skills", "--target", "codex", "--no-ui"})
	if err := command.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || input.Len() != len("next-owner\n") || !strings.Contains(stdout.String(), "Available Skills") || !strings.Contains(stdout.String(), "Installation canceled") {
		t.Fatalf("embedding fallback changed: calls=%d unread=%d stdout=%q", calls, input.Len(), stdout.String())
	}
	if journey.runner.calls != 0 || !reflect.DeepEqual(before, savedJourneyTree(t, app.DependenciesRoot)) {
		t.Fatal("declined embedding fallback changed state")
	}
}
