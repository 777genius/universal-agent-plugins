package generate

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
