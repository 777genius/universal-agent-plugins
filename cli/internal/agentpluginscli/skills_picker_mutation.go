package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func (s *skillsCatalogSession) prepare(ctx context.Context, input skillspicker.PlanRequest) (skillspicker.Prepared, error) {
	s.mu.Lock()
	if input.CatalogId != s.catalog.CatalogId || input.CatalogDigest != s.catalog.CatalogDigest {
		s.mu.Unlock()
		return nil, errors.New("catalog changed")
	}
	selections := []SkillsSelection{}
	for _, id := range input.SelectedIds {
		candidate, ok := s.candidates[id]
		if !ok || candidate.Status != "available" {
			s.mu.Unlock()
			return nil, errors.New("invalid selected candidate")
		}
		selections = append(selections, s.selection(candidate))
	}
	s.mu.Unlock()
	prepared, err := s.app.PrepareSkills(ctx, SkillsBatchRequest{Selections: selections, TargetIDs: input.TargetIds})
	if err != nil {
		return nil, s.reviewRefusal(err)
	}
	return skillsPickerPrepared{prepared}, nil
}
func (s *skillsCatalogSession) selection(candidate skillsource.Candidate) SkillsSelection {
	selector := environment.Selector{Kind: environment.SelectorRef, Value: s.source.RequestedSelector}
	switch s.source.RequestedSelector {
	case "default":
		selector = environment.Selector{Kind: environment.SelectorDefault, Value: ""}
	case s.source.Commit:
		selector = environment.Selector{Kind: environment.SelectorCommit, Value: s.source.Commit}
	}
	return SkillsSelection{Repository: s.source.Repository, Commit: s.source.Commit, Path: candidate.Path, Name: candidate.Name, CandidateID: candidate.ID, Selector: selector}
}

type skillsPickerPrepared struct{ *PreparedSkillsBatch }

func (p skillsPickerPrepared) Review() skillspicker.Plan {
	review := skillspicker.Plan{Actions: []skillspicker.Action{}, Conflicts: []skillspicker.Diagnostic{}, Warnings: []skillspicker.Diagnostic{}, Confirmable: true}
	for _, a := range p.Actions() {
		action := skillspicker.Action{CandidateId: a.CandidateID, TargetId: a.ClientID, Alias: a.Alias, Action: skillspicker.ActionAction(a.Action), DestinationPath: a.TargetPath, ReasonCodes: []string{}}
		if a.FromPath != "" || a.ToPath != "" {
			action.SourceMoved = &skillspicker.SourceMoved{From: a.FromPath, To: a.ToPath}
		}
		review.Actions = append(review.Actions, action)
	}
	for _, warning := range p.Warnings() {
		review.Warnings = append(review.Warnings, skillspicker.Diagnostic{Code: "INSTALLER_WARNING", Severity: skillspicker.Warning, Message: warning})
	}
	return review
}
func (p skillsPickerPrepared) Check(ctx context.Context) error {
	err := p.PreparedSkillsBatch.Check(ctx)
	if errors.Is(err, ErrSkillsPlanStale) {
		return skillspicker.ErrPlanStale
	}
	return err
}
func (p skillsPickerPrepared) Apply(ctx context.Context) (skillspicker.Outcome, error) {
	result, err := p.PreparedSkillsBatch.Apply(ctx)
	if errors.Is(err, ErrSkillsPlanStale) {
		err = skillspicker.ErrPlanStale
	}
	return skillBatchOutcome(result), err
}
func skillBatchOutcome(result SkillsBatchResult) skillspicker.Outcome {
	outcome := skillspicker.Outcome{State: skillspicker.OperationState(result.Status), Items: []skillspicker.OperationItem{}, Diagnostics: []skillspicker.Diagnostic{}, Result: skillspicker.OperationResult{Message: fmt.Sprintf("Skills installation %s", result.Status), RepairInstructions: append([]string{}, result.RepairInstructions...)}}
	for _, item := range result.Items {
		alias, path := item.Alias, item.TargetPath
		message := skillOperationMessage(skillOperationItemStatus(item.Status), item.Reason)
		outcome.Items = append(outcome.Items, skillspicker.OperationItem{CandidateId: item.CandidateID, TargetId: item.ClientID, Alias: &alias, DestinationPath: &path, Message: &message, Status: skillOperationItemStatus(item.Status)})
	}
	return outcome
}

func skillOperationItemStatus(status string) skillspicker.OperationItemStatus {
	switch status {
	case "completed", "succeeded":
		return skillspicker.OperationItemStatusSucceeded
	case "unchanged":
		return skillspicker.OperationItemStatusUnchanged
	case "not_attempted", "skipped":
		return skillspicker.OperationItemStatusSkipped
	default:
		return skillspicker.OperationItemStatusFailed
	}
}

func (s *skillsCatalogSession) scanProgress() skillspicker.ScanProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}

func wireSkillResource(resource skillsource.Resource) skillspicker.Resource {
	wire := skillspicker.Resource{Path: resource.Path, Executable: resource.Executable}
	if resource.Size != nil {
		size := int(*resource.Size)
		wire.Size = &size
	}
	return wire
}
