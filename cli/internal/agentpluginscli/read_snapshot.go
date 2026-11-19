package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type doctorReport struct {
	ReadOnly           bool                     `json:"read_only"`
	ToolVersion        string                   `json:"tool_version"`
	SupportedClients   []supportedClient        `json:"supported_clients"`
	Clients            []publicDetectedClient   `json:"clients"`
	InstallationCount  int                      `json:"installation_count"`
	OpenOperationCount int                      `json:"open_operation_count"`
	Installation       *publicInstallation      `json:"installation,omitempty"`
	ProjectReadiness   []projectReadinessRecord `json:"project_readiness"`
	Findings           []doctorFinding          `json:"findings"`
}

// Only ancillary mutex metadata changes: doctor never repairs or persists state.
// Keep local integrity observations in the same interval as state and journals;
// release before optional Directory requests and potentially blocked output.
func collectDoctorSnapshot(ctx context.Context, app App, clients []domain.DetectedClient, args []string) (report doctorReport, state domain.StateFileV2, selected *domain.Installation, resultErr error) {
	unlock, err := acquireDoctorLock(ctx, app.Lifecycle.Lock)
	if err != nil {
		return report, state, nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, unlock()) }()
	state, err = app.StateStore.Load()
	if err != nil {
		return report, state, nil, err
	}
	open, err := app.Lifecycle.Kernel.Directory.ListOpen()
	if err != nil {
		return report, state, nil, err
	}
	if len(args) == 1 {
		installation, err := selectInstallation(state, args[0])
		if err != nil {
			return report, state, nil, err
		}
		selected = &installation
	}
	report = doctorReport{ReadOnly: true, ToolVersion: normalizedToolVersion(app.Version),
		SupportedClients: doctorSupportedClients(), Clients: doctorPublicClients(clients),
		InstallationCount:  len(publicInstallations(state, false)),
		OpenOperationCount: len(open) + len(unresolvedNativeAttempts(app, state)),
		ProjectReadiness:   projectReadinessRecords(state, selected),
		Findings:           doctorFindings(ctx, app, clients, state, open, selected)}
	report.Findings = append(report.Findings, doctorCodexNativeFindings(state, selected)...)
	return report, state, selected, nil
}

func doctorPublicClients(clients []domain.DetectedClient) []publicDetectedClient {
	result := make([]publicDetectedClient, 0, len(clients))
	for _, client := range clients {
		result = append(result, publicDetectedClient{
			ClientID: client.ClientID, DisplayName: client.DisplayName, Status: client.Status, Version: client.Version,
			Surfaces: append([]domain.ClientSurface(nil), client.Surfaces...),
		})
	}
	return result
}

func acquireDoctorLock(ctx context.Context, lock ports.MutationLock) (ports.UnlockFunc, error) {
	if lock == nil {
		return nil, fmt.Errorf("agentplugins mutation lock is required")
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for {
		if err := bounded.Err(); err != nil {
			return nil, err
		}
		unlock, err := lock.Acquire(bounded)
		if err == nil {
			if unlock == nil {
				return nil, fmt.Errorf("agentplugins mutation lock returned no release function")
			}
			if err := bounded.Err(); err != nil {
				return nil, errors.Join(err, unlock())
			}
			return unlock, nil
		}
		if !errors.Is(err, processlock.ErrActive) {
			return nil, err
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-bounded.Done():
			timer.Stop()
			return nil, bounded.Err()
		case <-timer.C:
		}
	}
}
