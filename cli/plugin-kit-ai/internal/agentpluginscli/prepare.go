package agentpluginscli

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Retained preparation must not launch Kiro even for version discovery. Other
// clients may still supply their versions for normal Directory resolution.
func (app App) detectForLifecycle(ctx context.Context, probeVersion bool) ([]domain.DetectedClient, error) {
	if probeVersion && app.StateStore != nil {
		state, err := app.StateStore.Load()
		if err != nil {
			return nil, err
		}
		prepared := false
		for _, installation := range state.Installations {
			for _, binding := range installation.Clients {
				prepared = prepared || binding.InstallIntent == domain.InstallIntentPrepare
			}
			for _, preference := range installation.InstallPreferences {
				prepared = prepared || preference.InstallIntent == domain.InstallIntentPrepare
			}
		}
		if prepared {
			clients, err := app.Detector.Detect(ctx)
			if err != nil {
				return nil, err
			}
			var targets []domain.ClientID
			for _, client := range clients {
				if client.ClientID != domain.ClientKiro && client.Status == domain.DetectionDetected {
					targets = append(targets, client.ClientID)
				}
			}
			if targeted, ok := app.Detector.(ports.TargetedVersionProbingClientDetector); ok && len(targets) > 0 {
				probed, err := targeted.DetectTargetsWithVersionProbe(ctx, targets)
				if err != nil {
					return nil, err
				}
				for i, client := range clients {
					if client.ClientID == domain.ClientKiro {
						continue
					}
					for _, updated := range probed {
						if updated.ClientID == client.ClientID {
							clients[i] = updated
						}
					}
				}
			}
			return clients, nil
		}
	}
	return detectClientsForLifecycleResolution(ctx, app.Detector, probeVersion)
}

func automaticInstallTargets(targets []domain.ClientID, intents map[domain.ClientID]domain.InstallIntent) []domain.ClientID {
	var automatic []domain.ClientID
	for _, target := range targets {
		if intents[target] == domain.InstallIntentAutomatic {
			automatic = append(automatic, target)
		}
	}
	return automatic
}
