package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type RemoveGroupInput struct {
	Selector         string
	Targets          []RemoveInput
	OperationGroupID string
	DryRun           bool
	Confirmed        bool
	PurgeData        bool
}

type RemoveGroupResult struct {
	InstallationID   string                   `json:"installation_id"`
	OperationGroupID string                   `json:"operation_group_id,omitempty"`
	Targets          []RemoveResult           `json:"targets"`
	Receipts         []domain.MutationReceipt `json:"-"`
	Mutated          bool                     `json:"mutated"`
	Phase            GroupPhase               `json:"phase"`
	ManagedPhase     GroupPhase               `json:"managed_phase,omitempty"`
}

type plannedRemoval struct {
	input         RemoveInput
	clientKey     string
	client        domain.ClientBinding
	targetRoot    string
	resultIndexes []int
}

func (service Service) RemoveGroup(ctx context.Context, input RemoveGroupInput) (RemoveGroupResult, error) {
	session := &removeGroupSession{service: service, ctx: ctx, input: input}
	if err := session.validateRemoveGroupInput(); err != nil {
		return RemoveGroupResult{}, err
	}
	release, err := service.beginMutation(ctx, input.DryRun, input.Confirmed)
	if err != nil {
		return RemoveGroupResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	if err := session.resolveGroupBindings(); err != nil {
		return session.result, err
	}
	if input.DryRun || !input.Confirmed {
		return session.result, nil
	}
	if err := session.removeGroupNative(); err != nil {
		return session.result, err
	}
	return session.persistRemoveGroup()
}
