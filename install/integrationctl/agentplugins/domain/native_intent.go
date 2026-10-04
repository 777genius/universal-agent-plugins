package domain

import (
	"crypto/sha256"
	"fmt"
	"reflect"
)

type NativeIntentDirection string

const (
	NativeIntentRegister NativeIntentDirection = "register"
	NativeIntentRemove   NativeIntentDirection = "remove"
)

// PendingNativeIntent is the owned decision persisted in the existing binding
// before native effects. Reverse removal authority is as durable as registration.
// SelectedDelivery carries only the owned entry, never foreign document bytes.
type PendingNativeIntent struct {
	PreviousCursorObject  NativeObjectOwnership  `json:"previous_cursor_object,omitzero"`
	ProfileAuthority      *ProfileAuthority      `json:"profile_authority,omitempty"`
	ProfileNamespace      string                 `json:"profile_namespace,omitempty"`
	LocalEntryObservation *LocalEntryObservation `json:"local_entry_observation,omitempty"`
	RemoveOwnedEntry      bool                   `json:"remove_owned_entry"`
	AttemptID             string                 `json:"attempt_id"`
	Direction             NativeIntentDirection  `json:"direction"`
	Delivery              SelectedDelivery       `json:"selected_delivery"`
}

func (intent PendingNativeIntent) Validate(binding ClientBinding) error {
	if err := intent.validatePredecessor(binding); err != nil {
		return err
	}
	if intent.AttemptID == "" || intent.AttemptID != binding.NativeActivationAttempt {
		return fmt.Errorf("pending native intent attempt differs from binding")
	}
	if intent.Direction != NativeIntentRegister && intent.Direction != NativeIntentRemove || intent.Direction == NativeIntentRegister && intent.RemoveOwnedEntry {
		return fmt.Errorf("pending native intent direction is invalid")
	}
	if intent.Delivery.IsZero() || !intent.Delivery.SameSelection(binding.SelectedDelivery) {
		return fmt.Errorf("pending native intent delivery differs from binding")
	}
	if err := intent.Delivery.Validate(); err != nil {
		return err
	}
	if intent.Direction == NativeIntentRemove && intent.RemoveOwnedEntry != binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) {
		return fmt.Errorf("pending reverse decision differs from confirmed entry ownership")
	}
	if f, ok := intent.Delivery.CursorFacts(); ok {
		if !reflect.DeepEqual(intent.Delivery, binding.SelectedDelivery) {
			return fmt.Errorf("pending Cursor packet differs from binding attempt packet")
		}
		if binding.ClientID != string(ClientCursor) || binding.Scope != string(ScopeUser) || binding.NativeProfileRoot != f.ProfileRoot || f.ProjectionDigest == "" || f.ProjectionDigest != binding.SelectedDelivery.ProjectionDigest() || binding.PackageRevision == nil || f.CanonicalDigest != binding.PackageRevision.TreeDigest || f.CanonicalDigest != binding.SelectedDelivery.CanonicalDigest() {
			return fmt.Errorf("pending Cursor intent revision/profile differs from binding")
		}
		if err := binding.SelectedDelivery.ValidateCursorObjects(binding.NativeObjects); err != nil {
			return err
		}
		var previous NativeObjectOwnership
		for _, object := range binding.NativeObjects {
			if object.Kind == "cursor_user_stop" {
				previous = object
			}
		}
		if intent.PreviousCursorObject != previous {
			return fmt.Errorf("pending Cursor original predecessor changed")
		}
		return nil
	}
	if intent.PreviousCursorObject != (NativeObjectOwnership{}) {
		return fmt.Errorf("Cursor predecessor on Local intent")
	}
	facts, _ := intent.Delivery.LocalFacts()
	bound, _ := binding.SelectedDelivery.LocalFacts()
	if facts.ProjectionDigest == "" || facts.ProjectionDigest != bound.ProjectionDigest || facts.CanonicalDigest != bound.CanonicalDigest || facts.Registration.Selector != binding.TargetLocator || binding.PackageRevision == nil || facts.CanonicalDigest != binding.PackageRevision.TreeDigest {
		return fmt.Errorf("pending native intent package/projection authority is incomplete")
	}
	return nil
}

// Ownership is the selected adapter's receipt for a profile entry. The digest
// binds the exact bool, including false, rather than unrelated document bytes.
func (entry OwnedProfileEntry) Ownership(settingsPath string) NativeObjectOwnership {
	value := "null"
	if entry.DesiredValue != nil {
		value = fmt.Sprint(*entry.DesiredValue)
	}
	digest := sha256.Sum256([]byte(value))
	return NativeObjectOwnership{ObjectID: entry.ObjectID, Kind: "profile_plugin_location", Path: settingsPath, LogicalName: entry.Selector, ManagedDigest: fmt.Sprintf("sha256:%x", digest), ProtectionClass: "owned_selector"}
}

func (d SelectedDelivery) OwnsProfileEntry(objects []NativeObjectOwnership) bool {
	if _, ok := d.CursorFacts(); ok {
		if d.ValidateCursorObjects(objects) != nil {
			return false
		}
		for _, object := range objects {
			if object.Kind == "cursor_user_stop" {
				return true
			}
		}
		return false
	}
	facts, ok := d.LocalFacts()
	if !ok {
		return false
	}
	expected := facts.Registration.Ownership(facts.SettingsPath)
	for _, object := range objects {
		if reflect.DeepEqual(object, expected) {
			return true
		}
	}
	return false
}

func (intent *PendingNativeIntent) Clone() *PendingNativeIntent {
	if intent == nil {
		return nil
	}
	clone := *intent
	clone.ProfileAuthority = CloneProfileAuthority(intent.ProfileAuthority)
	clone.LocalEntryObservation = intent.LocalEntryObservation.Clone()
	return &clone
}

func (intent PendingNativeIntent) validatePredecessor(binding ClientBinding) error {
	if !SameProfileAuthority(intent.ProfileAuthority, binding.ProfileAuthority) || intent.ProfileNamespace != binding.ProfileNamespace {
		return fmt.Errorf("pending intent physical authority differs from binding")
	}
	if !intent.LocalEntryObservation.Equal(binding.LocalEntryObservation) {
		return fmt.Errorf("pending native intent observation differs from frozen predecessor")
	}
	if err := binding.ValidateLocalEntryObservation(); err != nil {
		return err
	}
	return nil
}
