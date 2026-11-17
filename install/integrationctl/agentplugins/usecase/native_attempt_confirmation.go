package usecase

import (
	"context"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

// InterruptedAttemptInput selects the one binding whose native attempt a
// killed agentplugins process left open without a persisted intent.
type InterruptedAttemptInput struct {
	InstallationID       string
	Client               domain.DetectedClient
	Scope                domain.InstallScope
	BackendExecutable    string
	EnvironmentAuthority *domain.EnvironmentAuthority
	DryRun               bool
	Confirmed            bool
}

// InterruptedAttemptResult names the attempt and whether its confirmation was
// recorded; a dry run records nothing.
type InterruptedAttemptResult struct {
	AttemptID string
	Recorded  bool
}

// ConfirmsInterruptedAttempt reports whether the client can confirm such an
// attempt in scope.
func (service Service) ConfirmsInterruptedAttempt(id domain.ClientID, scope domain.InstallScope) bool {
	confirmer, ok := service.Activator.(ports.InterruptedAttemptConfirmer)
	return ok && confirmer.ConfirmsInterruptedAttempt(id, scope)
}

// ConfirmInterruptedNativeAttempt settles a native attempt that a killed
// agentplugins process left open. Under the mutation lock it verifies the
// binding and its installed package, lets the client adapter prove that no
// client command of that run is alive and read the client state once, and
// records only a certain effect: the attempt is closed, the confirmed
// ownership kept and the binding marked failed, so the ordinary repair or
// removal decides the rest. A dry run performs the same checks and writes
// nothing.
func (service Service) ConfirmInterruptedNativeAttempt(ctx context.Context, input InterruptedAttemptInput) (InterruptedAttemptResult, error) {
	confirmer, ok := service.Activator.(ports.InterruptedAttemptConfirmer)
	if !ok || !confirmer.ConfirmsInterruptedAttempt(input.Client.ClientID, input.Scope) {
		return InterruptedAttemptResult{}, fmt.Errorf("%s cannot confirm an interrupted client change in %s scope", input.Client.ClientID, input.Scope)
	}
	if !input.DryRun && !input.Confirmed {
		return InterruptedAttemptResult{}, fmt.Errorf("confirming an interrupted client change requires explicit confirmation")
	}
	release, err := service.beginEnvironmentMutation(ctx, input.DryRun, input.Confirmed, input.EnvironmentAuthority)
	if err != nil {
		return InterruptedAttemptResult{}, err
	}
	if release != nil {
		defer func() { _ = release() }()
	}
	attempt, err := service.loadInterruptedAttempt(ctx, input)
	result := InterruptedAttemptResult{AttemptID: attempt.binding.NativeActivationAttempt}
	if err != nil {
		return result, err
	}
	outcome, err := confirmer.ConfirmInterruptedAttempt(ctx, attempt.request)
	if err != nil {
		return result, err
	}
	if outcome.NativeEffect != domain.NativeEffectCommitted && outcome.NativeEffect != domain.NativeEffectUnchanged {
		return result, fmt.Errorf("the client effect of native attempt %s is not certain", result.AttemptID)
	}
	if input.DryRun {
		return result, nil
	}
	// Only the attempt and the confirmed ownership change; activation is
	// decided by the next repair or removal.
	outcome.Authentication, outcome.Policy, outcome.LocalEntryObservation = attempt.binding.Authentication, attempt.binding.Policy, nil
	current, err := service.activationBinding(attempt.installationID, attempt.bindingID)
	if err != nil {
		return result, err
	}
	if !reflect.DeepEqual(current, attempt.binding) {
		return result, fmt.Errorf("the binding changed while native attempt %s was confirmed; retry", result.AttemptID)
	}
	objects := append([]domain.NativeObjectOwnership(nil), outcome.NativeObjects...)
	if _, err := service.updateLifecycleAndNativeObjectsWithAttempt(attempt.installationID, attempt.bindingID, outcome, &objects, true, true, nil); err != nil {
		return result, err
	}
	result.Recorded = true
	return result, nil
}

// interruptedAttempt is the selected binding and the exact installed delivery,
// with its recorded ownership, that the client adapter confirms.
type interruptedAttempt struct {
	installationID, bindingID string
	binding                   domain.ClientBinding
	request                   domain.ActivationRequest
}

// loadInterruptedAttempt selects the binding, requires a journal-free attempt
// and an intact installed package, and describes that exact delivery.
func (service Service) loadInterruptedAttempt(ctx context.Context, input InterruptedAttemptInput) (interruptedAttempt, error) {
	state, err := service.StateStore.Load()
	if err != nil {
		return interruptedAttempt{}, err
	}
	_, installation, err := findInstallation(state, input.InstallationID)
	if err != nil {
		return interruptedAttempt{}, err
	}
	key, binding, err := findClientBinding(installation, input.Client.ClientID, input.Scope)
	attempt := interruptedAttempt{installationID: installation.InstallationID, bindingID: key, binding: binding}
	if err != nil {
		return attempt, err
	}
	if binding.NativeActivationAttempt == "" || binding.PendingNativeIntent != nil {
		return attempt, fmt.Errorf("the target has no interrupted client change to confirm")
	}
	if err := transaction.AuthorizeEnvironmentMutation(state, input.EnvironmentAuthority, "repair", installation.InstallationID, key, installation.Source.SourceBindingID, false); err != nil {
		return attempt, err
	}
	unmarked := binding
	unmarked.NativeActivationAttempt = ""
	if err := service.validateNativeBinding(ctx, unmarked, input.Client); err != nil {
		return attempt, err
	}
	if err := service.verifyManagedTarget(ctx, input.Client, input.Scope, binding, "confirmation of the interrupted client change"); err != nil {
		return attempt, err
	}
	target, err := service.Targets.ResolveTarget(ctx, input.Client, input.Scope, binding.PhysicalArtifact)
	if err != nil {
		return attempt, err
	}
	plan := domain.DeliveryPlan{ClientID: input.Client.ClientID, Scope: input.Scope, DeclaredName: installation.DeclaredName, PhysicalArtifactID: binding.PhysicalArtifact,
		TargetContext: target.TargetContext, NativeRegistryRoot: target.NativeRegistryRoot, TargetAnchor: target.TargetAnchor, TargetRoot: target.TargetRoot, ActivePath: target.ActivePath}
	attempt.request = domain.ActivationRequest{
		Client: input.Client, Plan: plan, DeclaredName: installation.DeclaredName, BackendExecutable: input.BackendExecutable,
		Delivery:              domain.StagedDelivery{ClientID: input.Client.ClientID, OwnedBase: target.TargetRoot, ActivePath: target.ActivePath, ArtifactDigest: managedDigest(binding)},
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...), Replacing: true, VerifyOnly: true,
	}
	return attempt, nil
}
