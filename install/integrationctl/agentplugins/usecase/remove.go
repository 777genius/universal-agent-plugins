package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	if service.StateStore == nil || service.Paths == nil || service.Targets == nil || service.Stager == nil || service.Activator == nil {
		return RemoveResult{}, fmt.Errorf("agentplugins service dependencies are incomplete")
	}
	release, err := service.beginMutation(ctx, input.DryRun, input.Confirmed)
	if err != nil {
		return RemoveResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	session := &removeSession{service: service, ctx: ctx, input: input}
	if err := session.loadRemoveTarget(); err != nil {
		return session.result, err
	}
	if err := session.deactivateRemoveTarget(); err != nil {
		return session.result, err
	}
	if input.DryRun || !session.result.Deactivation.ArtifactRemovalAllowed {
		return session.result, nil
	}
	if !input.Confirmed {
		session.result.RequiresConfirmation = true
		return session.result, nil
	}
	return session.commitRemove()
}

func (service Service) cleanupUncommittedPluginData(receipts []domain.DataReceipt, phase transaction.GroupFailurePhase) {
	if service.PluginData == nil || (phase != transaction.GroupFailureUnchanged && phase != transaction.GroupFailureRolledBack) {
		return
	}
	for _, receipt := range receipts {
		_ = service.PluginData.PurgeData(context.Background(), receipt)
	}
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
