package codex

import "github.com/777genius/plugin-kit-ai/install/integrationctl/domain"

type codexLifecycle struct {
	State        domain.InstallState
	Activation   domain.ActivationState
	Restrictions []domain.EnvironmentRestrictionCode
	Warnings     []string
}

func classifyCodexLifecycle(surface codexObservedSurface, details codexInspectDetails) codexLifecycle {
	restrictions := append([]domain.EnvironmentRestrictionCode{}, surface.Restrictions...)
	warnings := append([]string{}, details.Warnings...)

	preparedExists := details.EntryFound && surface.PluginFound
	partialPrepared := details.EntryFound || surface.PluginFound
	state := domain.InstallRemoved
	activation := domain.ActivationNotRequired

	switch {
	case details.CacheExists && details.ConfigState.Present && details.ConfigState.Disabled:
		state = domain.InstallDisabled
		activation = domain.ActivationComplete
	case details.CacheExists:
		if !surface.CatalogFound || !surface.PluginFound {
			warnings = append(warnings, "Codex installed cache bundle exists but managed marketplace source is missing or drifted")
			state = domain.InstallDegraded
		} else {
			state = domain.InstallInstalled
		}
		activation = domain.ActivationComplete
	case preparedExists:
		state = domain.InstallActivationPending
		activation = domain.ActivationNativePending
		restrictions = append(restrictions, domain.RestrictionNativeActivation, domain.RestrictionNewThreadRequired)
	case partialPrepared:
		state = domain.InstallDegraded
		activation = domain.ActivationNativePending
		restrictions = append(restrictions, domain.RestrictionNativeActivation)
	}

	return codexLifecycle{
		State:        state,
		Activation:   activation,
		Restrictions: restrictions,
		Warnings:     warnings,
	}
}
