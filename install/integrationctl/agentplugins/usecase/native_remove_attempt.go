package usecase

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// nativeRemovalAttempt freezes the exact state around this invocation's marker
// before any adapter callback. A later read can validate it, never replace it.
type nativeRemovalAttempt struct {
	before, attempted domain.StateFileV2
	attemptID         string
}

func freezeNativeRemovalAttempt(before, attempted domain.StateFileV2, attemptID string) (*nativeRemovalAttempt, error) {
	// Snapshot through the persisted representation so neither a callback nor an
	// in-memory StateStore can mutate the token through shared slices or maps.
	body, err := json.Marshal([2]domain.StateFileV2{before, attempted})
	if err != nil {
		return nil, err
	}
	var frozen [2]domain.StateFileV2
	if err := json.Unmarshal(body, &frozen); err != nil {
		return nil, err
	}
	return &nativeRemovalAttempt{before: frozen[0], attempted: frozen[1], attemptID: attemptID}, nil
}

func (service Service) beginNativeRemovalAttempt(installationID, bindingID string, predecessor domain.ClientBinding) (*nativeRemovalAttempt, error) {
	return service.beginNativeAttemptWithPredecessor(installationID, bindingID, domain.NativeIntentRemove, predecessor.SelectedDelivery, predecessor.LocalEntryObservation.Clone(), &predecessor)
}

// acknowledgeUnchangedRemoval aborts only an owned attempt whose callback
// positively proved no native effect. Unknown errors retain the recovery guard.
// The mutation lock is held by Remove/RemoveGroup across comparison and save;
// persistence uses the kernel's existing visibility and durability handling.
func (service Service) acknowledgeUnchangedRemoval(attempt *nativeRemovalAttempt, outcome domain.DeactivationOutcome, removalErr error) error {
	if removalErr == nil || attempt == nil || outcome.NativeEffect != domain.NativeEffectUnchanged {
		return removalErr
	}
	current, err := service.StateStore.Load()
	if err != nil {
		return errors.Join(removalErr, fmt.Errorf("inspect unchanged native removal %s: %w", attempt.attemptID, err))
	}
	if !reflect.DeepEqual(current, attempt.attempted) {
		return errors.Join(removalErr, fmt.Errorf("unchanged native removal %s changed frozen predecessor; retain uncertainty", attempt.attemptID))
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	if err := kernel.PersistStateDecision(attempt.attempted, attempt.before); err != nil {
		return errors.Join(removalErr, fmt.Errorf("acknowledge unchanged native removal %s: %w", attempt.attemptID, err))
	}
	return removalErr
}
