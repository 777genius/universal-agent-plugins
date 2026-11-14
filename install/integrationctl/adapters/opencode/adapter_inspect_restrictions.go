package opencode

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func planBlockingManualSteps(inspect ports.InspectResult) ([]string, bool) {
	steps := []string{}
	blocking := false
	for _, restriction := range inspect.EnvironmentRestrictions {
		if restriction == domain.RestrictionReadOnlyNativeLayer {
			steps = append(steps,
				"OpenCode managed config is active at a higher-precedence system layer",
				"ask an administrator to update or remove the managed OpenCode config before mutating this integration",
			)
			blocking = true
			break
		}
	}
	return dedupeStrings(steps), blocking
}

func dedupeRestrictionCodes(values []domain.EnvironmentRestrictionCode) []domain.EnvironmentRestrictionCode {
	if len(values) == 0 {
		return nil
	}
	seen := map[domain.EnvironmentRestrictionCode]struct{}{}
	out := make([]domain.EnvironmentRestrictionCode, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
