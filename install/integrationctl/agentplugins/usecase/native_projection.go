package usecase

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// activeNativeDelivery contains the complete desired projection, derived from
// the selected components and a verified installed package. Its native objects
// are never ownership evidence until an adapter confirms the external effect.
// The settled result reports a complete projection that is only complete
// because no adapter action could finish the pending manual registration.
func (service Service) activeNativeDelivery(ctx context.Context, input AddInput, plan domain.DeliveryPlan, installation domain.Installation, client domain.ClientBinding) (delivery domain.StagedDelivery, complete, settled bool, err error) {
	delivery = domain.StagedDelivery{ClientID: input.Client.ClientID, OwnedBase: plan.TargetRoot, ActivePath: client.TargetLocator, ArtifactDigest: managedDigest(client)}
	if delivery.ArtifactDigest == "" {
		return delivery, false, false, fmt.Errorf("managed package receipt is missing")
	}
	if !nativeProjectionClient(input.Client.ClientID, plan.SelectedDelivery, input.Scope) {
		delivery.NativeObjects = append([]domain.NativeObjectOwnership(nil), client.NativeObjects...)
		complete = service.nonNativeRegistrationComplete(input, plan, client, delivery)
		settled = complete && service.manualRegistrationSettled(input, plan, client, delivery)
		return delivery, complete, settled, nil
	}
	if !packageRevisionMatches(client.PackageRevision, input.Envelope) {
		return delivery, false, false, fmt.Errorf("active package revision differs from the requested native projection")
	}
	projector, ok := service.Stager.(ports.ActiveNativeProjector)
	if !ok {
		return delivery, false, false, fmt.Errorf("stager cannot derive the installed native projection read-only")
	}
	dataPath := ""
	if packageNeedsPluginData(input.Envelope, plan) {
		receipt, found := installation.DataReceipts[client.DataReceiptID]
		if !found || receipt.DataReceiptID == "" || service.PluginData == nil {
			return delivery, false, false, fmt.Errorf("native projection has no confirmed PLUGIN_DATA receipt")
		}
		if err := service.PluginData.ValidateData(ctx, receipt); err != nil {
			return delivery, false, false, fmt.Errorf("validate native projection PLUGIN_DATA receipt: %w", err)
		}
		dataPath = receipt.Locator
	}
	projected, err := projector.ProjectActiveNative(ctx, input.Envelope, cloneLocalObservationPlan(plan), delivery.ArtifactDigest, dataPath)
	if err != nil {
		return delivery, false, false, err
	}
	for _, object := range client.NativeObjects {
		if object.Kind == "managed_package_directory" {
			delivery.NativeObjects = append(delivery.NativeObjects, object)
		}
	}
	if len(delivery.NativeObjects) != 1 {
		return delivery, false, false, fmt.Errorf("active package has no unique managed directory receipt")
	}
	delivery.NativeObjects, err = desiredNativeObjects(plan, client, delivery.NativeObjects[0], projected)
	if err != nil {
		return delivery, false, false, err
	}
	if confirmedNativeProjection(client.NativeObjects, delivery.NativeObjects) {
		return delivery, true, false, nil
	}
	// Classify before replacing the projection: the acting request carries the
	// full desired delivery, and adapters gate on its native objects.
	settled = service.manualRegistrationSettled(input, plan, client, delivery)
	if settled {
		// A deliberately pending manual registration is a stable projection:
		// no activation can finish it until the adapter can act again, so
		// update verifies the recorded objects instead of churning an attempt.
		delivery.NativeObjects = append([]domain.NativeObjectOwnership(nil), client.NativeObjects...)
		return delivery, true, true, nil
	}
	return delivery, false, false, nil
}

// desiredNativeObjects assembles the unique owned identity of the desired
// projection: the managed package directory receipt first, then each projected
// object, then a reconciled recorded Local selector.
func desiredNativeObjects(plan domain.DeliveryPlan, client domain.ClientBinding, managed domain.NativeObjectOwnership, projected []domain.NativeObjectOwnership) ([]domain.NativeObjectOwnership, error) {
	objects := make([]domain.NativeObjectOwnership, 0, len(projected)+1)
	objects = append(objects, managed)
	seen := map[string]bool{managed.ObjectID: true}
	for _, object := range projected {
		if object.ObjectID == "" || object.Kind == "" || seen[object.ObjectID] || object.Kind == "managed_package_directory" {
			return nil, fmt.Errorf("native projection has incomplete or duplicate ownership identity")
		}
		seen[object.ObjectID] = true
		objects = append(objects, object)
	}
	return retainRecordedLocalSelector(plan.SelectedDelivery, client, objects)
}

// manualRegistrationSettled reports an incomplete projection that no adapter
// action could complete for this exact request. The classifier sees the same
// request the acting adapter would, so a client whose adapter could act on
// this delivery keeps the projection incomplete and registration completes
// when its conditions arrive.
func (service Service) manualRegistrationSettled(input AddInput, plan domain.DeliveryPlan, client domain.ClientBinding, delivery domain.StagedDelivery) bool {
	if client.Activation != domain.ActivationManual || client.NativeActivationAttempt != "" || client.PendingNativeIntent != nil {
		return false
	}
	classifier, ok := service.Activator.(ports.AutomaticActivationClassifier)
	if !ok {
		return false
	}
	return !classifier.AutomaticallyActivates(activationClassifierRequest(input, plan, client, delivery))
}

// activationClassifierRequest mirrors the request the acting adapter would
// receive, so classifiers gate on the same delivery and prior objects.
func activationClassifierRequest(input AddInput, plan domain.DeliveryPlan, client domain.ClientBinding, delivery domain.StagedDelivery) domain.ActivationRequest {
	return domain.ActivationRequest{
		Client:                input.Client,
		Plan:                  plan,
		Delivery:              delivery,
		BackendExecutable:     input.BackendExecutable,
		PreviousNativeObjects: client.NativeObjects,
	}
}

// requireCurrentCommandProjection derives the installed native projection of
// an unchanged binding ahead of time, so a tree rendered under other command
// rules stops a preview or a group with ErrProjectionRefreshRequired before
// any effect. Other projection failures stay with the step that uses it.
func (service Service) requireCurrentCommandProjection(ctx context.Context, input AddInput, plan domain.DeliveryPlan, installation domain.Installation, client domain.ClientBinding) error {
	if !rendersCommands(input.Envelope, input.Client.ClientID) || !nativeLifecycleClient(input.Client.ClientID, plan.SelectedDelivery, input.Scope) {
		return nil
	}
	if _, _, _, err := service.activeNativeDelivery(ctx, input, plan, installation, client); errors.Is(err, ErrProjectionRefreshRequired) {
		return err
	}
	return nil
}

// nonNativeRegistrationComplete separates CLI effects from package reuse.
// Explicit attestation still runs the adapter's verifier; recognized negative
// evidence cannot be overridden. Unattempted registration still needs effects.
func (service Service) nonNativeRegistrationComplete(input AddInput, plan domain.DeliveryPlan, client domain.ClientBinding, delivery domain.StagedDelivery) bool {
	classifier, ok := service.Activator.(ports.AutomaticActivationClassifier)
	cliBacked := ok && classifier.AutomaticallyActivates(activationClassifierRequest(input, plan, client, delivery))
	if plan.SelectedDelivery.EffectiveTraitsForScope(input.Client.ClientID, input.Scope).LifecycleKind == domain.LifecycleCLIRegistry && cliBacked {
		return input.ActivationComplete || client.Activation == domain.ActivationActive && (client.Verification == domain.VerificationInstalled || client.Verification == domain.VerificationRuntime)
	}
	return true
}

func confirmedNativeProjection(confirmed, desired []domain.NativeObjectOwnership) bool {
	if len(confirmed) != len(desired) {
		return false
	}
	byID := make(map[string]domain.NativeObjectOwnership, len(confirmed))
	for _, object := range confirmed {
		if object.ObjectID == "" || object.Kind == "" {
			return false
		}
		if _, duplicate := byID[object.ObjectID]; duplicate {
			return false
		}
		byID[object.ObjectID] = object
	}
	for _, object := range desired {
		if !reflect.DeepEqual(byID[object.ObjectID], object) {
			return false
		}
	}
	return true
}
