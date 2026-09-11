package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

type RemoveInput struct {
	Selector            string
	Client              domain.DetectedClient
	Scope               domain.InstallScope
	DryRun              bool
	Confirmed           bool
	Interactive         bool
	ExternalUninstalled bool
	OperationID         string
	BackendExecutable   string
	PurgeData           bool
}

type RemoveResult struct {
	InstallationID       string                     `json:"installation_id"`
	Plugin               string                     `json:"plugin"`
	ClientID             domain.ClientID            `json:"client_id"`
	RequiresConfirmation bool                       `json:"requires_confirmation"`
	Mutated              bool                       `json:"mutated"`
	Deactivation         domain.DeactivationOutcome `json:"deactivation,omitempty"`
	Receipt              domain.MutationReceipt     `json:"-"`
	AffectedSurfaces     []string                   `json:"affected_surfaces,omitempty"`
	GroupPhase           GroupTargetPhase           `json:"group_phase,omitempty"`
}

func (service Service) Remove(ctx context.Context, input RemoveInput) (RemoveResult, error) {
	if service.StateStore == nil || service.Targets == nil || service.Stager == nil || service.Activator == nil {
		return RemoveResult{}, fmt.Errorf("agentplugins service dependencies are incomplete")
	}
	release, err := service.beginMutation(ctx, input.DryRun, input.Confirmed)
	if err != nil {
		return RemoveResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	state, err := service.StateStore.Load()
	if err != nil {
		return RemoveResult{}, err
	}
	installationIndex, installation, err := findInstallation(state, input.Selector)
	if err != nil {
		return RemoveResult{}, err
	}
	if input.PurgeData {
		if service.PluginData == nil {
			return RemoveResult{}, fmt.Errorf("PLUGIN_DATA manager is required for purge")
		}
		for _, receipt := range installation.DataReceipts {
			if receipt.State != domain.DataReceiptOwned {
				return RemoveResult{}, fmt.Errorf("PLUGIN_DATA receipt %s is not safely owned; purge aborted before mutation", receipt.DataReceiptID)
			}
			if err := service.PluginData.ValidateData(ctx, receipt); err != nil {
				return RemoveResult{}, fmt.Errorf("validate all PLUGIN_DATA receipts before purge: %w", err)
			}
		}
	}
	clientKey, client, err := findClientBinding(installation, input.Client.ClientID, input.Scope)
	if err != nil {
		return RemoveResult{}, err
	}
	if input.PurgeData {
		for key, binding := range installation.Clients {
			if key != clientKey && binding.Materialization != domain.MaterializationAbsent {
				return RemoveResult{}, fmt.Errorf("--purge-data requires ownership-preflight and removal of every active target")
			}
		}
	}
	result := RemoveResult{
		InstallationID:   installation.InstallationID,
		Plugin:           installation.DeclaredName,
		ClientID:         input.Client.ClientID,
		AffectedSurfaces: preparedAffectedSurfaces(client, input.Client.ClientID),
	}
	if len(result.AffectedSurfaces) == 0 {
		result.AffectedSurfaces = []string{client.ClientID}
	}
	deactivation, err := service.Activator.Deactivate(ctx, domain.DeactivationRequest{
		Client: input.Client, DeclaredName: installation.DeclaredName,
		CurrentActivation: client.Activation, Interactive: input.Interactive,
		ExternalUninstalled: input.ExternalUninstalled,
		Confirmed:           input.Confirmed && !input.DryRun,
		PhysicalArtifactID:  client.PhysicalArtifact,
		BackendExecutable:   input.BackendExecutable,
		ManagedArtifactPath: client.TargetLocator,
		NativeObjects:       append([]domain.NativeObjectOwnership(nil), client.NativeObjects...),
	})
	result.Deactivation = deactivation
	if err != nil {
		return result, err
	}
	if input.DryRun {
		return result, nil
	}
	if !deactivation.ArtifactRemovalAllowed {
		return result, nil
	}
	if !input.Confirmed {
		result.RequiresConfirmation = true
		return result, nil
	}
	if deactivation.ExternalRemovalComplete && client.Activation == domain.ActivationActive {
		if err := service.markDeactivated(state, installationIndex, clientKey); err != nil {
			return result, err
		}
		state, err = service.StateStore.Load()
		if err != nil {
			return result, err
		}
		installation = state.Installations[installationIndex]
		client = installation.Clients[clientKey]
	}
	expectedDigest := managedDigest(client)
	if expectedDigest == "" {
		return result, fmt.Errorf("managed package digest is missing; refusing removal and retaining state for reviewed recovery")
	}
	target, err := service.Targets.ResolveTarget(ctx, input.Client, input.Scope, client.PhysicalArtifact)
	if err != nil {
		return result, fmt.Errorf("resolve managed removal target: %w", err)
	}
	if err := pathpolicy.RequireExactPath(target.ActivePath, client.TargetLocator); err != nil {
		return result, fmt.Errorf("refuse removal from untrusted persisted target: %w", err)
	}
	if err := service.Stager.Verify(ctx, client.TargetLocator, expectedDigest); err != nil {
		return result, fmt.Errorf("managed package was changed or is missing; refusing silent removal and retaining state: %w", err)
	}
	operationID := strings.TrimSpace(input.OperationID)
	if operationID == "" {
		operationID, err = newOperationID()
		if err != nil {
			return result, err
		}
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	desired, purgeReceipts, createdData, err := service.prepareRemovedBindingsState(ctx, state, installationIndex, []string{clientKey}, input.PurgeData, operationID)
	if err != nil {
		return result, err
	}
	removals := []transaction.DirectoryRemoval{{
		OperationID: operationID, InstallationID: installation.InstallationID,
		ClientBindingID: clientKey, Sequence: nextSequence(client),
		OwnedBase: target.TargetRoot, ActivePath: client.TargetLocator,
		BeforeDigest: expectedDigest,
		Verify: func(verifyContext context.Context, activePath string) error {
			return service.Stager.Verify(verifyContext, activePath, expectedDigest)
		},
	}}
	for index, dataReceipt := range purgeReceipts {
		receipt := dataReceipt
		removals = append(removals, transaction.DirectoryRemoval{OperationID: fmt.Sprintf("%s-data-%03d", operationID, index+1),
			OperationGroupID: operationID, ClientBindingID: receipt.DataReceiptID, Sequence: 1,
			OwnedBase: filepath.Dir(receipt.Locator), ActivePath: receipt.Locator, BeforeDigest: receipt.OwnershipDigest, Standalone: true,
			Verify: func(verifyContext context.Context, _ string) error {
				return service.PluginData.ValidateData(verifyContext, receipt)
			}})
	}
	receipts, err := kernel.RemoveDirectoryGroup(ctx, transaction.DirectoryRemovalGroup{OperationGroupID: operationID, Removals: removals, DesiredState: desired})
	if len(receipts) > 0 {
		result.Receipt = receipts[0]
	}
	if err != nil {
		service.cleanupUncommittedPluginData(createdData, transaction.FailurePhase(err))
		return result, err
	}
	result.Mutated = true
	return result, nil
}

// prepareRemovedBindingsState computes the final binding/data state before any
// directory is renamed. That exact state is the kernel's commit decision.
func (service Service) prepareRemovedBindingsState(ctx context.Context, state domain.StateFileV2, installationIndex int, clientKeys []string, purge bool, groupID string) (domain.StateFileV2, []domain.DataReceipt, []domain.DataReceipt, error) {
	installation := state.Installations[installationIndex]
	removed := make([]domain.ClientBinding, 0, len(clientKeys))
	for _, clientKey := range clientKeys {
		client, ok := installation.Clients[clientKey]
		if !ok || client.Materialization == domain.MaterializationAbsent {
			return state, nil, nil, fmt.Errorf("removed binding state is not authoritative")
		}
		removed = append(removed, client)
		if client.InstallIntent != domain.InstallIntentAutomatic {
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
		delete(installation.Clients, clientKey)
	}
	active := 0
	for _, binding := range installation.Clients {
		if binding.Materialization != domain.MaterializationAbsent {
			active++
		}
	}
	if purge && active != 0 {
		return state, nil, nil, fmt.Errorf("--purge-data requires ownership-preflight and removal of every active target")
	}
	if active == 0 {
		for key := range installation.Clients {
			delete(installation.Clients, key)
		}
		createdData := []domain.DataReceipt(nil)
		if !purge && len(installation.DataReceipts) == 0 {
			if service.PluginData == nil {
				return state, nil, nil, fmt.Errorf("PLUGIN_DATA manager is required to retain final-binding data")
			}
			for _, client := range removed {
				receipt, created, err := service.PluginData.EnsureData(ctx, installation.InstallationID, client.PhysicalArtifact, client.Scope)
				if err != nil {
					service.cleanupUncommittedPluginData(createdData, transaction.GroupFailureUnchanged)
					return state, nil, nil, fmt.Errorf("establish retained PLUGIN_DATA ownership: %w", err)
				}
				if created {
					createdData = append(createdData, receipt)
				}
				if installation.DataReceipts == nil {
					installation.DataReceipts = map[string]domain.DataReceipt{}
				}
				installation.DataReceipts[receipt.DataReceiptID] = receipt
			}
		}
		if purge {
			receipts := make([]domain.DataReceipt, 0, len(installation.DataReceipts))
			for _, receipt := range installation.DataReceipts {
				receipts = append(receipts, receipt)
			}
			sort.Slice(receipts, func(i, j int) bool { return receipts[i].DataReceiptID < receipts[j].DataReceiptID })
			if len(installation.InstallPreferences) > 0 {
				installation.DataRetained = false
				installation.DataReceipts = nil
				state.Installations[installationIndex] = installation
			} else {
				state.Installations = append(state.Installations[:installationIndex], state.Installations[installationIndex+1:]...)
			}
			return state, receipts, createdData, nil
		}
		installation.DataRetained = true
		installation.OperationGroupID = groupID
		installation.UpdatedAt = service.now().Format(time.RFC3339Nano)
		state.Installations[installationIndex] = installation
		return state, nil, createdData, nil
	}
	installation.OperationGroupID = groupID
	installation.UpdatedAt = service.now().Format(time.RFC3339Nano)
	state.Installations[installationIndex] = installation
	return state, nil, nil, nil
}

func (service Service) cleanupUncommittedPluginData(receipts []domain.DataReceipt, phase transaction.GroupFailurePhase) {
	if service.PluginData == nil || (phase != transaction.GroupFailureUnchanged && phase != transaction.GroupFailureRolledBack) {
		return
	}
	for _, receipt := range receipts {
		_ = service.PluginData.PurgeData(context.Background(), receipt)
	}
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
	state, err := service.StateStore.Load()
	if err != nil {
		return err
	}
	index, installation, err := findInstallation(state, selector)
	if err != nil {
		return err
	}
	if !installation.DataRetained || len(installation.Clients) != 0 {
		return fmt.Errorf("installation is not a data_retained record")
	}
	for _, receipt := range installation.DataReceipts {
		if receipt.State != domain.DataReceiptOwned {
			return fmt.Errorf("PLUGIN_DATA receipt %s is not safely owned", receipt.DataReceiptID)
		}
		if err := service.PluginData.ValidateData(ctx, receipt); err != nil {
			return err
		}
	}
	if !confirmed {
		return nil
	}
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
	receipts := make([]domain.DataReceipt, 0, len(installation.DataReceipts))
	for _, receipt := range installation.DataReceipts {
		receipts = append(receipts, receipt)
	}
	sort.Slice(receipts, func(i, j int) bool { return receipts[i].DataReceiptID < receipts[j].DataReceiptID })
	removals := make([]transaction.DirectoryRemoval, 0, len(receipts))
	for receiptIndex, dataReceipt := range receipts {
		receipt := dataReceipt
		removals = append(removals, transaction.DirectoryRemoval{OperationID: fmt.Sprintf("%s-data-%03d", operationID, receiptIndex+1),
			OperationGroupID: operationID, ClientBindingID: receipt.DataReceiptID, Sequence: 1, OwnedBase: filepath.Dir(receipt.Locator),
			ActivePath: receipt.Locator, BeforeDigest: receipt.OwnershipDigest, Standalone: true,
			Verify: func(verifyContext context.Context, _ string) error {
				return service.PluginData.ValidateData(verifyContext, receipt)
			}})
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	_, err = kernel.RemoveDirectoryGroup(ctx, transaction.DirectoryRemovalGroup{OperationGroupID: operationID, Removals: removals, DesiredState: state})
	return err
}

func (service Service) markDeactivated(state domain.StateFileV2, installationIndex int, clientKey string) error {
	installation := state.Installations[installationIndex]
	client := installation.Clients[clientKey]
	client.Activation = domain.ActivationNotRequired
	client.Verification = domain.VerificationPackageValid
	client.UpdatedAt = service.now().Format(time.RFC3339Nano)
	installation.Clients[clientKey] = client
	installation.UpdatedAt = client.UpdatedAt
	state.Installations[installationIndex] = installation
	return service.StateStore.Save(state)
}

func findInstallation(state domain.StateFileV2, selector string) (int, domain.Installation, error) {
	selector = strings.TrimSpace(selector)
	matchIndex := -1
	for index, installation := range state.Installations {
		if installation.InstallationID == selector {
			return index, installation, nil
		}
		if installation.DeclaredName == selector {
			if matchIndex >= 0 {
				return -1, domain.Installation{}, fmt.Errorf("installation name %q is ambiguous; use installation_id", selector)
			}
			matchIndex = index
		}
	}
	if matchIndex < 0 {
		return -1, domain.Installation{}, fmt.Errorf("installation %q was not found", selector)
	}
	return matchIndex, state.Installations[matchIndex], nil
}

func findClientBinding(installation domain.Installation, clientID domain.ClientID, scope domain.InstallScope) (string, domain.ClientBinding, error) {
	var matchKey string
	var match domain.ClientBinding
	for key, client := range installation.Clients {
		if client.ClientID != string(clientID) || client.Scope != string(scope) || client.Materialization == domain.MaterializationAbsent {
			continue
		}
		if matchKey != "" {
			return "", domain.ClientBinding{}, fmt.Errorf("multiple client bindings match %s/%s", clientID, scope)
		}
		matchKey, match = key, client
	}
	if matchKey == "" {
		return "", domain.ClientBinding{}, fmt.Errorf("plugin is not materialized for %s/%s", clientID, scope)
	}
	return matchKey, match, nil
}
