package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestPlainSkillsDoesNotAutomaticallyChooseBrowser(t *testing.T) {
	selection := &skillsAddOptions{}
	if selection.useBrowserPicker(true, &options{plain: true}, false, false) {
		t.Error("--plain still chooses the automatic browser picker")
	}
	selection.ui = true
	if !selection.useBrowserPicker(true, &options{plain: true}, false, false) {
		t.Error("explicit --ui must retain precedence over --plain")
	}
}

func TestSkillsReviewUsesAlreadySelectedVisibleStream(t *testing.T) {
	var stdout, visible bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&visible)
	const destination = "/audit/exact/skill/destination"
	confirmed := false
	app := App{reviewOutput: &visible}
	app.Prompter = fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
		confirmed = true
		if !strings.Contains(visible.String(), destination) {
			t.Errorf("visible confirmation stream lacks authority review; stdout=%q visible=%q", stdout.String(), visible.String())
		}
		return prompt.ConfirmationResult{Accepted: false}, nil
	}}
	plan := &PreparedSkillsBatch{actions: []SkillsAction{{Alias: "review", ClientID: "codex", Action: "install", TargetPath: destination}}}
	if err := runSkillsReviewedPlan(cmd, app, &options{format: "human", plain: true}, plan, true); err != nil {
		t.Fatal(err)
	}
	if !confirmed || plan.applied {
		t.Fatalf("declined confirmation: confirmed=%t applied=%t", confirmed, plan.applied)
	}
}

func TestSkillsNoDetectedTargetsHasActionableDiagnosis(t *testing.T) {
	journey := newSavedJourney(t, t.TempDir())
	app := journey.cli.app
	app.Detector = staticDetector{}
	var stdout, visible bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	cmd.SetErr(&visible)
	app.reviewOutput = &visible
	selectorCalled := false
	app.Prompter = fakePrompter{selectFn: func(request prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
		selectorCalled = true
		return prompt.ValidateSelection(request, request.DefaultIDs)
	}}
	_, err := skillsTerminalTargets(cmd, app, &options{format: "human"}, true)
	if selectorCalled {
		t.Error("empty available-target list was sent to the selection UI")
	}
	if err == nil || strings.Contains(err.Error(), "select at least one target") {
		t.Fatalf("wrong empty-target diagnosis: %v", err)
	}
	if !strings.Contains(visible.String(), "not detected") && !strings.Contains(visible.String(), "not found") {
		t.Errorf("missing-agent reasons invisible: error=%v stdout=%q visible=%q", err, stdout.String(), visible.String())
	}
}

func TestSkillsConsolePresentationRouting(t *testing.T) {
	for _, test := range []struct {
		name                     string
		selection                skillsAddOptions
		opts                     options
		interactive, path, exact bool
		browser                  bool
	}{
		{name: "ordinary", interactive: true, browser: true},
		{name: "plain", interactive: true, opts: options{plain: true}},
		{name: "explicit-ui-plain", interactive: true, selection: skillsAddOptions{ui: true}, opts: options{plain: true}, browser: true},
		{name: "no-ui", interactive: true, selection: skillsAddOptions{noUI: true}},
		{name: "dry-run", interactive: true, opts: options{dryRun: true}},
		{name: "explicit-target", interactive: true, opts: options{target: "codex"}},
		{name: "explicit-skill", interactive: true, selection: skillsAddOptions{names: []string{"review"}}},
		{name: "explicit-path", interactive: true, path: true},
		{name: "exact-source", interactive: true, exact: true},
		{name: "noninteractive"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.selection.useBrowserPicker(test.interactive, &test.opts, test.path, test.exact); got != test.browser {
				t.Fatalf("browser=%t want %t", got, test.browser)
			}
		})
	}
}

func TestSkillsReviewStreamMatrix(t *testing.T) {
	for _, test := range []struct {
		name, format                   string
		confirm, dryRun, visible, rich bool
	}{
		{"interactive", "human", true, false, true, false},
		{"interactive-dry-run", "human", true, true, false, false},
		{"rich-interactive-dry-run", "human", true, true, false, true},
		{"explicit-dry-run", "human", false, true, false, false},
		{"json-dry-run", "json", false, true, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, visible bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&stdout)
			cmd.SetErr(&visible)
			app := App{reviewOutput: &visible, Prompter: fakePrompter{confirmFn: func() (prompt.ConfirmationResult, error) {
				if !strings.Contains(visible.String(), "/authority/destination") {
					t.Fatal("consent preceded visible authority")
				}
				return prompt.ConfirmationResult{Accepted: false}, nil
			}}}
			if test.rich {
				app.Prompter = &skillsReviewCapture{output: &visible, t: t}
			}
			prepared := &PreparedSkillsBatch{actions: []SkillsAction{{Alias: "review", ClientID: "codex", Action: "install", TargetPath: "/authority/destination"}}}
			err := runSkillsReviewedPlan(cmd, app, &options{format: test.format, dryRun: test.dryRun, plain: !test.rich}, prepared, test.confirm)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(visible.String(), "/authority/destination") != test.visible || strings.Contains(stdout.String(), "/authority/destination") == test.visible {
				t.Fatalf("wrong stream stdout=%q visible=%q", stdout.String(), visible.String())
			}
			if prepared.applied {
				t.Fatal("review applied effects")
			}
		})
	}
}

func TestSkillsNumericCatalogUsesInteractionWriter(t *testing.T) {
	session, catalog := richCatalogFixture([]skillsource.Candidate{{ID: "one", Name: "one", Path: "docs/one"}})
	catalog.Complete = true
	var stdout, visible bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetIn(strings.NewReader("oops\n1\ndone\n"))
	cmd.SetOut(&stdout)
	session.app.reviewOutput = &visible
	selected, err := promptSkillsCatalog(cmd, session, catalog, &options{plain: true, format: "human"})
	if err != nil || len(selected) != 1 {
		t.Fatalf("selection=%+v err=%v", selected, err)
	}
	if stdout.Len() != 0 || !strings.Contains(visible.String(), "docs/one") || !strings.Contains(visible.String(), "Choose unique numbers") {
		t.Fatalf("stdout=%q visible=%q", stdout.String(), visible.String())
	}
}

func TestSkillsCatalogOmissionsBoundAndSanitizeReasons(t *testing.T) {
	catalog := skillspicker.Catalog{}
	for i := range 15 {
		catalog.Candidates = append(catalog.Candidates, skillspicker.Candidate{Id: fmt.Sprint(i), Path: fmt.Sprintf("broken-%d\x1b[31m", i), Status: skillspicker.Invalid, DiagnosticCodes: []string{"skill_required_field"}})
	}
	var output bytes.Buffer
	if err := renderSkillsCatalogOmissions(&output, catalog); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if strings.Count(text, "Unavailable Skill") != 10 || !strings.Contains(text, "10 of 15") || !strings.Contains(text, "incomplete") || !strings.Contains(text, "name and description") || strings.Contains(text, "\x1b") {
		t.Fatalf("unsafe/unbounded omissions: %s", text)
	}
}

func TestSkillsEmptyCatalogExplainsReasonsWithoutReadingInput(t *testing.T) {
	session, catalog := richCatalogFixture(nil)
	catalog.Candidates = []skillspicker.Candidate{{Id: "bad", Path: "docs/bad", Status: skillspicker.Invalid, DiagnosticCodes: []string{"skill_name_mismatch"}}}
	var stdout, visible bytes.Buffer
	input := strings.NewReader("1\nyes\n")
	cmd := &cobra.Command{}
	cmd.SetIn(input)
	cmd.SetOut(&stdout)
	session.app.reviewOutput = &visible
	_, err := promptSkillsCatalog(cmd, session, catalog, &options{plain: true})
	if err == nil || !strings.Contains(err.Error(), "--path") || !strings.Contains(visible.String(), "frontmatter name must match") || input.Len() != len("1\nyes\n") || stdout.Len() != 0 {
		t.Fatalf("err=%v stdout=%q visible=%q unread=%d", err, stdout.String(), visible.String(), input.Len())
	}
}

func TestSkillsOmissionOutputFailureStopsSelection(t *testing.T) {
	session, catalog := richCatalogFixture([]skillsource.Candidate{{ID: "available", Name: "available"}})
	catalog.Candidates = append(catalog.Candidates, skillspicker.Candidate{Id: "bad", Path: "bad", Status: skillspicker.Invalid})
	session.app.reviewOutput = shortPromptWriter{}
	input := strings.NewReader("1\ndone\n")
	cmd := &cobra.Command{}
	cmd.SetIn(input)
	_, err := promptSkillsCatalog(cmd, session, catalog, &options{plain: true})
	if !errors.Is(err, io.ErrShortWrite) || input.Len() != len("1\ndone\n") {
		t.Fatalf("err=%v unread=%d", err, input.Len())
	}
}

func TestSkillsReviewSourceMovePreservesAuthorityPaths(t *testing.T) {
	from := "/old/" + strings.Repeat("a", 700)
	to := "/new/" + strings.Repeat("b", 700)
	review := skillspicker.Plan{Actions: []skillspicker.Action{{Alias: "review", TargetId: "codex", Action: "update", DestinationPath: "/destination", SourceMoved: &skillspicker.SourceMoved{From: from, To: to}, ReasonCodes: []string{"source_moved"}}}}
	var output bytes.Buffer
	if err := renderHumanSkillsReviewTo(&output, review); err != nil {
		t.Fatal(err)
	}
	details := strings.Join(skillsReviewConfirmation(review).Details, "\n")
	for _, authority := range []string{from, to, "/destination", "source_moved"} {
		if !strings.Contains(output.String(), authority) || !strings.Contains(details, authority) {
			t.Fatalf("missing authority %q", authority)
		}
	}
}

func TestSkillsPagingPreservesCanonicalBasket(t *testing.T) {
	available := make([]skillsource.Candidate, 120)
	for i := range available {
		available[i] = skillsource.Candidate{ID: fmt.Sprint(i), Name: fmt.Sprintf("skill-%03d", i), Path: fmt.Sprintf("docs/skill-%03d", i)}
	}
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	cmd.SetIn(strings.NewReader("next\n1\nnext\n20\nselected\n2\nall\nprev\n1\n/skill-119\n1\ndone\n"))
	selected, err := promptFilteredSkills(cmd, available)
	want := []skillsource.Candidate{available[0], available[50], available[119]}
	if err != nil || !reflect.DeepEqual(selected, want) {
		t.Fatalf("basket=%+v err=%v", selected, err)
	}
	for _, message := range []string{"Page 2 of 3", "Page 3 of 3", "selected Skills", "3 selected"} {
		if !strings.Contains(output.String(), message) {
			t.Fatalf("missing %q", message)
		}
	}
}

func TestSkillsPagingEmptySelectedViewReturnsToCatalog(t *testing.T) {
	available := []skillsource.Candidate{{ID: "one", Name: "one"}}
	cmd := &cobra.Command{}
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetIn(strings.NewReader("selected\nnext\nprev\n1\nall\n1\ndone\n"))
	selected, err := promptFilteredSkills(cmd, available)
	if err != nil || !reflect.DeepEqual(selected, available) || !strings.Contains(output.String(), "0 of 0 matches") {
		t.Fatalf("selected=%+v err=%v output=%s", selected, err, output.String())
	}
}

func TestSkillsNumericCancellationAndContextNeverSubmit(t *testing.T) {
	available := []skillsource.Candidate{{ID: "one", Name: "one"}}
	for _, input := range []string{"1\nq\n", "1\n\x03\n", "1\n\x04\n", "1\n\x1b\n", "1\n"} {
		cmd := &cobra.Command{}
		cmd.SetIn(strings.NewReader(input))
		cmd.SetOut(&bytes.Buffer{})
		selected, err := promptFilteredSkills(cmd, available)
		if len(selected) != 0 || (!errors.Is(err, prompt.ErrPromptCanceled) && !errors.Is(err, prompt.ErrPromptInputClosed)) {
			t.Fatalf("input=%q selected=%+v err=%v", input, selected, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	cmd.SetIn(strings.NewReader("1\ndone\n"))
	cmd.SetOut(&bytes.Buffer{})
	if _, err := promptFilteredSkills(cmd, available); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestSkillsTargetRequestRetainsSafeReasonsAndPaths(t *testing.T) {
	missing := "This client is not detected; install or configure it before selecting it"
	unreadable := "Client configuration could not be inspected safely"
	request := skillsTargetSelectionRequest([]skillspicker.Target{
		{Id: "codex", Label: "Codex", Available: true, Scope: "user", DisplayPath: "/isolated/codex"},
		{Id: "claude", Label: "Claude", Reason: &missing},
		{Id: "cursor", Label: "Cursor", Reason: &unreadable},
	})
	if len(request.Choices) != 1 || request.Choices[0].Info != "/isolated/codex" || request.Choices[0].Note != "user" || len(request.DefaultIDs) != 1 || len(request.Skipped) != 2 {
		t.Fatalf("lost target facts: %+v", request)
	}
	notice := prompt.SkippedClientsNotice(request.Skipped)
	for _, text := range []string{"Claude", missing, "Cursor", unreadable} {
		if !strings.Contains(notice, text) {
			t.Fatalf("missing %q: %s", text, notice)
		}
	}
	if _, err := prompt.ValidateSelection(request, []domain.ClientID{domain.ClientClaude}); err == nil {
		t.Fatal("unavailable target selectable")
	}
}

func TestSkillsJourneyCarriesFactoryVisibleWriter(t *testing.T) {
	journey, _, _ := skillsFullFlowFixture(t)
	var visible bytes.Buffer
	calls := 0
	journey.cli.app.Prompter = nil
	journey.cli.app.PromptFactory = func(_ io.Reader, _ io.Writer, _ io.Writer, plain, noColor bool) (prompt.Prompter, io.Writer, error) {
		calls++
		return fakePrompter{
			selectFn: func(request prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
				if !strings.Contains(visible.String(), "Available Skills") {
					t.Fatal("numeric selection not on visible factory stream")
				}
				return prompt.ValidateSelection(request, request.DefaultIDs)
			},
			confirmFn: func() (prompt.ConfirmationResult, error) {
				if !strings.Contains(visible.String(), "install") || !strings.Contains(visible.String(), "review") {
					t.Fatal("full review not on visible factory stream")
				}
				return prompt.ConfirmationResult{Accepted: false}, nil
			},
		}, &visible, nil
	}
	before := savedJourneyTree(t, journey.cli.app.DependenciesRoot)
	stdout, _, err := journey.cli.executeInput(true, "1\ndone\n", "skills", "add", "acme/skills", "--plain")
	if err != nil || calls != 1 || strings.Contains(stdout, "Available Skills") || !strings.Contains(visible.String(), "Installation canceled") {
		t.Fatalf("calls=%d err=%v stdout=%q visible=%q", calls, err, stdout, visible.String())
	}
	if !reflect.DeepEqual(before, savedJourneyTree(t, journey.cli.app.DependenciesRoot)) || journey.runner.calls != 0 {
		t.Fatal("declined journey changed persistent environment")
	}
}

func TestSkillsViewNavigationDoesNotChangeBasket(t *testing.T) {
	view := terminalSkillsView{page: 2}
	selected := map[string]bool{"chosen": true}
	for _, line := range []string{"next", "next", "prev", "previous", "p", "selected", "all", "/new query", "/", "n"} {
		if !view.navigate(line, 120) {
			t.Fatalf("unrecognized navigation %q", line)
		}
		if !selected["chosen"] || len(selected) != 1 {
			t.Fatal("navigation changed basket")
		}
	}
	if view.navigate("unknown", 120) {
		t.Fatal("invalid command changed view")
	}
	view.page = 10
	if got := view.pageMatches(nil); len(got) != 0 || view.page != 0 {
		t.Fatalf("empty matches did not clamp page: %+v %d", got, view.page)
	}
}

func TestSkillsSelectedViewShowsBasketAcrossFilters(t *testing.T) {
	available := []skillsource.Candidate{
		{ID: "docs", Name: "docs-review", Path: "docs/review"},
		{ID: "engineering", Name: "engineering-review", Path: "engineering/review"},
	}
	var output bytes.Buffer
	command := &cobra.Command{}
	command.SetOut(&output)
	command.SetIn(strings.NewReader("/docs\n1\n/engineering\n1\nselected\n2\ndone\n"))
	selected, err := promptFilteredSkills(command, available)
	if err != nil || !reflect.DeepEqual(selected, available[:1]) {
		t.Fatalf("selected basket retained stale filter: selected=%+v err=%v output=%s", selected, err, output.String())
	}
	if !strings.Contains(output.String(), "2 of 2 matches; 2 selected") {
		t.Fatalf("basket hid selections from another query: %s", output.String())
	}
}
func TestSkillsRichCatalogRequestRetainsDeepPathSuffix(t *testing.T) {
	prefix := strings.Repeat("shared/", 100)
	request := skillsCatalogRequest([]skillsource.Candidate{
		{ID: "first", Name: "review", Path: prefix + "first/review"},
		{ID: "second", Name: "review", Path: prefix + "second/review"},
	})
	if len(request.Choices) != 2 || request.Choices[0].Label == request.Choices[1].Label || !strings.HasSuffix(request.Choices[0].Label, "first/review") || !strings.HasSuffix(request.Choices[1].Label, "second/review") {
		t.Fatalf("request lost suffix before terminal adapter: %+v", request)
	}
}
