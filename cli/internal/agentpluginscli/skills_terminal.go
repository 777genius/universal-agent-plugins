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

type skillsSelectionError struct {
	Code       string                   `json:"code"`
	Message    string                   `json:"message"`
	Candidates []skillspicker.Candidate `json:"candidates"`
}

func (e skillsSelectionError) Error() string { return e.Message }
func skillsMissingSelection(cmd *cobra.Command, opts *options, code, message string, candidates []skillspicker.Candidate) error {
	err := skillsSelectionError{Code: code, Message: message, Candidates: candidates}
	if opts.format == "json" {
		_ = json.NewEncoder(cmd.OutOrStdout()).Encode(err)
	}
	return err
}
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
	prepared, err := app.PrepareSkills(cmd.Context(), request)
	if err != nil {
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
		if s.source.ExactPath {
			return nil, fmt.Errorf("selected Skill folder is unavailable: %w", err)
		}
		return nil, nil
	}
	if candidate.Status != "available" {
		return nil, errors.New("selected Skill metadata is invalid or unavailable")
	}
	if len(names) > 0 && (len(names) != 1 || names[0] != candidate.Name) {
		return nil, errors.New("--skill does not match the exact selected Skill folder")
	}
	return &candidate, nil
}
func selectSkillsTerminal(cmd *cobra.Command, opts *options, s *skillsCatalogSession, names []string, interactive bool) ([]skillsource.Candidate, error) {
	candidate, err := s.exactTerminalCandidate(cmd.Context(), names)
	if err != nil {
		return nil, err
	}
	if candidate != nil {
		return []skillsource.Candidate{*candidate}, nil
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
			return nil, errors.New("--skill requires unique non-empty exact names")
		}
		seen[name] = true
		matches := []skillsource.Candidate{}
		for _, candidate := range s.candidates {
			if candidate.Name == name && candidate.Status == "available" {
				matches = append(matches, candidate)
			}
		}
		if len(matches) != 1 {
			return nil, skillsMissingSelection(cmd, opts, "SKILL_AMBIGUOUS", "Skill name is missing or ambiguous; choose its exact --path", catalog.Candidates)
		}
		candidates = append(candidates, matches[0])
	}
	return candidates, nil
}
func promptSkillsCatalog(cmd *cobra.Command, s *skillsCatalogSession, catalog skillspicker.Catalog) ([]skillsource.Candidate, error) {
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Available Skills:")
	available := []skillsource.Candidate{}
	for _, candidate := range catalog.Candidates {
		if candidate.Status == skillspicker.Available {
			available = append(available, s.candidates[candidate.Id])
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s (%s)\n", len(available), prompt.SafeText(candidate.Name), prompt.SafeText(candidate.Path))
		}
	}
	if len(available) == 0 {
		return nil, errors.New("no available Skills in this catalog")
	}
	_, _ = fmt.Fprint(cmd.OutOrStdout(), "Choose Skill numbers, comma-separated: ")
	line, err := readInputLine(cmd.Context(), cmd.InOrStdin())
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return numberedSkillSelection(line, available)
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
		return json.NewEncoder(cmd.OutOrStdout()).Encode(review)
	}
	if opts.format != "json" {
		for _, action := range review.Actions {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s → %s (%s)\n", action.Action, prompt.SafeText(action.Alias), prompt.SafeText(action.TargetId), prompt.SafeText(action.DestinationPath))
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
		if err != nil || !answer.Accepted {
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
