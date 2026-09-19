package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func (session *removeGroupSession) persistRemoveGroup() (RemoveGroupResult, error) {
	removals, err := session.buildDirectoryRemovals()
	if err != nil {
		return session.result, err
	}
	kernel := session.service.Kernel
	kernel.StateStore = session.service.StateStore
	receipts, err := kernel.RemoveDirectoryGroup(session.ctx, transaction.DirectoryRemovalGroup{
		OperationGroupID: session.groupID, Removals: removals, DesiredState: session.desired,
	})
	session.result.Receipts = receipts
	session.assignRemovalReceipts(receipts)
	if err != nil {
		session.service.cleanupUncommittedPluginData(session.createdData, transaction.FailurePhase(err))
		session.markPersistFailure(err)
		return session.result, err
	}
	session.result.Phase = GroupPhaseCompleted
	session.result.Mutated = true
	for index := range session.result.Targets {
		session.result.Targets[index].Mutated = true
		session.result.Targets[index].GroupPhase = GroupTargetExternalCompleted
	}
	return session.result, nil
}

func (session *removeGroupSession) buildDirectoryRemovals() ([]transaction.DirectoryRemoval, error) {
	removals := make([]transaction.DirectoryRemoval, 0, len(session.planned))
	for index, item := range session.planned {
		expected := managedDigest(item.client)
		activePath := item.client.TargetLocator
		operationID := fmt.Sprintf("%s-%03d", session.groupID, index+1)
		stager := session.service.Stager
		removals = append(removals, transaction.DirectoryRemoval{
			OperationID: operationID, OperationGroupID: session.groupID,
			InstallationID: session.installation.InstallationID, ClientBindingID: item.clientKey, Sequence: nextSequence(item.client),
			OwnedBase: item.targetRoot, ActivePath: activePath, BeforeDigest: expected,
			Verify: func(verifyContext context.Context, path string) error {
				return stager.Verify(verifyContext, path, expected)
			},
		})
	}
	keys := make([]string, 0, len(session.planned))
	for _, item := range session.planned {
		keys = append(keys, item.clientKey)
	}
	sort.Strings(keys)
	desired, purgeReceipts, createdData, err := session.service.prepareRemovedBindingsState(session.ctx, session.state, session.index, keys, session.input.PurgeData, session.groupID)
	if err != nil {
		session.service.cleanupUncommittedPluginData(createdData, transaction.FailurePhase(err))
		session.result.Phase = GroupPhaseExternalPartialFailure
		for index := range session.result.Targets {
			session.result.Targets[index].GroupPhase = GroupTargetExternalPartial
		}
		return nil, err
	}
	session.desired = desired
	session.createdData = createdData
	pluginData := session.service.PluginData
	for index, dataReceipt := range purgeReceipts {
		receipt := dataReceipt
		operationID := fmt.Sprintf("%s-data-%03d", session.groupID, index+1)
		removals = append(removals, transaction.DirectoryRemoval{
			OperationID: operationID, OperationGroupID: session.groupID,
			ClientBindingID: receipt.DataReceiptID, Sequence: 1, OwnedBase: filepath.Dir(receipt.Locator),
			ActivePath: receipt.Locator, BeforeDigest: receipt.OwnershipDigest, Standalone: true,
			Verify: func(verifyContext context.Context, _ string) error {
				return pluginData.ValidateData(verifyContext, receipt)
			},
		})
	}
	return removals, nil
}

func (session *removeGroupSession) assignRemovalReceipts(receipts []domain.MutationReceipt) {
	byReceiptBinding := make(map[string]domain.MutationReceipt, len(receipts))
	for _, receipt := range receipts {
		byReceiptBinding[receipt.ClientBindingID] = receipt
	}
	session.byReceiptBinding = byReceiptBinding
	for _, item := range session.planned {
		receipt, ok := byReceiptBinding[item.clientKey]
		if !ok {
			continue
		}
		for _, resultIndex := range item.resultIndexes {
			session.result.Targets[resultIndex].Receipt = receipt
		}
	}
}

func (session *removeGroupSession) markPersistFailure(err error) {
	switch transaction.FailurePhase(err) {
	case transaction.GroupFailureRolledBack:
		session.result.ManagedPhase = GroupPhaseManagedRolledBack
		for index := range session.result.Targets {
			session.result.Targets[index].GroupPhase = GroupTargetExternalPartial
		}
	case transaction.GroupFailureCommitted:
		session.result.ManagedPhase = GroupPhaseManagedCommitted
		for _, item := range session.planned {
			if _, ok := session.byReceiptBinding[item.clientKey]; !ok {
				continue
			}
			for _, resultIndex := range item.resultIndexes {
				session.result.Targets[resultIndex].GroupPhase = GroupTargetManagedCommitted
			}
		}
	case transaction.GroupFailureUnknown:
		session.result.ManagedPhase = GroupPhaseManagedCommitUnknown
		for index := range session.result.Targets {
			session.result.Targets[index].GroupPhase = GroupTargetExternalPartial
		}
	default:
		session.result.ManagedPhase = GroupPhaseManagedUnchanged
		for index := range session.result.Targets {
			session.result.Targets[index].GroupPhase = GroupTargetExternalPartial
		}
	}
	if session.result.ManagedPhase == GroupPhaseManagedCommitted {
		session.result.Phase = GroupPhaseManagedCommitted
	} else if session.externalCompleted > 0 {
		session.result.Phase = GroupPhaseExternalPartialFailure
	} else {
		session.result.Phase = session.result.ManagedPhase
	}
}
