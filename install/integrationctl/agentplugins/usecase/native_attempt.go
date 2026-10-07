package usecase

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (service Service) persistLifecycleState(desired domain.StateFileV2) error {
	before, err := service.StateStore.Load()
	if err != nil {
		return err
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	return kernel.PersistStateDecision(before, desired)
}

// beginNativeAttempt is called under the service mutation lock, before an
// adapter can change client configuration. A retained marker means a previous
// effect could not be reconciled and forbids another blind mutation.
func (service Service) beginNativeAttemptWithObservation(installationID, bindingID string, direction domain.NativeIntentDirection, selected domain.SelectedDelivery, previousObservation *domain.LocalEntryObservation) error {
	state, err := service.StateStore.Load()
	if err != nil {
		return err
	}
	for i, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		client, ok := installation.Clients[bindingID]
		if !ok {
			return fmt.Errorf("client binding disappeared before native activation")
		}
		if !client.LocalEntryObservation.Equal(previousObservation) {
			return fmt.Errorf("native attempt observation predecessor changed")
		}
		if client.NativeActivationAttempt != "" {
			return fmt.Errorf("native activation attempt %s is unresolved; inspect owned objects before retry or removal", client.NativeActivationAttempt)
		}
		attemptID, err := newOperationID()
		if err != nil {
			return err
		}
		state.Installations = append([]domain.Installation(nil), state.Installations...)
		installation.Clients = cloneClientBindings(installation.Clients)
		client.NativeActivationAttempt = attemptID
		if err := validateDeliverySelection(client.SelectedDelivery, selected); err != nil {
			return err
		}
		if err := prepareSelectedNativeIntent(&client, attemptID, direction, selected); err != nil {
			return err
		}
		installation.Clients[bindingID] = client
		state.Installations[i] = installation
		return service.persistLifecycleState(state)
	}
	return fmt.Errorf("installation disappeared before native activation")
}

func prepareSelectedNativeIntent(client *domain.ClientBinding, attemptID string, direction domain.NativeIntentDirection, selected domain.SelectedDelivery) error {
	if selected.IsZero() {
		return nil
	}
	client.PendingNativeIntent = &domain.PendingNativeIntent{ProfileAuthority: domain.CloneProfileAuthority(client.ProfileAuthority), ProfileNamespace: client.ProfileNamespace, LocalEntryObservation: client.LocalEntryObservation.Clone(), AttemptID: attemptID, Direction: direction, Delivery: client.SelectedDelivery, RemoveOwnedEntry: direction == domain.NativeIntentRemove && client.SelectedDelivery.OwnsProfileEntry(client.NativeObjects)}
	if _, ok := selected.CursorFacts(); ok {
		// Selection is the attempt packet, not acknowledged ownership. Keep
		// the old whole owned object separately until actual readback succeeds.
		client.SelectedDelivery = selected
		client.PendingNativeIntent.Delivery = selected
		for _, object := range client.NativeObjects {
			if object.Kind == "cursor_user_stop" {
				client.PendingNativeIntent.PreviousCursorObject = object
			}
		}
	}
	return client.PendingNativeIntent.Validate(*client)
}

func cloneClientBindings(source map[string]domain.ClientBinding) map[string]domain.ClientBinding {
	result := make(map[string]domain.ClientBinding, len(source))
	for key, value := range source {
		value.LocalEntryObservation = value.LocalEntryObservation.Clone()
		if value.PendingNativeIntent != nil {
			intent := *value.PendingNativeIntent
			intent.LocalEntryObservation = intent.LocalEntryObservation.Clone()
			value.PendingNativeIntent = &intent
		}
		value.NativeObjects = append([]domain.NativeObjectOwnership(nil), value.NativeObjects...)
		result[key] = value
	}
	return result
}

func (service Service) activateWithNativeAttempt(ctx context.Context, installationID, bindingID string, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := request.Plan.SelectedDelivery.ValidatePlan(request.Plan, selectedCanonicalDigest(request.Plan)); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if service.ClientPreparation != nil && !request.VerifyOnly {
		if err := service.ClientPreparation.RevalidateClient(ctx, request.Client, request.Plan); err != nil {
			return service.refusedPreparedActivation(installationID, bindingID, err)
		}
	}
	if nativeLifecycleClient(request.Client.ClientID, request.Plan.SelectedDelivery) && !request.VerifyOnly {
		if err := service.beginNativeAttemptWithObservation(installationID, bindingID, domain.NativeIntentRegister, request.Plan.SelectedDelivery, request.Plan.LocalEntryObservation.Clone()); err != nil {
			return domain.ActivationOutcome{}, err
		}
		client, err := service.activationBinding(installationID, bindingID)
		if err != nil {
			return domain.ActivationOutcome{}, err
		}
		if client.NativeActivationAttempt == "" {
			return domain.ActivationOutcome{}, fmt.Errorf("native attempt disappeared")
		}
		request.NativeAttempt = domain.NativeAttemptIdentity{OperationID: client.NativeActivationAttempt, InstallationID: installationID, BindingID: bindingID, NativeRoot: request.Client.ConfigRoot}
	}
	if request.Plan.SelectedDelivery.IsZero() {
		return service.activatePreparedClient(ctx, installationID, bindingID, request)
	}
	before, err := service.activationBinding(installationID, bindingID)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	if !before.LocalEntryObservation.Equal(request.Plan.LocalEntryObservation) {
		return domain.ActivationOutcome{}, fmt.Errorf("planned observation predecessor changed before activation")
	}
	request.Plan.LocalEntryObservation = request.Plan.LocalEntryObservation.Clone()
	request.Plan.PreviousNativeObjects = append([]domain.NativeObjectOwnership(nil), request.Plan.PreviousNativeObjects...)
	outcome, err := service.activatePreparedClient(ctx, installationID, bindingID, request)
	after, loadErr := service.activationBinding(installationID, bindingID)
	if loadErr != nil {
		outcome.NativeEffect = domain.NativeEffectUncertain
		return outcome, loadErr
	}
	if !reflect.DeepEqual(before, after) {
		outcome.NativeEffect = domain.NativeEffectUncertain
		return outcome, fmt.Errorf("activation callback changed frozen binding authority")
	}
	outcome.LocalEntryObservation = outcome.LocalEntryObservation.Clone()
	if ctx.Err() != nil {
		return outcome, ctx.Err()
	}
	if err == nil && outcome.NativeEffect == domain.NativeEffectUncertain {
		return outcome, fmt.Errorf("selected native activation outcome is uncertain")
	}
	return outcome, err
}

// completeNativeRemoval records a known deactivation before a later managed
// directory transaction. A failed Deactivate leaves the attempt marker in
// place, because a partial native write cannot safely be inferred away.
func (service Service) completeNativeRemoval(installationID, bindingID string, removed bool) error {
	state, err := service.StateStore.Load()
	if err != nil {
		return err
	}
	for i, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		client, ok := installation.Clients[bindingID]
		if !ok || client.NativeActivationAttempt == "" {
			return fmt.Errorf("native removal attempt disappeared before recording its effect")
		}
		state.Installations = append([]domain.Installation(nil), state.Installations...)
		installation.Clients = cloneClientBindings(installation.Clients)
		if !client.SelectedDelivery.IsZero() && !removed {
			return fmt.Errorf("local removal did not establish a certain removal")
		}
		client.NativeActivationAttempt = ""
		client.PendingNativeIntent = nil
		if removed {
			ownedPackage := make([]domain.NativeObjectOwnership, 0, 1)
			for _, object := range client.NativeObjects {
				if object.Kind == "managed_package_directory" {
					ownedPackage = append(ownedPackage, object)
				}
			}
			client.NativeObjects = ownedPackage
			client.LocalEntryObservation = nil
			client.Activation = domain.ActivationNotRequired
			client.Verification = domain.VerificationPackageValid
		}
		client.UpdatedAt = service.now().Format(time.RFC3339Nano)
		installation.Clients[bindingID] = client
		installation.UpdatedAt = client.UpdatedAt
		state.Installations[i] = installation
		return service.persistLifecycleState(state)
	}
	return fmt.Errorf("installation disappeared before recording native removal")
}

// A deliberate profile change must materialize the new projection even when
// portable package bytes/revision are unchanged. Stored operations have no host.
func nativeDialectProjectionChanged(client domain.ClientBinding, plan domain.DeliveryPlan) bool {
	if client.ClientID != "opencode" || plan.OpenCodeHost == nil {
		return false
	}
	authority, ok := plan.OpenCodeHost.(interface{ ConfigDialect() string })
	if !ok {
		return false
	}
	target := authority.ConfigDialect()
	for _, object := range client.NativeObjects {
		if object.Kind == "opencode_global_mcp_server" && target == "opencode_v2" || object.Kind == "opencode_v2_global_mcp_server" && target == "opencode_v1" {
			return true
		}
	}
	return false
}

func (service Service) activatePreparedClient(ctx context.Context, installationID, bindingID string, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if service.ClientPreparation != nil && !request.VerifyOnly {
		if err := service.ClientPreparation.RevalidateClient(ctx, request.Client, request.Plan); err != nil {
			return service.refusedPreparedActivation(installationID, bindingID, err)
		}
	}
	return service.Activator.Activate(ctx, request)
}

// A host refusal happens before the adapter runs, so preserve the existing
// lifecycle and acknowledged objects instead of persisting an empty outcome.
func (service Service) refusedPreparedActivation(installationID, bindingID string, refusal error) (domain.ActivationOutcome, error) {
	binding, err := service.activationBinding(installationID, bindingID)
	if err != nil {
		return domain.ActivationOutcome{}, errors.Join(refusal, err)
	}
	outcome := lifecycleOutcome(binding)
	outcome.NativeEffect = domain.NativeEffectUnchanged
	return outcome, refusal
}
