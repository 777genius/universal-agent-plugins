package usecase

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func doctorTargetNeedsAttention(ti domain.TargetInstallation) bool {
	if ti.State == domain.InstallDegraded || ti.State == domain.InstallActivationPending || ti.State == domain.InstallAuthPending {
		return true
	}
	switch ti.ActivationState {
	case domain.ActivationNativePending, domain.ActivationReloadPending, domain.ActivationRestartPending, domain.ActivationNewThreadPending:
		return true
	}
	return false
}

func doctorManualSteps(integrationID string, ti domain.TargetInstallation) []string {
	steps := doctorStateManualSteps(integrationID, ti)
	for _, restriction := range ti.EnvironmentRestrictions {
		steps = append(steps, doctorRestrictionManualStep(restriction)...)
	}
	return dedupeStrings(steps)
}

func doctorStateManualSteps(integrationID string, ti domain.TargetInstallation) []string {
	switch ti.State {
	case domain.InstallDegraded:
		return []string{"run plugin-kit-ai integrations repair " + integrationID}
	case domain.InstallActivationPending:
		return []string{"complete the vendor-native activation step for this target"}
	case domain.InstallAuthPending:
		return []string{"complete the required authentication flow for this target"}
	default:
		return nil
	}
}

func doctorRestrictionManualStep(restriction domain.EnvironmentRestrictionCode) []string {
	switch restriction {
	case domain.RestrictionNewThreadRequired:
		return []string{"start a new agent thread before using the integration"}
	case domain.RestrictionReloadRequired:
		return []string{"reload the current agent session to pick up the integration"}
	case domain.RestrictionRestartRequired:
		return []string{"restart the agent CLI or desktop app"}
	case domain.RestrictionNativeActivation:
		return []string{"finish the native activation flow in the target agent"}
	case domain.RestrictionNativeAuthRequired, domain.RestrictionSourceAuthRequired:
		return []string{"complete the missing authentication step and rerun repair"}
	default:
		return nil
	}
}

func dedupeStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
