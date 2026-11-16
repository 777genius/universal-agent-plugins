package prompt

import "context"

// SkillsSelector is an optional rich-terminal capability. The legacy numeric
// catalog remains available to plain and embedded prompters.
type SkillsSelector interface {
	SelectSkills(context.Context, SkillsSelectionRequest) (SkillsSelectionResult, error)
}

type SkillChoice struct{ ID, Label string }
type SkillsSelectionRequest struct{ Choices []SkillChoice }
type SkillsSelectionResult struct{ IDs []string }
