package usecase

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

func (session *groupSession) buildDesiredGroupState() {
	session.desired = session.state
	for _, target := range session.planned {
		if target.noChange {
			continue
		}
		session.upsertGroupTargetState(target)
	}
	session.restoreGroupOrigin()
}

func (session *groupSession) upsertGroupTargetState(target plannedGroupTarget) {
	session.desired, session.installationIndex = upsertPreparedInstallation(session.desired, session.installationIndex, session.existing, target.input, target.plan, session.installationID, target.clientBindingID, session.sourceID, session.service.now())
	session.existing = true
	installation := session.desired.Installations[session.installationIndex]
	client := installation.Clients[target.clientBindingID]
	if target.managed != nil {
		// Addressing a shared backend through another logical surface must not
		// rewrite the persisted physical binding identity (and thereby make its
		// deterministic binding ID inconsistent).
		client.ClientID = target.managed.ClientID
		client.AffectedSurfaces = append(client.AffectedSurfaces, target.managed.AffectedSurfaces...)
		client.AffectedSurfaces = append(client.AffectedSurfaces, target.managed.ClientID)
	}
	if sharesPhysicalBackend(target.input.Client.ClientID) {
		client.AffectedSurfaces = append(client.AffectedSurfaces, string(target.input.Client.ClientID))
		for _, sibling := range domain.BackendSiblings(target.input.Client.ClientID) {
			client.AffectedSurfaces = append(client.AffectedSurfaces, string(sibling))
		}
	}
	for _, resultIndex := range target.resultIndexes {
		client.AffectedSurfaces = append(client.AffectedSurfaces, string(session.input.Targets[resultIndex].Client.ClientID))
	}
	client.AffectedSurfaces = uniqueSortedSurfaces(client.AffectedSurfaces)
	if session.input.Repair && target.managed != nil {
		// A repair commit becomes authoritative before directory/receipt
		// finalization. Preserve prior authentication evidence in that commit
		// decision so a late failure cannot reset completed authentication.
		client.Authentication = preservedGroupAuthentication(target.plan.Authentication, target.managed.Authentication)
	}
	if target.dataReceipt.DataReceiptID != "" {
		if installation.DataReceipts == nil {
			installation.DataReceipts = map[string]domain.DataReceipt{}
		}
		installation.DataReceipts[target.dataReceipt.DataReceiptID] = target.dataReceipt
		client.DataReceiptID = target.dataReceipt.DataReceiptID
	} else if session.input.Switch && target.managed != nil {
		// A destination that does not consume PLUGIN_DATA must not sever the
		// retained receipt from the active binding. A later reverse switch
		// can therefore recover the same owned directory.
		client.DataReceiptID = target.managed.DataReceiptID
	}
	installation.DataRetained = false
	installation.OperationGroupID = session.groupID
	installation.Clients[target.clientBindingID] = client
	session.desired.Installations[session.installationIndex] = installation
}

func (session *groupSession) restoreGroupOrigin() {
	if session.input.Repair && session.lifecycleBaseline != nil {
		// Repair restores each binding's own recorded projection. It must not let
		// iteration order move the installation-wide desired release/source back
		// to whichever heterogeneous binding happened to be repaired last.
		installation := session.desired.Installations[session.installationIndex]
		installation.DeclaredName = session.lifecycleBaseline.DeclaredName
		installation.Source = session.lifecycleBaseline.Source
		installation.Package = session.lifecycleBaseline.Package
		installation.OriginMode = session.lifecycleBaseline.OriginMode
		installation.Directory = cloneDirectoryOrigin(session.lifecycleBaseline.Directory)
		session.desired.Installations[session.installationIndex] = installation
	}
	if session.input.Switch {
		installation := session.desired.Installations[session.installationIndex]
		first := session.first
		installation.Source = domain.SourceBinding{
			SourceBindingID: session.sourceID, RequestedSource: first.Envelope.Source.RequestedSource,
			CanonicalSource: first.Envelope.Source.CanonicalSource, Repository: first.Envelope.Source.Repository,
			PackageSubpath: first.Envelope.Source.PackageSubpath, ResolvedRevision: first.Envelope.Source.ResolvedRevision,
			TreeDigest: first.Envelope.TreeDigest,
		}
		installation.OriginMode = normalizedOriginMode(first.OriginMode)
		installation.Directory = cloneDirectoryOrigin(first.DirectoryResolution)
		session.desired.Installations[session.installationIndex] = installation
		return
	}
	if session.input.Repair && session.lifecycleBaseline != nil {
		installation := session.desired.Installations[session.installationIndex]
		installation.Source = session.lifecycleBaseline.Source
		installation.Package = session.lifecycleBaseline.Package
		installation.OriginMode = session.lifecycleBaseline.OriginMode
		installation.Directory = cloneDirectoryOrigin(session.lifecycleBaseline.Directory)
		session.desired.Installations[session.installationIndex] = installation
	}
}

func (session *groupSession) applyGroupKernel() error {
	mutations := session.buildGroupMutations()
	kernel := session.service.Kernel
	kernel.StateStore = session.service.StateStore
	postApplyVerify := session.service.groupRecoveryPostApplyVerify(session.planned)
	if len(mutations) == 0 {
		session.clearCreatedPluginData()
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
		if target.managed != nil && !target.recovering {
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
		mutations = append(mutations, transaction.DirectoryMutation{
			OperationID: operationID, InstallationID: session.installationID, ClientBindingID: target.clientBindingID,
			Sequence: nextSequence(client), OwnedBase: delivery.OwnedBase, ActivePath: delivery.ActivePath, StagingPath: delivery.StagingPath,
			BeforeDigest: before, AfterDigest: delivery.ArtifactDigest, NativeObjects: delivery.NativeObjects, Activation: target.plan.Activation,
			Authentication: authentication, Policy: domain.PolicyAllowed, Verification: target.plan.Verification, RequireAbsent: target.recovering,
			Verify: func(verifyContext context.Context, activePath string) error {
				return stager.Verify(verifyContext, activePath, delivery.ArtifactDigest)
			},
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
