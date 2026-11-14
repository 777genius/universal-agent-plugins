package codex

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) Inspect(_ context.Context, in ports.InspectInput) (ports.InspectResult, error) {
	inputs := a.inspectInputs(in)
	surface := observeCodexSurface(inputs.Scope, inputs.Paths)
	details := a.inspectDetails(inputs)
	lifecycle := classifyCodexLifecycle(surface, details)

	return ports.InspectResult{
		TargetID:                a.ID(),
		Installed:               details.CacheExists || (surface.CatalogFound && surface.PluginFound) || details.ConfigState.Present,
		State:                   lifecycle.State,
		ActivationState:         lifecycle.Activation,
		CatalogPolicy:           details.CatalogPolicy,
		ConfigPrecedenceContext: []string{"repo_marketplace", "personal_marketplace", "cache", "config"},
		EnvironmentRestrictions: lifecycle.Restrictions,
		ObservedNativeObjects:   append(surface.Observed, details.Observed...),
		SettingsFiles:           []string{inputs.Paths.CatalogPath, inputs.Paths.ConfigPath},
		Warnings:                lifecycle.Warnings,
		EvidenceClass:           domain.EvidenceConfirmed,
	}, nil
}
