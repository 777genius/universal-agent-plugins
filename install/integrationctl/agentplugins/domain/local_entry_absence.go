package domain

import (
	"fmt"
	"reflect"
)

// LocalEntryAbsence classifies a parser-confirmed missing selector at one sealed
// revision. It grants no ownership or receipt. The adapter must first validate
// independently owned recorded authority; generic verification errors never
// qualify for absence restoration.
type LocalEntryAbsence struct{ basis SelectedDelivery }

func NewLocalEntryAbsence(basis SelectedDelivery) (*LocalEntryAbsence, error) {
	if err := basis.Validate(); err != nil {
		return nil, err
	}
	facts, ok := basis.LocalFacts()
	if !ok || !deliveryDigest(facts.ProjectionDigest) {
		return nil, fmt.Errorf("local absence requires an exact sealed revision")
	}
	return &LocalEntryAbsence{basis: cloneObservationBasis(basis)}, nil
}

func (e *LocalEntryAbsence) Error() string {
	return "recorded Local selector is absent; confirmed repair required"
}

// Matches compares the complete recorded revision, including qualification and
// desired selection. A zero classification or historical nil grants nothing.
func (e *LocalEntryAbsence) Matches(observation *LocalEntryObservation) bool {
	return e != nil && observation != nil && observation.Validate() == nil &&
		reflect.DeepEqual(e.basis, observation.Facts().RevisionBasis)
}
