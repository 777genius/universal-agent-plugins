package opencode

import (
	"context"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) inspectTrackedConfig(ctx context.Context, target domain.TargetInstallation, config string, surface inspectSurface) (ports.InspectResult, error) {
	body, err := a.fs().ReadFile(ctx, config)
	if err != nil {
		return ports.InspectResult{}, domain.NewError(domain.ErrMutationApply, "read OpenCode config during inspect", err)
	}
	doc, err := decodeConfigMap(body)
	if err != nil {
		return ports.InspectResult{}, domain.NewError(domain.ErrMutationApply, "parse OpenCode config during inspect", err)
	}
	present, observed, err := inspectTrackedObjects(doc, config, target)
	if err != nil {
		return ports.InspectResult{}, err
	}
	_, restrictions := statelessRestrictions(surface, config)
	return buildInspectResult(surface, observed, restrictions, present), nil
}

func inspectTrackedObjects(doc map[string]any, config string, target domain.TargetInstallation) (bool, []domain.NativeObjectRef, error) {
	var observed []domain.NativeObjectRef
	present := false

	plugins, err := existingPluginRefs(doc["plugin"])
	if err != nil {
		return false, nil, domain.NewError(domain.ErrMutationApply, "parse OpenCode plugin refs during inspect", err)
	}
	for _, ref := range ownedPluginRefs(target) {
		if _, ok := plugins[ref]; ok {
			present = true
			observed = append(observed, domain.NativeObjectRef{Kind: "opencode_plugin_ref", Name: ref, Path: config})
		}
	}

	mcp, err := existingObjectMap(doc["mcp"], "mcp")
	if err != nil {
		return false, nil, domain.NewError(domain.ErrMutationApply, "parse OpenCode MCP config during inspect", err)
	}
	for _, alias := range ownedMCPAliases(target) {
		if _, ok := mcp[alias]; ok {
			present = true
			observed = append(observed, domain.NativeObjectRef{Kind: "opencode_mcp_server", Name: alias, Path: config})
		}
	}

	for _, key := range ownedConfigKeys(target) {
		if _, ok := doc[key]; ok {
			present = true
			observed = append(observed, domain.NativeObjectRef{Kind: "opencode_config_key", Name: key, Path: config})
		}
	}
	return present, observed, nil
}

func (a Adapter) inspectSurfaceState(config string, surface inspectSurface) ports.InspectResult {
	present, restrictions := statelessRestrictions(surface, config)
	return buildInspectResult(surface, nil, restrictions, present)
}

func statelessRestrictions(surface inspectSurface, config string) (bool, []domain.EnvironmentRestrictionCode) {
	restrictions := append([]domain.EnvironmentRestrictionCode(nil), surface.EnvironmentRestrictions...)
	_, cmdErr := execLookPath("opencode")
	_, statErr := os.Stat(config)
	if cmdErr != nil && statErr != nil {
		restrictions = append(restrictions, domain.RestrictionSourceToolMissing)
	}
	return statErr == nil, restrictions
}

func buildInspectResult(surface inspectSurface, observed []domain.NativeObjectRef, restrictions []domain.EnvironmentRestrictionCode, present bool) ports.InspectResult {
	state := domain.InstallRemoved
	if present {
		state = domain.InstallInstalled
	}
	return ports.InspectResult{
		TargetID:                 domain.TargetOpenCode,
		Installed:                present,
		State:                    state,
		ActivationState:          domain.ActivationRestartPending,
		ConfigPrecedenceContext:  surface.ConfigPrecedenceContext,
		EnvironmentRestrictions:  restrictions,
		VolatileOverrideDetected: surface.VolatileOverride,
		ObservedNativeObjects:    observed,
		SourceAccessState:        surface.SourceAccessState,
		SettingsFiles:            append([]string(nil), surface.SettingsFiles...),
		EvidenceClass:            domain.EvidenceConfirmed,
	}
}
