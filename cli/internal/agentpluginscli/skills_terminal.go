package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	if interactive {
		if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Preparing installation review…"); err != nil {
			return err
		}
	}
	prepared, err := app.PrepareSkills(cmd.Context(), request)
	if err != nil {
		if cmd.Context().Err() != nil {
			return cmd.Context().Err()
		}
		refusal := s.reviewRefusal(err)
		var issue *skillspicker.ReviewRefusal
		if errors.As(refusal, &issue) {
			return skillsMissingSelection(cmd, opts, issue.Code, issue.Message, []skillspicker.Candidate{})
		}
		return err
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
	if interactive {
		if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Scanning Skills catalog…"); err != nil {
			return nil, err
		}
	}
	catalog, err := s.scan(cmd.Context(), s.source.RequestedPath)
	if err != nil {
		return nil, err
	}
	if len(names) > 0 {
		return selectNamedSkills(cmd, opts, s, catalog, names)
	}
	if !interactive {
		return nil, skillsMissingSelection(cmd, opts, "SELECTION_REQUIRED", "Use --skill or --path with --target in a non-interactive session", catalog.Candidates)
	}
	return promptSkillsCatalog(cmd, s, catalog)
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
	choices, err := app.skillsTargets(cmd.Context())
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
	if opts.format != "json" {
		if err := renderHumanSkillsReview(cmd, review); err != nil {
			return err
		}
	}
	if opts.dryRun {
		return nil
	}
	if confirm {
		app, err := withPrompter(cmd, app, opts)
		if err != nil {
			return err
		}
		answer, err := app.Prompter.Confirm(cmd.Context(), prompt.ConfirmationRequest{Title: "Install these Skills?", Default: true})
		if err != nil {
			return err
		}
		if !answer.Accepted {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Installation canceled. No changes applied.")
			return err
		}
	}
	result, err := prepared.Apply(cmd.Context())
	if writeErr := renderSkillsResult(cmd, opts, result); writeErr != nil {
		return writeErr
	}
	return err
}
func renderSkillsResult(cmd *cobra.Command, opts *options, result SkillsBatchResult) error {
	if opts.format != "json" {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "Skills installation %s\n", result.Status)
		return err
	}
	outcome := skillBatchOutcome(result)
	output := struct {
		Status             string                       `json:"status"`
		Items              []skillspicker.OperationItem `json:"items"`
		RepairInstructions []string                     `json:"repairInstructions"`
	}{Status: result.Status, Items: outcome.Items, RepairInstructions: outcome.Result.RepairInstructions}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(output)
}
