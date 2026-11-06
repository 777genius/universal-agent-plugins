package installer

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (e *Engine) applyMutatingPackage(ctx context.Context, prepared *PreparedOperation, call func(usecase.Service, usecase.AddInput) (usecase.AddResult, error)) (Result, error) {
	// A projection refresh invokes the host handoff from its dedicated
	// post-commit activation path, including an identical-output retry.
	if committed, binding, ok := e.liveBinding(prepared); ok && prepared.req.Operation != OpRefreshProjection && hostHandoffPending(binding) && e.cfg.OnCommittedBinding != nil {
		if err := e.checkPreparedProfiles(ctx, prepared); err != nil {
			return committed, err
		}
		committed.Binding.ProfileAuthority = domain.CloneProfileAuthority(committed.Binding.ProfileAuthority)
		if err := e.cfg.OnCommittedBinding(ctx, committed.Binding); err != nil {
			committed.Outcome = OutcomeIncomplete
			committed.Reason = err.Error()
			e.report(ProgressCommit)
			return committed, err
		}
	}
	if err := e.checkPreparedProfiles(ctx, prepared); err != nil {
		return Result{Operation: prepared.req.Operation, Outcome: OutcomeIncomplete}, err
	}
	helper, err := e.preparedHelper(prepared)
	if err != nil {
		return Result{Operation: prepared.req.Operation, Outcome: OutcomeIncomplete, Reason: err.Error()}, err
	}
	svc := confirmationLifecycle(prepared, e.lifecycle(helper, prepared.facts, prepared.detected), false)
	e.report(ProgressStage)
	if err := e.confirmDeliveries(ctx, prepared); err != nil {
		return Result{Operation: prepared.req.Operation, Outcome: OutcomeConflict, Reason: "plan_changed"}, err
	}
	added, err := call(svc, usecase.AddInput{
		Envelope: prepared.envelope, Client: prepared.client, Scope: e.scope(), Confirmed: true,
		InstallationID: firstNonEmpty(prepared.req.InstallationID, prepared.plan.InstallationID), OperationID: prepared.req.OperationID,
		BackendExecutable: prepared.req.ClientExecutable,
		OriginMode:        prepared.snapshot.OriginMode, DirectoryResolution: cloneAcquiredDirectory(prepared.snapshot.Directory),
		Hints:                prepared.req.acquiredCompatibility(),
		EnvironmentAuthority: prepared.req.EnvironmentAuthority,
		ActivationComplete:   prepared.req.ActivationComplete, AuthComplete: prepared.req.AuthComplete,
	})
	err = wrapLifecycleError(err)
	result := Result{Operation: prepared.req.Operation, InstallationID: added.InstallationID, Binding: prepared.facts,
		Mutated: added.Mutated, RequiresConfirmation: added.RequiresConfirmation}
	if added.Plan.ClientID != "" {
		delivery := deliveryPlan(added.Plan)
		delivery.ProfileAuthority = added.Plan.ProfileAuthority()
		result.Delivery = &delivery
	}
	if added.Activation.UserActions != nil {
		result.ManualActions = append([]string(nil), added.Activation.UserActions...)
	}
	result.LocalActions = append([]string(nil), added.Activation.LocalActions...)
	result, committed, loadErr := e.readCommittedPackage(prepared, added, result, err)
	if added.GroupPhase != "" {
		if result.Client.ClientID == "" {
			result.Client.ClientID = string(added.Plan.ClientID)
		}
		attachClientAttemptResult(&result.Client, added)
	}
	if loadErr != nil {
		return result, loadErr
	}
	return e.finishMutatingPackage(result, added, committed, err)
}

func (e *Engine) readCommittedPackage(prepared *PreparedOperation, added usecase.AddResult, result Result, err error) (Result, bool, error) {
	committed := false
	if state, loadErr := e.store.Load(); loadErr == nil {
		installationID := firstNonEmpty(added.InstallationID, prepared.req.InstallationID)
		if installation, ok := findInstall(state, installationID); ok {
			if binding, receipt, ok := findBinding(installation, prepared.client.ClientID); ok {
				committed = true
				result.InstallationID = firstNonEmpty(installation.InstallationID, installationID)
				result.Binding = BindingFacts{ProfileAuthority: domain.CloneProfileAuthority(binding.ProfileAuthority),
					InstallationID: result.InstallationID, ClientID: string(prepared.client.ClientID),
					SelectedDelivery: binding.SelectedDelivery,
					BindingID:        binding.ClientBindingID, Scope: binding.Scope, TargetPath: binding.TargetLocator,
					DataRoot: receipt.Locator, DataReceiptID: binding.DataReceiptID,
					OperationID: prepared.req.OperationID, TreeDigest: recordedBindingDigest(binding, prepared.plan.TreeDigest),
				}
				result.Client = liveClientResult(binding, prepared.req.RequiredComponents, prepared.plan.TreeDigest)
			}
		}
	} else if err == nil && !added.NoChange {
		result.Outcome = OutcomeIncomplete
		result.Reason = "committed state is unknown"
		result.Recovery.Unknown = []PendingReceipt{{
			OperationID: prepared.req.OperationID, InstallationID: added.InstallationID,
		}}
		return result, false, loadErr
	}
	return result, committed, nil
}

func (e *Engine) finishMutatingPackage(result Result, added usecase.AddResult, committed bool, err error) (Result, error) {
	if reason, conflict := lifecycleConflictReason(err); conflict {
		result.Outcome = OutcomeConflict
		result.Reason = reason
		return result, err
	}
	if added.NoChange && err == nil {
		result.Outcome = OutcomeUnchanged
		result.NoChange = true
	} else if err == nil {
		result.Outcome = OutcomeCompleted
		e.report(ProgressCommit)
		e.report(ProgressActivate)
		e.report(ProgressVerify)
		e.report(ProgressComplete)
	} else {
		result.Outcome = OutcomeIncomplete
		result.Reason = err.Error()
		if committed {
			e.report(ProgressCommit)
		}
	}
	return result, err
}
