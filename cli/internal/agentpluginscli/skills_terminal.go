package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func runSkillsTerminal(cmd *cobra.Command, app App, opts *options, s *skillsCatalogSession, names []string, interactive, yes bool) error {
	candidates, err := selectSkillsTerminal(cmd, opts, s, names, interactive)
	if err != nil {
		return err
	}
	targets, err := skillsTerminalTargets(cmd, app, opts, interactive)
	if err != nil {
		return err
	}
	if !interactive && !yes && !opts.dryRun {
		return skillsMissingSelection(cmd, opts, "CONFIRMATION_REQUIRED", "Pass --yes to accept this installation in a non-interactive session; --dry-run reviews without applying", []skillspicker.Candidate{})
	}
	request := SkillsBatchRequest{Selections: []SkillsSelection{}, TargetIDs: []string{}}
	for _, candidate := range candidates {
		request.Selections = append(request.Selections, s.selection(candidate))
	}
	for _, target := range targets {
		request.TargetIDs = append(request.TargetIDs, string(target))
	}
	s.rememberTerminalCandidates(candidates)
	stop := startCommandActivity(cmd, app, opts, "Preparing installation review")
	prepared, err := app.PrepareSkills(cmd.Context(), request)
	stop()
	if err != nil {
		return reportSkillsPreparationRefusal(cmd, opts, s, err)
	}
	defer prepared.Close()
	return runSkillsReviewedPlan(cmd, app, opts, prepared, interactive && !yes)
}
func (s *skillsCatalogSession) exactTerminalCandidate(ctx context.Context, names []string) (*skillsource.Candidate, error) {
	if !s.source.ExactPath && (len(names) > 0 || s.source.RequestedPath == "") {
		return nil, nil
	}
	candidate, err := s.session.CandidateAt(ctx, s.source.RequestedPath)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if s.source.ExactPath {
			return nil, exactSkillIssue(s.source.RequestedPath, nil)
		}
		return nil, nil
	}
	if candidate.Status != "available" {
		return nil, exactSkillIssue(candidate.Path, &candidate)
	}
	if len(names) > 0 && (len(names) != 1 || names[0] != candidate.Name) {
		return nil, skillsSelectionError{Code: "SKILL_MISMATCH", Message: fmt.Sprintf("The selected folder contains Skill %q; --skill must match that exact name", prompt.SafeText(candidate.Name)), Candidates: []skillspicker.Candidate{wireSkillCandidate(candidate)}}
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
	return applySkillsWithRecovery(cmd, app, opts, prepared, confirm && opts.format != "json")
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

func applySkillsTerminalAttempt(cmd *cobra.Command, app App, opts *options, prepared *PreparedSkillsBatch) (SkillsBatchResult, error) {
	if board := startSkillsProgressBoard(app, opts, prepared.Actions()); board != nil {
		result, err := prepared.ApplyWithProgress(cmd.Context(), board.observe)
		board.finish(result)
		return result, err
	}
	stop := startCommandActivity(cmd, app, opts, "Installing selected Skills")
	result, err := prepared.Apply(cmd.Context())
	stop()
	return result, err
}

func renderSkillsResult(cmd *cobra.Command, opts *options, result SkillsBatchResult, logPath string) error {
	if opts.format != "json" {
		writer := cmd.OutOrStdout()
		if err := renderHumanSkillsResult(writer, result); err != nil {
			return err
		}
		reports := reportByAgent(skillsAgentItems(result.Items))
		return renderAgentAttention(writer, reports, logPath, retryTargetHint(reports))
	}
	outcome := skillBatchOutcome(result)
	output := struct {
		SchemaVersion        int                          `json:"schema_version"`
		Command              string                       `json:"command"`
		Result               string                       `json:"result"`
		Status               string                       `json:"status"`
		Items                []skillspicker.OperationItem `json:"items"`
		RepairInstructions   []string                     `json:"repairInstructions"`
		LogFile              string                       `json:"log_file,omitempty"`
		Readiness            string                       `json:"readiness"`
		NextActions          []AgentAction                `json:"next_actions,omitempty"`
		Scope                string                       `json:"scope"`
		ScopeSelectionReason string                       `json:"scope_selection_reason"`
	}{SchemaVersion: outputSchemaVersion, Command: "skills add", Result: outputResultSuccess, Status: result.Status, Items: outcome.Items, RepairInstructions: outcome.Result.RepairInstructions, LogFile: logPath}
	output.Readiness, output.NextActions = deriveSkillsReadiness(result.Status, result.Items...)
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
		"Files: %d of %d target results installed or already up to date; other targets need attention.\n", applied, len(result.Items))
	return err
}
