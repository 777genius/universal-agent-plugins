package usecase

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Package reuse and native setup are separate. An online review can describe
// intact-package repair without staging or changing the saved binding. Apply
// repeats that observation under its existing lock before any group effects.
func (session *groupSession) observeGroupPreparedSetups() error {
	if !session.existing {
		return nil
	}
	for index := range session.planned {
		target := &session.planned[index]
		if !target.noChange || target.managed == nil || session.input.DryRun && !target.input.OnlinePreview {
			continue
		}
		probe := session.groupPreparedSetupProbe(*target)
		if !probe.freshExistingVerification() {
			continue
		}
		_, complete, _, err := session.service.activeNativeDelivery(session.ctx, target.input, target.plan, session.state.Installations[session.installationIndex], *target.managed)
		if err != nil {
			return err
		}
		if !complete {
			continue // The existing group registration path already owns this retry.
		}
		observed, err := session.service.verifyClientReadOnly(session.ctx, probe.input, probe.result, *target.managed)
		if err != nil {
			return fmt.Errorf("observe native setup for %s: %w", target.input.Client.ClientID, err)
		}
		if observed.Activation == "" {
			continue
		}
		needed, _, err := probe.completeExistingPreparedSetup(*target.managed, observed)
		if err != nil {
			return fmt.Errorf("review native setup for %s: %w", target.input.Client.ClientID, err)
		}
		target.completePreparedSetup = needed
		session.setGroupPreparedSetupResults(*target, observed)
	}
	return nil
}

func (session *groupSession) setGroupPreparedSetupResults(target plannedGroupTarget, observed domain.ActivationOutcome) {
	for _, resultIndex := range target.resultIndexes {
		result := &session.result.Targets[resultIndex]
		result.Activation = observed
		result.NoChange = !target.completePreparedSetup && sameLifecycleOutcome(observed, lifecycleOutcome(*target.managed))
		result.RequiresConfirmation = !result.NoChange && (session.input.DryRun || !session.input.Confirmed)
		if target.completePreparedSetup && result.RequiresConfirmation {
			result.Plan.UserActions = append(append([]string(nil), result.Plan.UserActions...), "Repair native setup for the selected project using the existing owned package.")
		}
	}
}

func (session *groupSession) groupPreparedSetupProbe(target plannedGroupTarget) applySession {
	input := target.input
	input.DryRun, input.Confirmed = true, false
	return applySession{
		service: session.service, ctx: session.ctx, input: input, state: session.state,
		installationIndex: session.installationIndex, installationID: session.installationID,
		clientBindingID: target.clientBindingID, plan: target.plan,
		result: AddResult{InstallationID: session.installationID, Plan: target.plan, Activation: lifecycleOutcome(*target.managed)},
	}
}

// Progress and host callbacks run after preflight. Reuse the single-target
// repair checks immediately before native activation so neither a changed
// receipt nor replaced package can inherit the earlier setup classification.
func (session *groupSession) recheckGroupPreparedSetups() error {
	for _, target := range session.planned {
		if target.completePreparedSetup {
			if err := session.recheckGroupPreparedSetup(target); err != nil {
				return err
			}
		}
	}
	return nil
}

func (session *groupSession) recheckGroupPreparedSetup(target plannedGroupTarget) error {
	if err := session.service.revalidateReadOnlyBinding(*target.managed); err != nil {
		return err
	}
	probe := session.groupPreparedSetupProbe(target)
	observed := session.result.Targets[target.resultIndexes[0]].Activation
	needed, _, err := probe.completeExistingPreparedSetup(*target.managed, observed)
	if err != nil {
		return err
	}
	if !needed {
		return fmt.Errorf("native setup no longer matches the reviewed owned package")
	}
	return nil
}
