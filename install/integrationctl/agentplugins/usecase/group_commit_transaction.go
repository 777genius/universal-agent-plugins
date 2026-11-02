package usecase

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func (session *groupSession) applyGroupKernel() error {
	// Keep the prepared host authority pinned through the final kernel boundary.
	for _, target := range session.planned {
		if session.service.ClientPreparation != nil {
			if err := session.service.ClientPreparation.RevalidateClient(session.ctx, target.input.Client, target.plan); err != nil {
				return fmt.Errorf("prepared host changed before group commit: %w", err)
			}
		}
	}
	mutations := session.buildGroupMutations()
	kernel := session.service.Kernel
	kernel.StateStore = session.service.StateStore
	postApplyVerify := session.service.groupRecoveryPostApplyVerify(session.planned)
	if len(mutations) == 0 {
		session.clearCreatedPluginData()
		for _, target := range session.planned {
			if err := session.reportGroupProgress(target, GroupProgressConfigured); err != nil {
				return err
			}
		}
		return nil
	}
	receipts, err := kernel.ApplyDirectoryGroup(session.ctx, transaction.DirectoryGroup{
		OperationGroupID: session.groupID, Mutations: mutations, DesiredState: session.desired, PostApplyVerify: postApplyVerify,
	})
	if err != nil {
		session.result.Receipts = receipts
		assignGroupReceipts(session.result.Targets, session.planned, receipts)
		session.markGroupKernelFailure(err, receipts)
		return err
	}
	session.result.Receipts, session.result.Mutated = receipts, true
	assignGroupReceipts(session.result.Targets, session.planned, receipts)
	session.result.Phase = GroupPhaseManagedCommitted
	for index := range session.result.Targets {
		session.result.Targets[index].GroupPhase = GroupTargetManagedCommitted
	}
	session.clearCreatedPluginData()
	for _, target := range session.planned {
		if err := session.reportGroupProgress(target, GroupProgressConfigured); err != nil {
			return err
		}
	}
	return nil
}

func (session *groupSession) buildGroupMutations() []transaction.DirectoryMutation {
	mutations := make([]transaction.DirectoryMutation, 0, len(session.planned))
	stager := session.service.Stager
	for targetIndex, target := range session.planned {
		if target.noChange {
			continue
		}
		client := session.desired.Installations[session.installationIndex].Clients[target.clientBindingID]
		before := ""
		if target.managed != nil && !target.requireAbsent {
			// A recovering target's active path is positively absent right now; its
			// recorded receipt digest describes what was there before it disappeared,
			// not the current (absent) state this mutation actually observed.
			before = managedDigest(*target.managed)
		}
		operationID := fmt.Sprintf("%s-%03d", session.groupID, targetIndex+1)
		delivery := target.delivery
		authentication := target.plan.Authentication
		if session.input.Repair && target.managed != nil {
			authentication = preservedGroupAuthentication(authentication, target.managed.Authentication)
		}
		activation, verification := initialLifecycle(target.plan)
		mutations = append(mutations, transaction.DirectoryMutation{
			OperationID: operationID, InstallationID: session.installationID, ClientBindingID: target.clientBindingID,
			Sequence: nextSequence(client), OwnedBase: delivery.OwnedBase, ActivePath: delivery.ActivePath, StagingPath: delivery.StagingPath,
			BeforeDigest: before, AfterDigest: delivery.ArtifactDigest, NativeObjects: delivery.NativeObjects, Activation: activation,
			Authentication: authentication, Policy: domain.PolicyAllowed, Verification: verification, RequireAbsent: target.requireAbsent,
			VerifyBefore: func(verifyContext context.Context, path string) error {
				return stager.Verify(verifyContext, path, before)
			},
			Verify: func(verifyContext context.Context, activePath string) error {
				return stager.Verify(verifyContext, activePath, delivery.ArtifactDigest)
			},
			EnvironmentAuthority: target.input.EnvironmentAuthority,
		})
	}
	return mutations
}

func (session *groupSession) markGroupKernelFailure(err error, receipts []domain.MutationReceipt) {
	failure := transaction.FailurePhase(err)
	switch failure {
	case transaction.GroupFailureRolledBack:
		session.result.Phase = GroupPhaseManagedRolledBack
		for index := range session.result.Targets {
			session.result.Targets[index].GroupPhase = GroupTargetManagedRolledBack
		}
	case transaction.GroupFailureCommitted:
		session.result.Phase = GroupPhaseManagedCommitted
	case transaction.GroupFailureUnknown:
		session.result.Phase = GroupPhaseManagedCommitUnknown
		for index := range session.result.Targets {
			session.result.Targets[index].GroupPhase = GroupTargetManagedUnknown
		}
	default:
		session.result.Phase = GroupPhaseManagedUnchanged
	}
	if failure == transaction.GroupFailureCommitted {
		receipted := make(map[string]bool, len(receipts))
		for _, receipt := range receipts {
			receipted[receipt.ClientBindingID] = true
		}
		for _, target := range session.planned {
			if !receipted[target.clientBindingID] {
				continue
			}
			for _, resultIndex := range target.resultIndexes {
				session.result.Targets[resultIndex].GroupPhase = GroupTargetManagedCommitted
			}
		}
	}
	if failure == transaction.GroupFailureCommitted || failure == transaction.GroupFailureUnknown {
		session.clearCreatedPluginData()
	}
}

func (session *groupSession) clearCreatedPluginData() {
	for index := range session.planned {
		session.planned[index].dataCreated = false
	}
}
