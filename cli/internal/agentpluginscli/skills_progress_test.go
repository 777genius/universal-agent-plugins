package agentpluginscli

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func completedSkillClient() installer.ClientResult {
	return installer.ClientResult{Activation: string(domain.ActivationActive), Verification: string(domain.VerificationInstalled), Authentication: string(domain.AuthenticationNotRequired)}
}

func TestSkillsProgressCountsAllBindingsBeforeFinishingAgent(t *testing.T) {
	var output bytes.Buffer
	actions := []SkillsAction{{Alias: "first", ClientID: "codex", Action: "install"}, {Alias: "second", ClientID: "codex", Action: "install"}, {Alias: "first", ClientID: "claude", Action: "install"}}
	progress := startSkillsProgressBoard(App{Terminal: true, ErrorOutput: &output}, &options{format: "human", plain: true}, actions)
	event := SkillsProgress{Alias: "first", ClientID: "codex", Action: "install", Phase: "finished", Result: installer.Result{Outcome: installer.OutcomeCompleted, Client: completedSkillClient()}}
	progress.observe(SkillsProgress{Alias: "first", ClientID: "codex", Action: "install", Phase: "preparing"})
	progress.observe(event)
	row := progress.board.rows[0]
	if row.bindingsCompleted != 1 || row.bindingsTotal != 2 || row.bindingState == "installed" {
		t.Fatalf("first Skill prematurely finished the agent: %+v", row)
	}
	progress.observe(event) // duplicate observations cannot inflate counts.
	if progress.board.rows[0].bindingsCompleted != 1 {
		t.Fatal("duplicate event inflated completion")
	}
	progress.finish(SkillsBatchResult{Status: "partial", Items: []SkillsBatchItem{
		{Alias: "first", ClientID: "codex", Action: "install", Status: "completed", Client: completedSkillClient()},
		{Alias: "second", ClientID: "codex", Action: "install", Status: "failed"},
		{Alias: "first", ClientID: "claude", Action: "install", Status: "not_attempted"},
	}})
	if progress.board.rows[0].bindingState != "failed" || progress.board.rows[1].bindingState != "not attempted" {
		t.Fatalf("wrong final rows: %+v", progress.board.rows)
	}
	for _, want := range []string{"1/2 waiting", "1/2 failed", "0/1 not attempted"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("missing %q: %s", want, output.String())
		}
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatal("plain progress contains terminal controls")
	}
}

func TestSkillsProgressFinalResultOverridesEarlierEvents(t *testing.T) {
	var output bytes.Buffer
	progress := startSkillsProgressBoard(App{Terminal: true, ErrorOutput: &output}, &options{format: "human"}, []SkillsAction{{Alias: "same-name-source-a", ClientID: "codex", Action: "install"}, {Alias: "same-name-source-b", ClientID: "codex", Action: "unchanged"}})
	progress.observe(SkillsProgress{Alias: "same-name-source-a", ClientID: "codex", Action: "install", Phase: "finished", Result: installer.Result{Outcome: installer.OutcomeCompleted, Client: completedSkillClient()}})
	progress.finish(SkillsBatchResult{Status: "failed"})
	if row := progress.board.rows[0]; row.bindingsCompleted != 0 || row.bindingState != "not attempted" {
		t.Fatalf("animation overrode transaction result: %+v", row)
	}
}

func TestSkillsProgressCancellationAfterOneBindingKeepsItsSuccess(t *testing.T) {
	var output bytes.Buffer
	progress := startSkillsProgressBoard(App{Terminal: true, ErrorOutput: &output}, &options{format: "human"}, []SkillsAction{{Alias: "one", ClientID: "codex", Action: "install"}, {Alias: "two", ClientID: "codex", Action: "install"}})
	progress.finish(SkillsBatchResult{Status: "partial", Items: []SkillsBatchItem{{Alias: "one", ClientID: "codex", Action: "install", Status: "completed", Client: completedSkillClient()}, {Alias: "two", ClientID: "codex", Action: "install", Status: "not_attempted"}}})
	if row := progress.board.rows[0]; row.bindingsCompleted != 1 || row.bindingState != "not completed" {
		t.Fatalf("partial agent was presented as wholly unattempted: %+v", row)
	}
}

func TestSkillsProgressPreservesReadinessAndUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name string
		item SkillsBatchItem
		want string
	}{
		{"verified", SkillsBatchItem{Status: "completed", Client: completedSkillClient()}, "installed"},
		{"untrusted", SkillsBatchItem{Status: "completed", Client: installer.ClientResult{Activation: string(domain.ActivationPrepared), Verification: string(domain.VerificationInstalled), Authentication: string(domain.AuthenticationNotRequired)}}, "setup required"},
		{"sign in", SkillsBatchItem{Status: "completed", Client: installer.ClientResult{Authentication: string(domain.AuthenticationPending)}}, "sign-in required"},
		{"rollback", SkillsBatchItem{Status: "incomplete", Client: installer.ClientResult{GroupPhase: string(usecase.GroupTargetManagedRolledBack)}}, "rolled back"},
		{"unchanged", SkillsBatchItem{Status: "unchanged"}, "unchanged"},
		{"cancel", SkillsBatchItem{Status: "cancelled"}, "canceled"}, //nolint:misspell // Preserve the published status value.
		{"removed", SkillsBatchItem{Status: "completed", Action: "remove"}, "removed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := skillsItemPresentation(tc.item); got != tc.want {
				t.Fatalf("%s != %s", got, tc.want)
			}
		})
	}
}

func TestSkillsProgressCompletedCounterDoesNotHideSetupRequired(t *testing.T) {
	var output bytes.Buffer
	progress := startSkillsProgressBoard(App{Terminal: true, ErrorOutput: &output}, &options{format: "human", plain: true}, []SkillsAction{
		{Alias: "one", ClientID: "claude", Action: "install"}, {Alias: "two", ClientID: "claude", Action: "unchanged"},
	})
	progress.finish(SkillsBatchResult{Status: "succeeded", Items: []SkillsBatchItem{
		{Alias: "one", ClientID: "claude", Action: "install", Status: "completed", Client: installer.ClientResult{Activation: string(domain.ActivationPrepared), Verification: string(domain.VerificationInstalled), Authentication: string(domain.AuthenticationNotRequired)}},
		{Alias: "two", ClientID: "claude", Action: "unchanged", Status: "unchanged"},
	}})
	row := progress.board.rows[0]
	if row.bindingsCompleted != 2 || row.bindingState != "setup required" || !strings.Contains(output.String(), "! 2/2 setup required") {
		t.Fatalf("coarse completion overstated readiness: %+v: %s", row, output.String())
	}
}

func TestSkillsResultMappingDoesNotBlameAnUnattemptedBinding(t *testing.T) {
	actions := []SkillsAction{{Alias: "one", ClientID: "claude", Action: "install"}, {Alias: "one", ClientID: "codex", Action: "install"}, {Alias: "two", ClientID: "claude", Action: "install"}}
	failure := errors.New("native error")
	for _, tc := range []struct {
		name, started string
		native        []installer.Result
		want          []string
	}{
		{"before bindings", "", nil, []string{"not_attempted", "not_attempted", "not_attempted"}},
		{"first prepare", skillsBindingKey("one", "claude", "install"), nil, []string{"failed", "not_attempted", "not_attempted"}},
		{"first apply", skillsBindingKey("one", "claude", "install"), []installer.Result{{Outcome: installer.OutcomeIncomplete}}, []string{"incomplete", "not_attempted", "not_attempted"}},
		{"second prepare", skillsBindingKey("one", "codex", "install"), []installer.Result{{Outcome: installer.OutcomeCompleted}}, []string{"completed", "failed", "not_attempted"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, item := range skillsBatchItems(actions, tc.native, failure, tc.started) {
				got = append(got, item.Status)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("%v != %v", got, tc.want)
			}
		})
	}
}

type progressPanicWriter struct{}

func (progressPanicWriter) Write([]byte) (int, error) { panic("broken terminal") }

func TestSkillsProgressObserversAndWritersAreNonAuthoritative(t *testing.T) {
	observed := 0
	for range 3 {
		emitSkillsProgress(func(SkillsProgress) { observed++; panic("observer failure") }, SkillsProgress{})
	}
	if observed != 3 {
		t.Fatal("observer interrupted caller")
	}
	native := installer.Result{Outcome: installer.OutcomeCompleted, ManualActions: []string{"trust workspace"}, Client: installer.ClientResult{RequiredComponents: []string{"skill"}, Failure: &installer.ClientFailure{Message: "native fact"}}}
	emitSkillsProgress(func(event SkillsProgress) {
		event.Result.ManualActions[0] = "changed"
		event.Result.Client.RequiredComponents[0] = "changed"
		event.Result.Client.Failure.Message = "changed"
	}, SkillsProgress{Result: native})
	if native.ManualActions[0] != "trust workspace" || native.Client.RequiredComponents[0] != "skill" || native.Client.Failure.Message != "native fact" {
		t.Fatal("observer changed transaction result fac…4967 tokens truncated…KILL_MISMATCH", Message: fmt.Sprintf("The selected folder contains Skill %q; --skill must match that exact name", prompt.SafeText(candidate.Name)), Candidates: []skillspicker.Candidate{wireSkillCandidate(candidate)}}
	}
	return &candidate, nil
}
func selectSkillsTerminal(cmd *cobra.Command, opts *options, s *skillsCatalogSession, names []string, interactive bool) ([]skillsource.Candidate, error) {
	candidate, err := s.exactTerminalCandidate(cmd.Context(), names)
	if err != nil {
		var issue skillsSelectionError
		if errors.As(err, &issue) {
			return nil, emitSkillsSelectionIssue(cmd, opts, issue)
		}
		return nil, err
	}
	if candidate != nil {
		return []skillsource.Candidate{*candidate}, nil
	}
	stop := startCommandActivity(cmd, s.app, opts, "Scanning Skills catalog")
	catalog, err := s.scan(cmd.Context(), s.source.RequestedPath)
	stop()
	if err != nil {
		return nil, err
	}
	if len(names) > 0 {
		return selectNamedSkills(cmd, opts, s, catalog, names)
	}
	if !interactive {
		return nil, skillsMissingSelection(cmd, opts, "SELECTION_REQUIRED", "Use --skill or --path with --target in a non-interactive session", catalog.Candidates)
	}
	return promptSkillsCatalog(cmd, s, catalog, opts)
}
func selectNamedSkills(cmd *cobra.Command, opts *options, s *skillsCatalogSession, catalog skillspicker.Catalog, names []string) ([]skillsource.Candidate, error) {
	if !catalog.Complete {
		return nil, skillsMissingSelection(cmd, opts, "CATALOG_INCOMPLETE", "The catalog is incomplete; narrow the source with an exact --path", catalog.Candidates)
	}
	candidates := []skillsource.Candidate{}
	seen := map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			return nil, skillsMissingSelection(cmd, opts, "INVALID_SELECTION", "--skill requires unique non-empty exact names", []skillspicker.Candidate{})
		}
		seen[name] = true
		matches := []skillsource.Candidate{}
		for _, candidate := range s.candidates {
			if candidate.Name == name {
				matches = append(matches, candidate)
			}
		}
		if issue := namedSkillIssue(name, matches, catalog.Candidates); issue != nil {
			return nil, emitSkillsSelectionIssue(cmd, opts, *issue)
		}
		candidates = append(candidates, matches[0])
	}
	return candidates, nil
}
func numberedSkillSelection(line string, available []skillsource.Candidate) ([]skillsource.Candidate, error) {
	result := []skillsource.Candidate{}
	seen := map[int]bool{}
	for _, part := range strings.Split(strings.TrimSpace(line), ",") {
		index, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || index < 1 || index > len(available) || seen[index] {
			return nil, errors.New("choose unique Skill numbers from the catalog")
		}
		seen[index] = true
		result = append(result, available[index-1])
	}
	return result, nil
}
func skillsTerminalTargets(cmd *cobra.Command, app App, opts *options, interactive bool) ([]domain.ClientID, error) {
	targets, err := parseTargetOption(opts.target)
	if err != nil || len(targets) > 0 {
		return targets, err
	}
	if !interactive {
		return nil, skillsMissingSelection(cmd, opts, "TARGET_REQUIRED", "Explicit --target is required in a non-interactive session", []skillspicker.Candidate{})
	}
	app, err = withPrompter(cmd, app, opts)
	if err != nil {
		return nil, err
	}
	stop := startCommandActivity(cmd, app, opts, "Detecting available agents")
	choices, err := app.skillsTargets(cmd.Context())
	stop()
	if err != nil {
		return nil, err
	}
	request := prompt.TargetSelectionRequest{}
	for _, target := range choices {
		if target.Available {
			request.Choices = append(request.Choices, prompt.TargetChoice{ID: domain.ClientID(target.Id), Label: target.Label})
			request.DefaultIDs = append(request.DefaultIDs, domain.ClientID(target.Id))
		}
	}
	selected, err := app.Prompter.SelectTargets(cmd.Context(), request)
	if err != nil {
		return nil, err
	}
	validated, err := prompt.ValidateSelection(request, selected.IDs)
	return validated.IDs, err
}
func runSkillsReviewedPlan(cmd *cobra.Command, app App, opts *options, prepared *PreparedSkillsBatch, confirm bool) error {
	review := skillsPickerPrepared{prepared}.Review()
	if opts.format == "json" && opts.dryRun {
		return renderSkillsDryRun(cmd, review)
	}
	if confirm {
		var err error
		app, err = withPrompter(cmd, app, opts)
		if err != nil {
			return err
		}
	}
	richReview := false
	if rich, ok := app.Prompter.(interface{ RichReview() bool }); ok {
		richReview = confirm && !opts.plain && opts.format == "human" && rich.RichReview()
	}
	if opts.format != "json" && !richReview {
		if err := renderHumanSkillsReview(cmd, review); err != nil {
			return err
		}
	}
	if opts.dryRun {
		return nil
	}
	if confirm {
		accepted, err := confirmSkillsReview(cmd, app, review, richReview)
		if err != nil {
			return err
		}
		if !accepted {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Installation canceled. No changes applied.")
			return err
		}
	}
	var result SkillsBatchResult
	var err error
	if board := startSkillsProgressBoard(app, opts, prepared.Actions()); board != nil {
		result, err = prepared.ApplyWithProgress(cmd.Context(), board.observe)
		board.finish(result)
	} else {
		stop := startCommandActivity(cmd, app, opts, "Installing selected Skills")
		result, err = prepared.Apply(cmd.Context())
		stop()
	}
	if writeErr := renderSkillsResult(cmd, opts, result); writeErr != nil {
		return errors.Join(err, writeErr)
	}
	return err
}
func confirmSkillsReview(cmd *cobra.Command, app App, review skillspicker.Plan, rich bool) (bool, error) {
	request := prompt.ConfirmationRequest{Title: "Install these Skills?", Default: true}
	if rich {
		request = skillsReviewConfirmation(review)
	}
	answer, err := app.Prompter.Confirm(cmd.Context(), request)
	if errors.Is(err, prompt.ErrReviewTooLarge) {
		if err := renderHumanSkillsReview(cmd, review); err != nil {
			return false, err
		}
		answer, err = app.Prompter.Confirm(cmd.Context(), prompt.ConfirmationRequest{Title: "Install the reviewed Skills?", Default: true})
	}
	return answer.Accepted, err
}

func renderSkillsResult(cmd *cobra.Command, opts *options, result SkillsBatchResult) error {
	if opts.format != "json" {
		return renderHumanSkillsResult(cmd.OutOrStdout(), result)
	}
	outcome := skillBatchOutcome(result)
	output := struct {
		SchemaVersion        int                          `json:"schema_version"`
		Command              string                       `json:"command"`
		Result               string                       `json:"result"`
		Status               string                       `json:"status"`
		Items                []skillspicker.OperationItem `json:"items"`
		RepairInstructions   []string                     `json:"repairInstructions"`
		Readiness            string                       `json:"readiness"`
		NextActions          []AgentAction                `json:"next_actions,omitempty"`
		Scope                string                       `json:"scope"`
		ScopeSelectionReason string                       `json:"scope_selection_reason"`
	}{SchemaVersion: outputSchemaVersion, Command: "skills add", Result: outputResultSuccess, Status: result.Status, Items: outcome.Items, RepairInstructions: outcome.Result.RepairInstructions}
	output.Readiness, output.NextActions = deriveSkillsReadiness(result.Status)
	if output.Readiness == "failed" {
		output.Result = outputResultFailure
	}
	output.Scope, output.ScopeSelectionReason = opts.scope, scopeSelectionReason(cmd, opts)
	return json.NewEncoder(cmd.OutOrStdout()).Encode(output)
}

// A partial batch can still deliver some Skills; files do not prove native readiness.
func renderPartialSkillsReadiness(writer io.Writer, result SkillsBatchResult) error {
	if result.Status != "partial" {
		return nil
	}
	applied := 0
	for _, item := range result.Items {
		if item.Status == "completed" || item.Status == "unchanged" {
			applied++
		}
	}
	if applied == 0 {
		return nil
	}
	_, err := fmt.Fprintf(writer,
		"Files: %d of %d target results installed or already up to date; other targets need attention.\n"+
			"Agent discovery: not verified by this Skills flow.\n"+
			"Activation: not verified.\n", applied, len(result.Items))
	return err
}
