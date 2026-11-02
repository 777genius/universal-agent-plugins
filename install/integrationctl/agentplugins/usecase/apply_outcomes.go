package usecase

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (session *applySession) dryRunPath() (AddResult, error) {
	if err := session.service.observePreparedIdentity(session.ctx, session.input.Client, session.plan, session.managedBinding); err != nil {
		return session.result, err
	}
	if session.replace && !session.isMaterialized {
		return session.result, fmt.Errorf("plugin is not materialized for %s; use add", session.input.Client.ClientID)
	}
	if session.isMaterialized {
		current := session.state.Installations[session.installationIndex].Clients[session.clientBindingID]
		session.result.Activation = lifecycleOutcome(current)
		if err := session.service.verifyManagedTarget(session.ctx, session.input.Client, session.input.Scope, current, "dry-run"); err != nil {
			return session.result, err
		}
		if !session.replace && !packageRevisionMatches(current.PackageRevision, session.input.Envelope) {
			return session.result, fmt.Errorf("plugin is already materialized for %s at a different revision; use update", session.input.Client.ClientID)
		}
		if !session.replace {
			if err := session.service.requireCurrentCommandProjection(session.ctx, session.input, session.plan, session.state.Installations[session.installationIndex], current); err != nil {
				return session.result, err
			}
		}
		session.result.NoChange = !session.replace && !session.registrationMigration && lifecycleConverged(current)
	}
	return session.result, nil
}

func (session *applySession) noChangeOrResume() (bool, AddResult, error) {
	if session.managedBinding == nil {
		if err := session.service.observeNativeIdentity(session.ctx, session.input.Client, session.plan, nil); err != nil {
			return true, session.result, err
		}
	}
	if session.isMaterialized && !session.replace && !session.registrationMigration {
		current := session.state.Installations[session.installationIndex].Clients[session.clientBindingID]
		done, result, err := session.finishExistingLifecycle(current, "no-change check")
		return done, result, err
	}
	if session.replace && !session.isMaterialized {
		return true, session.result, fmt.Errorf("plugin is not materialized for %s; use add", session.input.Client.ClientID)
	}
	if session.replace {
		previousClient := session.state.Installations[session.installationIndex].Clients[session.clientBindingID]
		session.result.Activation = lifecycleOutcome(previousClient)
		if err := session.service.verifyManagedTarget(session.ctx, session.input.Client, session.input.Scope, previousClient, "update"); err != nil {
			return true, session.result, err
		}
		if err := session.service.observeNativeIdentity(session.ctx, session.input.Client, session.plan, session.managedBinding); err != nil {
			return true, session.result, err
		}
		if !session.registrationMigration && !session.plan.RequiresNativeProjectionRefresh() && !requiresComponentRemoval(session.plan) && packageRevisionMatches(previousClient.PackageRevision, session.input.Envelope) && previousClient.PackageRevision.ResolvedRevision == session.input.Envelope.Source.ResolvedRevision {
			return session.finishExistingLifecycle(previousClient, "")
		}
	}
	return false, AddResult{}, nil
}

func (session *applySession) finishExistingLifecycle(current domain.ClientBinding, verifyLabel string) (bool, AddResult, error) {
	session.result.Activation = lifecycleOutcome(current)
	if verifyLabel == "no-change check" && !packageRevisionMatches(current.PackageRevision, session.input.Envelope) {
		return true, session.result, fmt.Errorf("plugin is already materialized for %s at a different revision; use update", session.input.Client.ClientID)
	}
	if session.input.Confirmed {
		installation := session.state.Installations[session.installationIndex]
		if err := session.service.prepareExistingRuntime(session.ctx, session.input.Envelope, session.plan, installation, current); err != nil {
			return true, session.result, fmt.Errorf("prepare installed MCP runtime: %w", err)
		}
	}
	if !lifecycleConverged(current) {
		result, err := session.service.resume(session.ctx, session.input, session.result, session.installationID, session.clientBindingID, current)
		return true, result, err
	}
	if verifyLabel != "" {
		if err := session.service.verifyManagedTarget(session.ctx, session.input.Client, session.input.Scope, current, verifyLabel); err != nil {
			return true, session.result, err
		}
	}
	if nativeLifecycleClient(session.input.Client.ClientID, session.plan.SelectedDelivery, session.input.Scope) {
		_, complete, _, err := session.service.activeNativeDelivery(session.ctx, session.input, session.plan, session.state.Installations[session.installationIndex], current)
		if err != nil {
			return true, session.result, err
		}
		if !complete || current.NativeActivationAttempt != "" || current.PendingNativeIntent != nil {
			result, err := session.service.resume(session.ctx, session.input, session.result, session.installationID, session.clientBindingID, current)
			return true, result, err
		}
	}
	return session.persistReadOnlyObservation(current)
}

func (session *applySession) persistReadOnlyObservation(current domain.ClientBinding) (bool, AddResult, error) {
	if !session.result.Plan.SelectedDelivery.IsZero() {
		plan, err := session.service.readOnlyObservationPlan(current, session.result.Plan)
		if err != nil {
			return true, session.result, err
		}
		session.result.Plan = plan
	}
	verified, verifyErr := session.service.verifyClientReadOnly(session.ctx, session.input, session.result, current)
	if verifyErr != nil {
		if retainPreparedProjectVerificationFailure(&session.result, verified, verifyErr) {
			return true, session.result, verifyErr
		}
		if verified.Activation != "" && !session.input.DryRun && (session.input.Confirmed || session.input.PersistAuthoritativeObservations && verified.AuthoritativeObservation) {
			session.result.Activation = verified
			changed, updateErr := session.service.persistAuthoritativeObservation(session.ctx, session.input, session.installationID, session.clientBindingID, current, verified)
			session.result.Mutated = changed
			if updateErr != nil {
				return true, session.result, fmt.Errorf("client verification failed: %w; persist negative verification evidence: %w", verifyErr, updateErr)
			}
		}
		return true, session.result, verifyErr
	}
	if verified.Activation != "" && !sameLifecycleOutcome(verified, lifecycleOutcome(current)) {
		result, err := session.service.persistObservedLifecycle(session.input, session.result, session.installationID, session.clientBindingID, verified)
		return true, result, err
	}
	session.result.NoChange = true
	return true, session.result, nil
}
