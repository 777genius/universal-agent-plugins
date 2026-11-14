package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// The executable composition owns the OS detector and its explicit registry.
// Private profile facts are never reconstructed from public display paths.
func runManagementDiscovery(ctx context.Context, app App, output io.Writer) error {
	if app.scope() != domain.ScopeUser || app.Detector == nil || app.ClientRegistry == nil || app.Installer == nil {
		return errors.New("passive user-profile discovery is unavailable")
	}
	clients, err := app.Detector.Detect(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("client profiles could not be inspected safely")
	}
	result := management.TargetDiscovery{ApiVersion: management.N1, EngineVersion: app.Version, Targets: []management.DiscoveredTarget{}}
	for _, client := range clients {
		definition, defined := domain.ClientDefinitionFor(client.ClientID)
		if _, registered := app.ClientRegistry.Lookup(client.ClientID); !defined || !registered {
			continue
		}
		target := management.DiscoveredTarget{Client: string(client.ClientID), Label: definition.DisplayName, DetectionStatus: management.ClientNotDetected}
		if client.Status == domain.DetectionDetected {
			target.DetectionStatus = management.ClientDetected
		}
		switch {
		case !installer.ClientScopeSupported(app.ClientRegistry, target.Client, domain.ScopeUser):
			target.Reason = "This client is not qualified for managed user-scope installation"
		case client.DetectionError != nil:
			target.Reason = "Client configuration could not be inspected safely"
		default:
			profile := management.StartupTarget{Id: target.Client, Client: target.Client, DetectionStatus: target.DetectionStatus}
			profile.ConfigRoot, err = managementAbsolute(client.ConfigRoot)
			if err == nil && client.ExecutablePath != "" {
				profile.Executable, err = managementAbsolute(client.ExecutablePath)
			}
			if err != nil {
				target.Reason = "Client configuration could not be inspected safely"
				break
			}
			target.Profile = &profile
			target.Reason = managementTargetReason(profile, app.Installer.RequiresClientExecutable(target.Client))
			target.Available = target.Reason == ""
		}
		result.Targets = append(result.Targets, target)
	}
	sort.Slice(result.Targets, func(i, j int) bool { return result.Targets[i].Client < result.Targets[j].Client })
	body, err := json.Marshal(result)
	if err != nil || len(body)+1 > 64*1024 || len(result.Targets) > 32 {
		return errors.New("client discovery exceeds the supported response bounds")
	}
	_, err = output.Write(append(body, '\n'))
	return err
}
