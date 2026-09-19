package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
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

// PurgeRetainedData handles the only target-less removal: an explicit purge of
// a data_retained installation after complete ownership preflight.
func (service Service) PurgeRetainedData(ctx context.Context, selector string, confirmed bool) error {
	if service.StateStore == nil || service.PluginData == nil {
		return fmt.Errorf("state and PLUGIN_DATA managers are required")
	}
	release, err := service.beginMutation(ctx, false, confirmed)
	if err != nil {
		return err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	state, index, installation, err := service.loadRetainedPurgeTarget(selector)
	if err != nil {
		return err
	}
	if err := service.validateOwnedDataReceipts(ctx, installation); err != nil {
		return err
	}
	if !confirmed {
		return nil
	}
	return service.commitRetainedDataPurge(ctx, state, index, installation)
}

func (service Service) loadRetainedPurgeTarget(selector string) (domain.StateFileV2, int, domain.Installation, error) {
	state, err := service.StateStore.Load()
	if err != nil {
		return domain.StateFileV2{}, -1, domain.Installation{}, err
	}
	index, installation, err := findInstallation(state, selector)
	if err != nil {
		return domain.StateFileV2{}, -1, domain.Installation{}, err
	}
	if !installation.DataRetained || len(installation.Clients) != 0 {
		return domain.StateFileV2{}, -1, domain.Installation{}, fmt.Errorf("installation is not a data_retained record")
	}
	return state, index, installation, nil
}

func (service Service) validateOwnedDataReceipts(ctx context.Context, installation domain.Installation) error {
	for _, receipt := range installation.DataReceipts {
		if receipt.State != domain.DataReceiptOwned {
			return fmt.Errorf("PLUGIN_DATA receipt %s is not safely owned", receipt.DataReceiptID)
		}
		if err := service.PluginData.ValidateData(ctx, receipt); err != nil {
			return err
		}
	}
	return nil
}

func (service Service) commitRetainedDataPurge(ctx context.Context, state domain.StateFileV2, index int, installation domain.Installation) error {
	operationID, err := newOperationID()
	if err != nil {
		return err
	}
	if len(installation.InstallPreferences) > 0 {
		retained := installation
		retained.DataRetained = false
		retained.DataReceipts = nil
		state.Installations[index] = retained
	} else {
		state.Installations = append(state.Installations[:index], state.Installations[index+1:]...)
	}
	receipts := sortedDataReceipts(installation.DataReceipts)
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	_, err = kernel.RemoveDirectoryGroup(ctx, transaction.DirectoryRemovalGroup{OperationGroupID: operationID, Removals: dataReceiptRemovals(service, operationID, receipts), DesiredState: state})
	return err
}

func (service Service) validatePurgeReceipts(ctx context.Context, purge bool, installation domain.Installation) error {
	if !purge {
		return nil
	}
	if service.PluginData == nil {
		return fmt.Errorf("PLUGIN_DATA manager is required for purge")
	}
	for _, receipt := range installation.DataReceipts {
		if receipt.State != domain.DataReceiptOwned {
			return fmt.Errorf("PLUGIN_DATA receipt %s is not safely owned; purge aborted before mutation", receipt.DataReceiptID)
		}
		if err := service.PluginData.ValidateData(ctx, receipt); err != nil {
			return fmt.Errorf("validate all PLUGIN_DATA receipts before purge: %w", err)
		}
	}
	return nil
}

func validatePurgeCoversActiveBindings(purge bool, installation domain.Installation, clientKey string) error {
	if !purge {
		return nil
	}
	for key, binding := range installation.Clients {
		if key != clientKey && binding.Materialization != domain.MaterializationAbsent {
			return fmt.Errorf("--purge-data requires ownership-preflight and removal of every active target")
		}
	}
	return nil
}

func sortedDataReceipts(receipts map[string]domain.DataReceipt) []domain.DataReceipt {
	values := make([]domain.DataReceipt, 0, len(receipts))
	for _, receipt := range receipts {
		values = append(values, receipt)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].DataReceiptID < values[j].DataReceiptID })
	return values
}

func dataReceiptRemovals(service Service, operationID string, receipts []domain.DataReceipt) []transaction.DirectoryRemoval {
	removals := make([]transaction.DirectoryRemoval, 0, len(receipts))
	for index, dataReceipt := range receipts {
		receipt := dataReceipt
		removals = append(removals, transaction.DirectoryRemoval{OperationID: fmt.Sprintf("%s-data-%03d", operationID, index+1),
			OperationGroupID: operationID, ClientBindingID: receipt.DataReceiptID, Sequence: 1,
			OwnedBase: filepath.Dir(receipt.Locator), ActivePath: receipt.Locator, BeforeDigest: receipt.OwnershipDigest, Standalone: true,
			Verify: func(verifyContext context.Context, _ string) error {
				return service.PluginData.ValidateData(verifyContext, receipt)
			}})
	}
	return removals
}
