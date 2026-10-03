package usecase

import (
	"context"
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
		if !selected.IsZero() {
			client.PendingNativeIntent = &domain.PendingNativeIntent{LocalEntryObservation: client.LocalEntryObservation.Clone(), AttemptID: attemptID, Direction: direction, Delivery: client.SelectedDelivery, RemoveOwnedEntry: direction == domain.NativeIntentRemove && client.SelectedDelivery.OwnsProfileEntry(client.NativeObjects)}
			if err := client.PendingNativeIntent.Validate(client); err != nil {
				return err
			}
		}
		installation.Clients[bindingID] = client
		state.Installations[i] = installation
		return service.persistLifecycleState(state)
	}
	return fmt.Errorf("installation disappeared before native activation")
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
	if nativeLifecycleClient(request.Client.ClientID, request.Plan.SelectedDelivery) && !request.VerifyOnly {
		if err := service.beginNativeAttemptWithObservation(installationID, bindingID, domain.NativeIntentRegister, request.Plan.SelectedDelivery, request.Plan.LocalEntryObservation.Clone()); err != nil {
			return domain.ActivationOutcome{}, err
		}
	}
	if request.Plan.SelectedDelivery.IsZero() {
		return service.Activator.Activate(ctx, request)
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
	outcome, err := service.Activator.Activate(ctx, request)
	after, loadErr := service.activationBinding(installationID, bindingID)
	if loadErr != nil {
		return outcome, loadErr
	}
	if !reflect.DeepEqual(before, after) {
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
