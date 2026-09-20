package app

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/skills/domain"
)

func compatibilitySummary(spec domain.CompatibilitySpec) []string {
	var out []string
	if len(spec.Requires) > 0 {
		out = append(out, "Requires: "+strings.Join(spec.Requires, ", "))
	}
	if len(spec.SupportedOS) > 0 {
		out = append(out, "Supported OS: "+strings.Join(spec.SupportedOS, ", "))
	}
	if spec.RepoRequired {
		out = append(out, "Requires a repository checkout")
	}
	if spec.NetworkRequired {
		out = append(out, "May require network access")
	}
	out = append(out, spec.Notes...)
	return out
}

func executionNotes(spec domain.SkillSpec) []string {
	var out []string
	if spec.SafeToRetry != nil {
		out = append(out, "Safe to retry: "+yesNo(*spec.SafeToRetry))
	}
	if spec.WritesFiles != nil {
		out = append(out, "Writes files: "+yesNo(*spec.WritesFiles))
	}
	if spec.ProducesJSON != nil {
		out = append(out, "Produces JSON: "+yesNo(*spec.ProducesJSON))
	}
	return out
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
