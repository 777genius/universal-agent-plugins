package installer

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (e *Engine) prepareGroup(ctx context.Context, req Request) (*PreparedOperation, error) {
	if err := validateGroupTargets(req.Targets); err != nil {
		return nil, err
	}
	switch req.Operation {
	case OpInstall, OpUpdate, OpRepair:
		return e.prepareMutatingGroup(ctx, req)
	case OpRemove:
		return e.prepareRemoveGroup(ctx, req)
	default:
		return nil, fmt.Errorf("%w: unknown operation", ErrInvalidRequest)
	}
}

func validateGroupTargets(targets []ClientTarget) error {
	if len(targets) != 2 {
		return fmt.Errorf("%w: group operations accept exactly two Claude/Codex targets", ErrInvalidRequest)
	}
	seen := map[string]struct{}{}
	for _, target := range targets {
		if target.ClientID != "claude" && target.ClientID != "codex" {
			return fmt.Errorf("%w: client %q is not in this beta", ErrUnsupported, target.ClientID)
		}
		if _, ok := seen[target.ClientID]; ok {
			return fmt.Errorf("%w: duplicate client %s", ErrInvalidRequest, target.ClientID)
		}
		seen[target.ClientID] = struct{}{}
	}
	return nil
}

func (e *Engine) previewGroup(ctx context.Context, svc usecase.Service, req Request, inputs []usecase.AddInput) (usecase.GroupResult, error) {
	group := usecase.GroupInput{
		Targets: inputs, CompatibilityChecks: inputs, OperationGroupID: firstNonEmpty(req.OperationID, "group"),
		DryRun: true, Confirmed: false, Repair: req.Operation == OpRepair,
	}
	switch req.Operation {
	case OpUpdate:
		return svc.UpdateGroup(ctx, group)
	case OpRepair:
		return svc.RepairGroup(ctx, group)
	default:
		return svc.AddGroup(ctx, group)
	}
}

func groupPreviewUnchanged(preview usecase.GroupResult) bool {
	if len(preview.Targets) == 0 {
		return false
	}
	for _, target := range preview.Targets {
		if !target.NoChange {
			return false
		}
	}
	return true
}

func (e *Engine) groupAddInputs(req Request, envelopes []domain.PackageEnvelope, dry, confirmed bool) ([]usecase.AddInput, []domain.DetectedClient, error) {
	if len(envelopes) != len(req.Targets) {
		return nil, nil, fmt.Errorf("%w: group envelope count", ErrInvalidRequest)
	}
	var inputs []usecase.AddInput
	var clients []domain.DetectedClient
	for i, target := range req.Targets {
		client, err := e.detectedClient(Request{
			Operation: req.Operation, ClientID: target.ClientID,
			ClientConfigRoot: target.ClientConfigRoot, ClientExecutable: firstNonEmpty(target.ClientExecutable, req.ClientExecutable),
		})
		if err != nil {
			return nil, nil, err
		}
		inputs = append(inputs, usecase.AddInput{
			Envelope: envelopes[i], Client: client, Scope: domain.ScopeUser, DryRun: dry, Confirmed: confirmed,
			PersistAuthoritativeObservations: e.persistObservations,
			InstallationID:                   req.InstallationID, OperationID: req.OperationID,
			BackendExecutable: client.ExecutablePath,
		})
		clients = append(clients, client)
	}
	return inputs, clients, nil
}
