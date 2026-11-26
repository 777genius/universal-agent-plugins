package prompt

import (
	"context"
	"strings"
	"unicode"
)

// SkillsSelector is an optional rich-terminal capability. The legacy numeric
// catalog remains available to plain and embedded prompters.
type SkillsSelector interface {
	SelectSkills(context.Context, SkillsSelectionRequest) (SkillsSelectionResult, error)
}

type SkillChoice struct{ ID, Label string }
type SkillsSelectionRequest struct{ Choices []SkillChoice }
type SkillsSelectionResult struct{ IDs []string }

// SkillNavigationLabel sanitizes the complete label before middle elision. Deep
// path suffixes therefore remain distinguishable without raising display limits.
// Full authority paths belong in the installation review, not navigation labels.
func SkillNavigationLabel(label string) string {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, label)
	runes := []rune(cleaned)
	if len(runes) <= 128 {
		return cleaned
	}
	return string(runes[:63]) + "…" + string(runes[len(runes)-64:])
}
