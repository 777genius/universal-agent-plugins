package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

type LegacyRemoveInput struct {
	Selector    string
	DryRun      bool
	Confirmed   bool
	OperationID string
}

type LegacyRemoveResult struct {
	InstallationID       string                  `json:"installation_id"`
	Plugin               string                  `json:"plugin"`
	Targets              []string                `json:"targets"`
	LegacyPlan           ports.LegacyRemovalPlan `json:"legacy_plan"`
	RequiresConfirmation bool                    `json:"requires_confirmation"`
	Reconciled           bool                    `json:"reconciled,omitempty"`
	Mutated              bool                    `json:"mutated"`
}

func (service Service) RemoveLegacy(ctx context.Context, input LegacyRemoveInput) (LegacyRemoveResult, error) {
	if service.StateStore == nil || service.Legacy == nil || service.LegacyLock == nil {
		return LegacyRemoveResult{}, fmt.Errorf("legacy lifecycle dependencies are incomplete")
	}
	release, err := service.beginMutation(ctx, input.DryRun, input.Confirmed)
	if err != nil {
		return LegacyRemoveResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	state, installationIndex, installation, err := service.loadLegacyInstallation(input.Selector)
	if err != nil {
		return LegacyRemoveResult{}, err
	}
	result := LegacyRemoveResult{
		InstallationID: installation.InstallationID,
		Plugin:         installation.DeclaredName,
		Targets:        materializedTargets(installation),
	}
	exists, err := service.inspectLegacyRemoval(ctx, installation, &result)
	if err != nil {
		return result, err
	}
	if input.DryRun {
		return result, nil
	}
	if !input.Confirmed {
		result.RequiresConfirmation = true
		return result, nil
	}
	return service.commitLegacyRemoval(ctx, state, installationIndex, installation, input, exists, result)
}

func (service Service) loadLegacyInstallation(selector string) (domain.StateFileV2, int, domain.Installation, error) {
	state, err := service.StateStore.Load()
	if err != nil {
		return domain.StateFileV2{}, -1, domain.Installation{}, err
	}
	installationIndex, installation, err := findInstallation(state, selector)
	if err != nil {
		return domain.StateFileV2{}, -1, domain.Installation{}, err
	}
	if installation.Package.LoaderKind != domain.LoaderKindLegacy {
		return domain.StateFileV2{}, -1, domain.Installation{}, fmt.Errorf("installation %s is not managed by the legacy lifecycle", installation.InstallationID)
	}
	return state, installationIndex, installation, nil
}

func (service Service) inspectLegacyRemoval(ctx context.Context, installation domain.Installation, result *LegacyRemoveResult) (bool, error) {
	exists, err := service.Legacy.Exists(ctx, installation.DeclaredName)
	if err != nil {
		return false, fmt.Errorf("inspect legacy lifecycle state: %w", err)
	}
	if !exists {
		result.LegacyPlan = ports.LegacyRemovalPlan{Summary: "legacy lifecycle already reports this installation absent"}
		result.Reconciled = true
		return false, nil
	}
	result.LegacyPlan, err = service.Legacy.PlanRemove(ctx, installation.DeclaredName)
	if err != nil {
		return true, fmt.Errorf("plan legacy removal: %w", err)
	}
	return true, nil
}

func (service Service) commitLegacyRemoval(ctx context.Context, state domain.StateFileV2, installationIndex int, installation domain.Installation, input LegacyRemoveInput, exists bool, result LegacyRemoveResult) (LegacyRemoveResult, error) {
	if exists {
		if _, err := service.Legacy.Remove(ctx, installation.DeclaredName); err != nil {
			return result, fmt.Errorf("remove through legacy lifecycle: %w", err)
		}
	}
	if err := service.verifyLegacyAbsent(ctx, installation.DeclaredName); err != nil {
		return result, err
	}
	operationID := strings.TrimSpace(input.OperationID)
	if operationID == "" {
		var err error
		operationID, err = newOperationID()
		if err != nil {
			return result, err
		}
	}
	markLegacyBindingsAbsent(&installation, operationID, service.now().Format(time.RFC3339Nano))
	state.Installations[installationIndex] = installation
	if err := service.StateStore.Save(state); err != nil {
		return result, fmt.Errorf("reconcile Agent Plugins state after legacy removal: %w", err)
	}
	result.Mutated = true
	return result, nil
}

func (service Service) verifyLegacyAbsent(ctx context.Context, declaredName string) error {
	legacyRelease, err := service.LegacyLock.Acquire(ctx, "state")
	if err != nil {
		return fmt.Errorf("acquire legacy state lock for reconciliation: %w", err)
	}
	defer func() { _ = legacyRelease() }()
	stillExists, err := service.Legacy.Exists(ctx, declaredName)
	if err != nil {
		return fmt.Errorf("verify legacy lifecycle removal: %w", err)
	}
	if stillExists {
		return fmt.Errorf("legacy lifecycle changed before reconciliation; retry after reviewing the current legacy state")
	}
	return nil
}

func markLegacyBindingsAbsent(installation *domain.Installation, operationID, timestamp string) {
	for key, client := range installation.Clients {
		if client.Materialization == domain.MaterializationAbsent && len(client.NativeObjects) == 0 {
			continue
		}
		client.Receipts = append(client.Receipts, domain.MutationReceipt{
			OperationID: operationID + "-" + client.ClientBindingID,
			Sequence:    nextSequence(client), MutationType: "legacy_remove_bridge",
			ClientBindingID: client.ClientBindingID, BeforeDigest: managedDigest(client),
			Phase: transaction.ReceiptPhaseCommitted,
		})
		client.Materialization = domain.MaterializationAbsent
		client.Activation = domain.ActivationNotRequired
		client.Authentication = domain.AuthenticationNotRequired
		client.Policy = domain.PolicyAllowed
		client.Verification = domain.VerificationNotRun
		client.NativeObjects = nil
		client.UpdatedAt = timestamp
		installation.Clients[key] = client
	}
	installation.UpdatedAt = timestamp
}

func materializedTargets(installation domain.Installation) []string {
	values := make([]string, 0, len(installation.Clients))
	for _, client := range installation.Clients {
		if client.Materialization != domain.MaterializationAbsent || len(client.NativeObjects) > 0 {
			values = append(values, client.ClientID+"/"+client.Scope)
		}
	}
	sort.Strings(values)
	return values
}
