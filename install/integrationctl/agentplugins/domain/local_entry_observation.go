package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
)

// LocalEntryObservation records a verified entry at one sealed revision. It
// does not establish ownership; the producer must independently verify it.
type LocalEntryObservation struct{ facts LocalEntryObservationFacts }

type LocalEntryObservationFacts struct {
	RevisionBasis SelectedDelivery `json:"revision_basis"`
	Enabled       bool             `json:"enabled"`
	ReceiptDigest string           `json:"receipt_digest"`
}

func NewLocalEntryObservation(facts LocalEntryObservationFacts) (*LocalEntryObservation, error) {
	result := &LocalEntryObservation{facts: facts}
	if err := result.Validate(); err != nil {
		return nil, err
	}
	result.facts.RevisionBasis = cloneObservationBasis(facts.RevisionBasis)
	return result, nil
}

func cloneObservationBasis(basis SelectedDelivery) SelectedDelivery {
	facts, ok := basis.LocalFacts()
	if !ok {
		return SelectedDelivery{}
	}
	return SelectedDelivery{mode: DeliveryVSCodeLocalV1, local: cloneLocalFacts(facts)}
}

func (o *LocalEntryObservation) Facts() LocalEntryObservationFacts {
	if o == nil {
		return LocalEntryObservationFacts{}
	}
	facts := o.facts
	facts.RevisionBasis = cloneObservationBasis(facts.RevisionBasis)
	return facts
}

func (o *LocalEntryObservation) Validate() error {
	if o == nil {
		return fmt.Errorf("local entry observation is absent")
	}
	if err := o.facts.RevisionBasis.Validate(); err != nil {
		return err
	}
	facts, ok := o.facts.RevisionBasis.LocalFacts()
	if !ok || !deliveryDigest(facts.ProjectionDigest) || !deliveryDigest(o.facts.ReceiptDigest) {
		return fmt.Errorf("local entry observation requires a sealed Local revision and receipt digest")
	}
	if o.facts.ReceiptDigest != observationReceiptDigest(facts, o.facts.Enabled) {
		return fmt.Errorf("local entry observation receipt digest differs from revision/value")
	}
	return nil
}

func (o *LocalEntryObservation) Clone() *LocalEntryObservation {
	if o == nil {
		return nil
	}
	return &LocalEntryObservation{facts: o.Facts()}
}

func (o *LocalEntryObservation) Equal(other *LocalEntryObservation) bool {
	if o == nil || other == nil {
		return o == other
	}
	return reflect.DeepEqual(o.facts, other.facts)
}

func (o *LocalEntryObservation) MarshalJSON() ([]byte, error) {
	if o == nil {
		return []byte("null"), nil
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(o.Facts())
	if len(body) > maxObservationBytes {
		return nil, fmt.Errorf("local observation exceeds byte budget")
	}
	return body, err
}

func (o *LocalEntryObservation) UnmarshalJSON(raw []byte) error {
	facts, err := decodeLocalEntryObservation(raw)
	if err != nil {
		return err
	}
	result, err := NewLocalEntryObservation(facts)
	if err != nil {
		return err
	}
	*o = *result
	return nil
}

// ValidateLocalEntryObservation checks linkage, including independently recorded
// selector ownership. A pending revision transition may retain the old sealed
// basis until a verified outcome replaces it; it must retain the same profile.
func (binding ClientBinding) ValidateLocalEntryObservation() error {
	o := binding.LocalEntryObservation
	if o == nil {
		return nil
	}
	if err := o.Validate(); err != nil {
		return err
	}
	basis := o.Facts().RevisionBasis
	f, _ := basis.LocalFacts()
	if !basis.SameProfile(binding.SelectedDelivery) || f.Registration.Selector != binding.TargetLocator || f.ProfileRoot != binding.NativeProfileRoot || !basis.OwnsProfileEntry(binding.NativeObjects) {
		return fmt.Errorf("local observation differs from binding profile or owned selector")
	}
	return nil
}

// This fixed wire shape is the public vscodeprofile v1 receipt digest contract.
// Recomputing it checks integrity only; it supplies no provenance or ownership.
func observationReceiptDigest(f LocalDeliveryFacts, enabled bool) string {
	type identity struct {
		SettingsPath     string `json:"settings_path"`
		ProfileID        string `json:"profile_id"`
		PluginRoot       string `json:"plugin_root"`
		PackageID        string `json:"package_id"`
		PackageDigest    string `json:"package_digest"`
		ProjectionDigest string `json:"projection_digest"`
	}
	wire := struct {
		Version  string   `json:"version"`
		Selector string   `json:"selector"`
		Identity identity `json:"identity"`
		Enabled  bool     `json:"enabled"`
		Digest   string   `json:"digest"`
	}{"1", "chat.pluginLocations", identity{f.SettingsPath, f.ProfileIdentity, f.Registration.Selector, f.Registration.ObjectID, f.CanonicalDigest, f.ProjectionDigest}, enabled, ""}
	body, _ := json.Marshal(wire)
	sum := sha256.Sum256(append([]byte("agentplugins-vscode-profile-v1\x00"), body...))
	return fmt.Sprintf("sha256:%x", sum)
}
