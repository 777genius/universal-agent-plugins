package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

// prepareRemovedBindingsState computes the final binding/data state before any
// directory is renamed. That exact state is the kernel's commit decision.
func (service Service) prepareRemovedBindingsState(ctx context.Context, state domain.StateFileV2, installationIndex int, clientKeys []string, purge bool, groupID string) (domain.StateFileV2, []domain.DataReceipt, []domain.DataReceipt, error) {
	installation := state.Installations[installationIndex]
	removed, err := detachRemovedBindings(&installation, clientKeys)
	if err != nil {
		return state, nil, nil, err
	}
	if purge && countActiveBindings(installation) != 0 {
		return state, nil, nil, fmt.Errorf("--purge-data requires ownership-preflight and removal of every active target")
	}
	if countActiveBindings(installation) == 0 {
		return service.finalizeLastBindingState(ctx, state, installationIndex, installation, removed, purge, groupID)
	}
	installation.OperationGroupID = groupID
	installation.UpdatedAt = service.now().Format(time.RFC3339Nano)
	state.Installations[installationIndex] = installation
	return state, nil, nil, nil
}

func detachRemovedBindings(installation *domain.Installation, clientKeys []string) ([]domain.ClientBinding, error) {
	removed := make([]domain.ClientBinding, 0, len(clientKeys))
	for _, clientKey := range clientKeys {
		client, ok := installation.Clients[clientKey]
		if !ok || client.Materialization == domain.MaterializationAbsent {
			return nil, fmt.Errorf("removed binding state is not authoritative")
		}
		removed = append(removed, client)
		recordRemovedInstallPreference(installation, client)
		delete(installation.Clients, clientKey)
	}
	return removed, nil
}

func recordRemovedInstallPreference(installation *domain.Installation, client domain.ClientBinding) {
	if client.InstallIntent == domain.InstallIntentAutomatic {
		return
	}
	preference := domain.InstallPreference{ClientID: domain.ClientID(client.ClientID), Scope: domain.InstallScope(client.Scope), InstallIntent: client.InstallIntent}
	found := false
	for i, previous := range installation.InstallPreferences {
		if previous.ClientID == preference.ClientID && previous.Scope == preference.Scope {
			installation.InstallPreferences[i] = preference
			found = true
		}
	}
	if !found {
		installation.InstallPreferences = append(installation.InstallPreferences, preference)
	}
}

func countActiveBindings(installation domain.Installation) int {
	active := 0
	for _, binding := range installation.Clients {
		if binding.Materialization != domain.MaterializationAbsent {
			active++
		}
	}
	return active
}

func (service Service) finalizeLastBindingState(ctx context.Context, state domain.StateFileV2, installationIndex int, installation domain.Installation, removed []domain.ClientBinding, purge bool, groupID string) (domain.StateFileV2, []domain.DataReceipt, []domain.DataReceipt, error) {
	for key := range installation.Clients {
		delete(installation.Clients, key)
	}
	createdData := []domain.DataReceipt(nil)
	if !purge && len(installation.DataReceipts) == 0 {
		var err error
		createdData, err = service.ensureRetainedPluginData(ctx, &installation, removed)
		if err != nil {
			return state, nil, nil, err
		}
	}
	if purge {
		return applyPurgedInstallation(state, installationIndex, installation, createdData)
	}
	installation.DataRetained = true
	installation.OperationGroupID = groupID
	installation.UpdatedAt = service.now().Format(time.RFC3339Nano)
	state.Installations[installationIndex] = installation
	return state, nil, createdData, nil
}

func (service Service) ensureRetainedPluginData(ctx context.Context, installation *domain.Installation, removed []domain.ClientBinding) ([]domain.DataReceipt, error) {
	if service.PluginData == nil {
		return nil, fmt.Errorf("PLUGIN_DATA manager is required to retain final-binding data")
	}
	createdData := []domain.DataReceipt(nil)
	for _, client := range removed {
		receipt, created, err := service.PluginData.EnsureData(ctx, installation.InstallationID, client.PhysicalArtifact, client.Scope)
		if err != nil {
			service.cleanupUncommittedPluginData(createdData, transaction.GroupFailureUnchanged)
			return nil, fmt.Errorf("establish retained PLUGIN_DATA ownership: %w", err)
		}
		if created {
			createdData = append(createdData, receipt)
		}
		if installation.DataReceipts == nil {
			installation.DataReceipts = map[string]domain.DataReceipt{}
		}
		installation.DataReceipts[receipt.DataReceiptID] = receipt
	}
	return createdData, nil
}

func applyPurgedInstallation(state domain.StateFileV2, installationIndex int, installation domain.Installation, createdData []domain.DataReceipt) (domain.StateFileV2, []domain.DataReceipt, []domain.DataReceipt, error) {
	receipts := sortedDataReceipts(installation.DataReceipts)
	if len(installation.InstallPreferences) > 0 {
		installation.DataRetained = false
		installation.DataReceipts = nil
		state.Installations[installationIndex] = installation
	} else {
		state.Installations = append(state.Installations[:installationIndex], state.Installations[installationIndex+1:]...)
	}
	return state, receipts, createdData, nil
}
