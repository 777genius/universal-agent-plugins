package usecase

import (
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

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
