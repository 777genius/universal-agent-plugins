package agentpluginscli

import (
	"context"
	"fmt"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Native User siblings participate in compatibility planning without becoming
// mutation targets. Obtain only their adapter-required missing version, using
// the existing identity-bound targeted probe rather than persisted evidence.
func (adapter *dependencyInstaller) ensureDependencySiblingVersions(ctx context.Context, view installer.Inspection, installationID, selectedClientID string, operation installer.Operation) error {
	if adapter.app.scope() != domain.ScopeUser || operation == installer.OpRemove {
		return nil
	}
	var targets []domain.ClientID
	for _, installation := range view.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		for _, binding := range installation.Bindings {
			if binding.ClientID == selectedClientID {
				continue
			}
			sibling, ok := adapter.detected[domain.ClientID(binding.ClientID)]
			if !ok {
				return fmt.Errorf("installed sibling %s is not detected", binding.ClientID)
			}
			if dependencyNativeConfigRoot(adapter.app, sibling) != "" {
				targets = append(targets, sibling.ClientID)
			}
		}
	}
	if len(targets) == 0 {
		return nil
	}
	return ensureSelectedProjectVersions(ctx, adapter.app, adapter.opts, targets, adapter.detected)
}

func (adapter *dependencyInstaller) populateDependencyTargets(view installer.Inspection, installationID string, request *installer.Request) error {
	for _, installation := range view.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		for _, binding := range installation.Bindings {
			if binding.ClientID == request.ClientID {
				continue
			}
			sibling, ok := adapter.detected[domain.ClientID(binding.ClientID)]
			if !ok {
				return fmt.Errorf("installed sibling %s is not detected", binding.ClientID)
			}
			request.KnownTargets = append(request.KnownTargets, installer.TargetFacts{ClientID: binding.ClientID, BindingID: binding.BindingID, ConfigRoot: sibling.ConfigRoot, NativeConfigRoot: dependencyNativeConfigRoot(adapter.app, sibling), Executable: backendExecutable(sibling, adapter.detected, adapter.app.scope()), Version: sibling.Version, Surfaces: slices.Clone(sibling.Surfaces)})
		}
	}
	return nil
}
