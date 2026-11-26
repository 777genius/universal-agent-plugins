package agentpluginscli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This is command-layer qualification using the real catalog renderer and bounded
// input reader. Source acquisition/public-process qualification lives separately.
func TestSkillsTwoThousandCandidateCatalogReachesLastPage(t *testing.T) {
	available := consoleLargeCatalog()
	for _, redirected := range []bool{false, true} {
		t.Run(fmt.Sprintf("redirected-%t", redirected), func(t *testing.T) {
			session, catalog := richCatalogFixture(available)
			catalog.Complete = false
			var stdout, visible bytes.Buffer
			command := &cobra.Command{}
			command.Flags().Bool("plain", true, "")
			command.SetOut(&stdout)
			writer := io.Writer(&stdout)
			if redirected {
				writer = &visible
			}
			format := "human"
			policy := terminaltheme.Policy{Mode: "always", Explicit: true}
			session.app.reviewOutput = terminaltheme.Wrap(writer, &policy, &format)
			input := "1\n" + strings.Repeat("next\n", 39) + "50\n/skill-1000\n1\nselected\n99\n2\nall\ndone\n"
			command.SetIn(strings.NewReader(input))
			selected, err := promptSkillsCatalog(command, session, catalog, &options{format: format, plain: true})
			want := []skillsource.Candidate{available[0], available[1999]}
			if err != nil || !reflect.DeepEqual(selected, want) {
				t.Fatalf("large catalog selection=%+v err=%v", selected, err)
			}
			text := stdout.String()
			if redirected {
				text = visible.String()
				if stdout.Len() != 0 {
					t.Fatalf("interactive catalog leaked into redirected stdout: %q", stdout.String())
				}
			}
			for _, expected := range []string{"Page 40 of 40", "docs/skill-1999", "3 of 3 matches; 3 selected", "Choose unique numbers", "Catalog scan is incomplete"} {
				if !strings.Contains(text, expected) {
					t.Fatalf("missing boundary guidance %q", expected)
				}
			}
			if strings.Contains(text, "\x1b") {
				t.Fatal("--plain rendered terminal escapes under explicit color policy")
			}
		})
	}
}

func consoleLargeCatalog() []skillsource.Candidate {
	available := make([]skillsource.Candidate, 2000)
	for index := range available {
		available[index] = skillsource.Candidate{ID: fmt.Sprintf("candidate-%d", index), Name: fmt.Sprintf("skill-%04d", index), Path: fmt.Sprintf("docs/skill-%04d", index)}
	}
	return available
}

func TestSkillsCatalogCancellationDoesNotConsumeNextOwnerInput(t *testing.T) {
	available := consoleLargeCatalog()
	for _, ending := range []string{"q\nnext-owner\n", "\nnext-owner\n", "\x1b\nnext-owner\n", "\x03\nnext-owner\n", "\x04\nnext-owner\n"} {
		t.Run(fmt.Sprintf("%q", ending), func(t *testing.T) {
			input := strings.NewReader("next\n1\n/skill-1999\n1\nselected\n" + ending)
			command := &cobra.Command{}
			command.SetIn(input)
			var output bytes.Buffer
			command.SetOut(&output)
			selected, err := promptFilteredSkills(command, available)
			if len(selected) != 0 || !errors.Is(err, prompt.ErrPromptCanceled) {
				t.Fatalf("cancel submitted basket: selected=%+v err=%v", selected, err)
			}
			remaining, err := io.ReadAll(input)
			if err != nil || string(remaining) != "next-owner\n" {
				t.Fatalf("picker consumed next prompt input: %q %v", remaining, err)
			}
		})
	}
}

func TestSkillsInvalidCatalogAnswersRetainRealBasket(t *testing.T) {
	available := []skillsource.Candidate{
		{ID: "first", Name: "first", Path: "docs/first"},
		{ID: "last", Name: "last", Path: "docs/last"},
	}
	command := &cobra.Command{}
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetIn(strings.NewReader("1\nnext 100\nprev -1\n0\n3\n1,1\n1,invalid\n/last\n1\nselected\ndone\n"))
	selected, err := promptFilteredSkills(command, available)
	if err != nil || !reflect.DeepEqual(selected, available) {
		t.Fatalf("invalid input changed earlier basket: selected=%+v err=%v output=%s", selected, err, output.String())
	}
	if strings.Count(output.String(), "Choose unique numbers") != 6 {
		t.Fatalf("invalid commands silently altered navigation: %s", output.String())
	}
}

type consoleReviewWriteFailure struct {
	remaining int
	cause     error
}

func (writer *consoleReviewWriteFailure) Write(data []byte) (int, error) {
	if writer.remaining <= 0 {
		return 0, writer.cause
	}
	n := min(writer.remaining, len(data))
	writer.remaining -= n
	return n, writer.cause
}

func TestSkillsFailedAuthorityWritesNeverConfirmOrApply(t *testing.T) {
	for _, test := range []struct {
		name   string
		writer io.Writer
		want   error
	}{
		{name: "short-first-row", writer: shortPromptWriter{}, want: io.ErrShortWrite},
		{name: "short-mid-row", writer: &consoleReviewWriteFailure{remaining: 7}, want: io.ErrShortWrite},
		{name: "failed-mid-row", writer: &consoleReviewWriteFailure{remaining: 7, cause: io.ErrClosedPipe}, want: io.ErrClosedPipe},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			command := &cobra.Command{}
			command.SetOut(&stdout)
			confirms := 0
			app := App{reviewOutput: test.writer, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
				confirms++
				return prompt.ConfirmationResult{Accepted: true}, nil
			}}}
			prepared := &PreparedSkillsBatch{actions: []SkillsAction{{Alias: "review", ClientID: "codex", Action: "install", TargetPath: "/authority/complete/destination"}}}
			err := runSkillsReviewedPlan(command, app, &options{format: "human", plain: true}, prepared, true)
			if !errors.Is(err, test.want) || confirms != 0 || prepared.applied || stdout.Len() != 0 {
				t.Fatalf("failed review crossed authority boundary: err=%v confirms=%d applied=%t stdout=%q", err, confirms, prepared.applied, stdout.String())
			}
		})
	}
}

func TestSkillsOversizeReviewFallbackMustFinishVisibleAuthority(t *testing.T) {
	var stdout, unused bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&stdout)
	prompter := &skillsReviewCapture{output: &unused, t: t}
	app := App{reviewOutput: shortPromptWriter{}, Prompter: prompter}
	prepared := &PreparedSkillsBatch{actions: []SkillsAction{{Alias: "review", ClientID: "codex", Action: "install", TargetPath: "/exact/path"}}}
	err := runSkillsReviewedPlan(command, app, &options{format: "human"}, prepared, true)
	if !errors.Is(err, io.ErrShortWrite) || prompter.calls != 1 || prepared.applied || stdout.Len() != 0 {
		t.Fatalf("oversize fallback crossed failed review: err=%v calls=%d applied=%t stdout=%q", err, prompter.calls, prepared.applied, stdout.String())
	}
}

func TestSkillsUnexpectedTargetSelectionCannotPrepareOrConfirm(t *testing.T) {
	journey, _, _ := skillsFullFlowFixture(t)
	confirms, selections := 0, 0
	journey.cli.app.Prompter = fakePrompter{
		selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
			selections++
			return prompt.TargetSelectionResult{IDs: []domain.ClientID{"unexpected-agent"}}, nil
		},
		confirmFn: func() (prompt.ConfirmationResult, error) {
			confirms++
			return prompt.ConfirmationResult{Accepted: true}, nil
		},
	}
	before := savedJourneyTree(t, journey.cli.app.DependenciesRoot)
	_, _, err := journey.cli.executeInput(true, "1\ndone\n", "skills", "add", "acme/skills", "--plain")
	if err == nil || !strings.Contains(err.Error(), "invalid or duplicate target") || selections != 1 || confirms != 0 {
		t.Fatalf("unexpected adapter result accepted: err=%v selections=%d confirms=%d", err, selections, confirms)
	}
	if journey.runner.calls != 0 || !reflect.DeepEqual(before, savedJourneyTree(t, journey.cli.app.DependenciesRoot)) {
		t.Fatal("invalid target changed native or durable state")
	}
}

func TestSkillsNoTargetReasonWriteFailureNeverInvokesSelector(t *testing.T) {
	journey := newSavedJourney(t, t.TempDir())
	app := journey.cli.app
	app.Detector = staticDetector{}
	app.reviewOutput = shortPromptWriter{}
	app.Prompter = fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
		t.Fatal("selector invoked after failed empty-target explanation")
		return prompt.TargetSelectionResult{}, nil
	}}
	command := &cobra.Command{}
	command.SetOut(&bytes.Buffer{})
	_, err := skillsTerminalTargets(command, app, &options{format: "human"}, true)
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("missing output error: %v", err)
	}
}

func TestSkillsAvailableAndUnavailableCatalogShareOneVisibleStream(t *testing.T) {
	session, catalog := richCatalogFixture([]skillsource.Candidate{{ID: "valid", Name: "valid", Path: "docs/valid"}})
	catalog.Complete = true
	catalog.Candidates = append(catalog.Candidates, skillspicker.Candidate{Id: "invalid", Name: "invalid", Path: "docs/invalid", Status: skillspicker.Invalid, DiagnosticCodes: []string{"skill_required_field"}})
	var stdout, visible bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&stdout)
	command.SetIn(strings.NewReader("1\ndone\n"))
	session.app.reviewOutput = &visible
	selected, err := promptSkillsCatalog(command, session, catalog, &options{plain: true})
	if err != nil || len(selected) != 1 || selected[0].ID != "valid" || stdout.Len() != 0 {
		t.Fatalf("mixed catalog failed: selected=%+v err=%v stdout=%q", selected, err, stdout.String())
	}
	for _, expected := range []string{"Unavailable Skill docs/invalid", "name and description", "Available Skills", "docs/valid"} {
		if !strings.Contains(visible.String(), expected) {
			t.Fatalf("missing visible explanation %q: %s", expected, visible.String())
		}
	}
}
