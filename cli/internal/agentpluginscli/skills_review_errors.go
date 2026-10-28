package agentpluginscli

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/skillspicker"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func (s *skillsCatalogSession) reviewRefusal(err error) error {
	refusal := &skillspicker.ReviewRefusal{Code: "PLAN_REJECTED", Message: "Review one Skill and target at a time to identify the refused selection, or inspect agentplugins deps status for recovery guidance"}
	switch {
	case errors.Is(err, installer.ErrRecoveryRequired):
		refusal.Code = "RECOVERY_REQUIRED"
		refusal.Message = "The environment has an unfinished operation. Run agentplugins deps status and follow its recovery instructions before reviewing again."
	case errors.Is(err, installer.ErrAssessmentRejected):
		refusal.Code = "SECURITY_REVIEW_BLOCKED"
		refusal.Message = "Automated security assessment blocked this selection. Remove this Skill or inspect its security findings before reviewing again."
	case errors.Is(err, installer.ErrTargetFactsUnavailable), errors.Is(err, installer.ErrCompatibilityUnavailable):
		refusal.Code = "TARGET_UNAVAILABLE"
		refusal.Message = "A selected client's configuration or compatibility could not be verified. Choose another target or correct its configuration before reviewing again."
	case errors.Is(err, installer.ErrUnsupported), errors.Is(err, installer.ErrIncomplete):
		refusal.Code = "TARGET_UNSUPPORTED"
		refusal.Message = "A selected target cannot install all required components. Remove that target or choose a compatible Skill."
	case errors.Is(err, installer.ErrUpdateRequired):
		refusal.Code = "UPDATE_REQUIRED"
		refusal.Message = "An existing installation requires an explicit update. Use agentplugins update for the saved dependency before reviewing again."
	case errors.Is(err, installer.ErrPlanChanged), errors.Is(err, ErrSkillsPlanStale):
		refusal.Code = "ENVIRONMENT_CHANGED"
		refusal.Message = "The environment changed during review. Refresh the selection and prepare a new review."
	case errors.Is(err, context.DeadlineExceeded):
		refusal.Code = "REVIEW_TIMEOUT"
		// Preparation has no aggregate review deadline: a bounded source or
		// client check can expire even for one Skill. Do not imply that reducing
		// the selection fixes a slow or unavailable check.
		refusal.Message = "A source or client check timed out while preparing the review. Check network access and the selected clients' configuration, then prepare a new review."
		refusal.Retryable = true
	}
	var selection *SkillsSelectionError
	if errors.As(err, &selection) {
		s.addCandidateRefusalContext(refusal, selection.CandidateID)
	}
	return refusal
}
func (s *skillsCatalogSession) addCandidateRefusalContext(refusal *skillspicker.ReviewRefusal, id string) {
	s.mu.Lock()
	candidate, ok := s.candidates[id]
	s.mu.Unlock()
	if !ok {
		return
	}
	if refusal.Code == "PLAN_REJECTED" {
		refusal.Code = "SKILL_REJECTED"
		refusal.Message = "The chosen Skill could not be acquired or validated. Remove it and prepare a new review with the remaining Skills."
	}
	refusal.Message = fmt.Sprintf("Skill %q (%s): %s", prompt.SafeText(candidate.Name), prompt.SafeText(candidate.Path), refusal.Message)
}

// Installer reasons may contain underlying provider output. Only these stable
// codes affect public guidance; arbitrary error strings never cross HTTP.
func skillOperationMessage(status skillspicker.OperationItemStatus, reason string) string {
	switch reason {
	case "plan_changed":
		return "Installer facts changed. Prepare a new review before retrying."
	case "target_facts_unavailable":
		return "The target configuration could not be verified. Correct it before preparing a new review."
	case "recovery_required":
		return "The environment requires recovery. Inspect agentplugins deps status and follow its instructions."
	case "assessment_rejected":
		return "Security assessment blocked this item. Inspect the findings before preparing a new review."
	}
	switch status {
	case skillspicker.OperationItemStatusSucceeded:
		return "The Skill was installed."
	case skillspicker.OperationItemStatusUnchanged:
		return "The pinned Skill already matches."
	case skillspicker.OperationItemStatusSkipped:
		return "This item was not attempted."
	default:
		return "Installation stopped. Follow the operation's recovery instructions before retrying."
	}
}
