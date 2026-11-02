package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

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
		owners := service.dataProfileOwners(receipt.DataReceiptID)
		installationID := ""
		if len(owners) > 0 {
			installationID = owners[0].InstallationID
		}
		removals = append(removals, transaction.DirectoryRemoval{OperationID: fmt.Sprintf("%s-data-%03d", operationID, index+1),
			OperationGroupID: operationID, InstallationID: installationID, DataReceiptID: receipt.DataReceiptID, ProfileOwners: owners, ClientBindingID: receipt.DataReceiptID, Sequence: 1,
			OwnedBase: filepath.Dir(receipt.Locator), ActivePath: receipt.Locator, BeforeDigest: receipt.OwnershipDigest, Standalone: true,
			Verify: func(verifyContext context.Context, path string) error {
				return service.PluginData.ValidateDataAt(verifyContext, receipt, path)
			}})
	}
	return removals
}
