package usecase

import (
	"context"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// activeNativeDelivery contains the complete desired projection, derived from
// the selected components and a verified installed package. Its native objects
// are never ownership evidence until an adapter confirms the external effect.
func (service Service) activeNativeDelivery(ctx context.Context, input AddInput, plan domain.DeliveryPlan, installation domain.Installation, client domain.ClientBinding) (domain.StagedDelivery, bool, error) {
	delivery := domain.StagedDelivery{ClientID: input.Client.ClientID, OwnedBase: plan.TargetRoot, ActivePath: client.TargetLocator, ArtifactDigest: managedDigest(client)}
	if delivery.ArtifactDigest == "" {
		return delivery, false, fmt.Errorf("managed package receipt is missing")
	}
	if !nativeLifecycleClient(input.Client.ClientID, plan.SelectedDelivery) {
		delivery.NativeObjects = append([]domain.NativeObjectOwnership(nil), client.NativeObjects...)
		return delivery, true, nil
	}
	if !packageRevisionMatches(client.PackageRevision, input.Envelope) {
		return delivery, false, fmt.Errorf("active package revision differs from the requested native projection")
	}
	projector, ok := service.Stager.(ports.ActiveNativeProjector)
	if !ok {
		return delivery, false, fmt.Errorf("stager cannot derive the installed native projection read-only")
	}
	dataPath := ""
	if packageNeedsPluginData(input.Envelope, plan) {
		receipt, found := installation.DataReceipts[client.DataReceiptID]
		if !found || receipt.DataReceiptID == "" || service.PluginData == nil {
			return delivery, false, fmt.Errorf("native projection has no confirmed PLUGIN_DATA receipt")
		}
		if err := service.PluginData.ValidateData(ctx, receipt); err != nil {
			return delivery, false, fmt.Errorf("validate native projection PLUGIN_DATA receipt: %w", err)
		}
		dataPath = receipt.Locator
	}
	projected, err := projector.ProjectActiveNative(ctx, input.Envelope, cloneLocalObservationPlan(plan), delivery.ArtifactDigest, dataPath)
	if err != nil {
		return delivery, false, err
	}
	for _, object := range client.NativeObjects {
		if object.Kind == "managed_package_directory" {
			delivery.NativeObjects = append(delivery.NativeObjects, object)
		}
	}
	if len(delivery.NativeObjects) != 1 {
		return delivery, false, fmt.Errorf("active package has no unique managed directory receipt")
	}
	seen := map[string]bool{delivery.NativeObjects[0].ObjectID: true}
	for _, object := range projected {
		if object.ObjectID == "" || object.Kind == "" || seen[object.ObjectID] || object.Kind == "managed_package_directory" {
			return delivery, false, fmt.Errorf("native projection has incomplete or duplicate ownership identity")
		}
		seen[object.ObjectID] = true
		delivery.NativeObjects = append(delivery.NativeObjects, object)
	}
	delivery.NativeObjects, err = retainRecordedLocalSelector(plan.SelectedDelivery, client, delivery.NativeObjects)
	if err != nil {
		return delivery, false, err
	}
	return delivery, confirmedNativeProjection(client.NativeObjects, delivery.NativeObjects), nil
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

// retainRecordedLocalSelector reconciles an independently confirmed selector
// before comparing desired projections. Empty routes do not relinquish it.
// Neither projected objects nor a matching native bool establish ownership.
func retainRecordedLocalSelector(selected domain.SelectedDelivery, client domain.ClientBinding, projected []domain.NativeObjectOwnership) ([]domain.NativeObjectOwnership, error) {
	if !client.SelectedDelivery.OwnsProfileEntry(client.NativeObjects) {
		return projected, nil
	}
	if err := validateRetainedLocalAuthority(selected, client); err != nil {
		return nil, err
	}
	facts, _ := client.SelectedDelivery.LocalFacts()
	expected := facts.Registration.Ownership(facts.SettingsPath)
	if _, ok := client.SelectedDelivery.CursorFacts(); ok {
		for _, object := range client.NativeObjects {
			if object.Kind == "cursor_user_stop" {
				expected = object
			}
		}
	}
	// OwnsProfileEntry established an exact recorded match. Require uniqueness
	// rather than allowing a duplicate or foreign receipt to supply authority.
	var recorded domain.NativeObjectOwnership
	found := false
	for _, object := range client.NativeObjects {
		if object.Kind == "managed_package_directory" {
			continue
		}
		if found || !reflect.DeepEqual(object, expected) {
			return nil, fmt.Errorf("recorded Local ownership is outside the exact selector")
		}
		recorded, found = object, true
	}
	present := false
	for _, object := range projected {
		if object.ObjectID == recorded.ObjectID || object.Kind == recorded.Kind {
			matches := retainedLocalSelectorMatchesProjection(selected, client, object, recorded)
			if present || !matches {
				return nil, fmt.Errorf("desired Local projection conflicts with recorded selector ownership")
			}
			present = true
		}
	}
	if present {
		return projected, nil
	}
	retained := append([]domain.NativeObjectOwnership(nil), projected...)
	return append(retained, recorded), nil
}

func retainedLocalSelectorMatchesProjection(selected domain.SelectedDelivery, client domain.ClientBinding, object, recorded domain.NativeObjectOwnership) bool {
	matches := reflect.DeepEqual(object, recorded)
	if f, ok := selected.CursorFacts(); ok && client.PendingNativeIntent != nil && client.PendingNativeIntent.Direction == domain.NativeIntentRegister {
		// Only acknowledgement of the recorded attempt may replace the old
		// receipt. Package projection continues to retain its original remainder.
		planned, _ := client.PendingNativeIntent.Delivery.CursorFacts()
		matches = object == selected.CursorOwnership(planned.PlannedReceipt) && f.EntryDigest == planned.EntryDigest
	}
	return matches
}

func validateRetainedLocalAuthority(selected domain.SelectedDelivery, client domain.ClientBinding) error {
	if err := client.SelectedDelivery.Validate(); err != nil {
		return err
	}
	if err := selected.Validate(); err != nil {
		return err
	}
	if f, ok := client.SelectedDelivery.CursorFacts(); ok {
		return validateRetainedCursorAuthority(selected, client, f)
	}
	facts, _ := client.SelectedDelivery.LocalFacts()
	if !client.SelectedDelivery.SameProfile(selected) || client.TargetLocator != facts.Registration.Selector || client.NativeProfileRoot != facts.ProfileRoot {
		return fmt.Errorf("retained Local selector differs from the managed binding profile")
	}
	if client.PackageRevision == nil || client.PackageRevision.TreeDigest != facts.CanonicalDigest || facts.ProjectionDigest == "" || managedDigest(client) != facts.ProjectionDigest {
		return fmt.Errorf("retained Local selector has no matching managed package revision")
	}
	packages := 0
	for _, object := range client.NativeObjects {
		if object.Kind == "managed_package_directory" {
			packages++
			if object.Path != client.TargetLocator || object.ManagedDigest != facts.ProjectionDigest {
				return fmt.Errorf("retained Local selector package receipt differs from the binding")
			}
		}
	}
	if packages != 1 {
		return fmt.Errorf("retained Local selector requires one managed package receipt")
	}
	if client.PendingNativeIntent != nil {
		return client.PendingNativeIntent.Validate(client)
	}
	if client.NativeActivationAttempt != "" {
		return fmt.Errorf("retained Local selector has an unresolved unqualified native attempt")
	}
	return nil
}

func validateRetainedCursorAuthority(selected domain.SelectedDelivery, client domain.ClientBinding, facts domain.CursorDeliveryFacts) error {
	if err := client.SelectedDelivery.ValidateCursorObjects(client.NativeObjects); err != nil {
		return err
	}
	if !client.SelectedDelivery.SameSelection(selected) || client.NativeProfileRoot != facts.ProfileRoot || client.PackageRevision == nil || client.PackageRevision.TreeDigest != facts.CanonicalDigest || facts.ProjectionDigest == "" || managedDigest(client) != facts.ProjectionDigest {
		return fmt.Errorf("retained Cursor receipt differs from binding authority")
	}
	packages := 0
	for _, object := range client.NativeObjects {
		if object.Kind == "managed_package_directory" {
			packages++
			if object.Path != client.TargetLocator || object.ManagedDigest != facts.ProjectionDigest {
				return fmt.Errorf("retained Cursor package receipt differs from binding")
			}
		}
	}
	if packages != 1 {
		return fmt.Errorf("retained Cursor requires one managed package receipt")
	}
	if client.PendingNativeIntent != nil {
		return client.PendingNativeIntent.Validate(client)
	}
	if client.NativeActivationAttempt != "" {
		return fmt.Errorf("unresolved Cursor attempt")
	}
	return nil
}
