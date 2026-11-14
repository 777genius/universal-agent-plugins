package cursor

import (
	"context"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) Inspect(ctx context.Context, in ports.InspectInput) (ports.InspectResult, error) {
	config := a.targetConfigPath(in.Scope, workspaceRootFromInspectInput(in))
	observed := []domain.NativeObjectRef{}
	if in.Record != nil {
		if target, ok := in.Record.Targets[domain.TargetCursor]; ok {
			config = configPathFromTarget(target, config)
		}
	}
	_, statErr := os.Stat(config)
	restrictions := []domain.EnvironmentRestrictionCode{}
	state := domain.InstallRemoved
	if in.Record != nil {
		if target, ok := in.Record.Targets[domain.TargetCursor]; ok {
			if pluginRoot := pluginRootFromTarget(target, ""); pluginRoot != "" {
				info, err := a.fs().Stat(ctx, pluginRoot)
				if err != nil {
					return ports.InspectResult{}, err
				}
				if info.Exists && info.IsDir {
					observed = append(observed, domain.NativeObjectRef{Kind: cursorPluginRootKind, Name: target.NativeRef, Path: pluginRoot})
					return ports.InspectResult{
						TargetID:                a.ID(),
						Installed:               true,
						State:                   domain.InstallInstalled,
						ActivationState:         domain.ActivationNotRequired,
						ConfigPrecedenceContext: []string{"cursor_local_plugins", "project", "global", "parent_discovery"},
						EnvironmentRestrictions: restrictions,
						ObservedNativeObjects:   observed,
						SettingsFiles:           []string{filepath.Join(pluginRoot, cursorPluginManifestRelPath)},
						EvidenceClass:           domain.EvidenceConfirmed,
					}, nil
				}
				return ports.InspectResult{
					TargetID:                a.ID(),
					Installed:               false,
					State:                   domain.InstallRemoved,
					ActivationState:         domain.ActivationNotRequired,
					ConfigPrecedenceContext: []string{"cursor_local_plugins", "project", "global", "parent_discovery"},
					EnvironmentRestrictions: restrictions,
					SettingsFiles:           []string{filepath.Join(pluginRoot, cursorPluginManifestRelPath)},
					EvidenceClass:           domain.EvidenceConfirmed,
				}, nil
			}
			aliases := ownedAliases(target.OwnedNativeObjects)
			if len(aliases) > 0 && statErr == nil {
				doc, _, _, err := a.readDocument(ctx, config)
				if err != nil {
					return ports.InspectResult{}, err
				}
				present := false
				for _, alias := range aliases {
					if _, ok := doc[alias]; ok {
						present = true
						observed = append(observed, domain.NativeObjectRef{Kind: "cursor_mcp_server", Name: alias, Path: config})
					}
				}
				if present {
					state = domain.InstallInstalled
				} else {
					state = domain.InstallRemoved
				}
				return ports.InspectResult{
					TargetID:                a.ID(),
					Installed:               present,
					State:                   state,
					ActivationState:         domain.ActivationNotRequired,
					ConfigPrecedenceContext: []string{"project", "global", "parent_discovery"},
					EnvironmentRestrictions: restrictions,
					ObservedNativeObjects:   observed,
					SettingsFiles:           []string{config},
					EvidenceClass:           domain.EvidenceConfirmed,
				}, nil
			}
		}
	}
	if statErr == nil {
		state = domain.InstallInstalled
	}
	return ports.InspectResult{
		TargetID:                a.ID(),
		Installed:               statErr == nil,
		State:                   state,
		ActivationState:         domain.ActivationNotRequired,
		ConfigPrecedenceContext: []string{"project", "global", "parent_discovery"},
		EnvironmentRestrictions: restrictions,
		ObservedNativeObjects:   observed,
		SettingsFiles:           []string{config},
		EvidenceClass:           domain.EvidenceConfirmed,
	}, nil
}
