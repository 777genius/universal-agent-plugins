package usecase

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

// nativeRemovalAttempt freezes the exact state around this invocation's marker
// before any adapter callback. A later read can validate it, never replace it.
type nativeRemovalAttempt struct {
	before, attempted domain.StateFileV2
	attemptID         string
	attemptedDigest   string
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
	digest, err := transaction.ComparableStateDigest(frozen[1])
	if err != nil {
		return nil, err
	}
	return &nativeRemovalAttempt{before: frozen[0], attempted: frozen[1], attemptID: attemptID, attemptedDigest: digest}, nil
}

func (attempt *nativeRemovalAttempt) identity(installationID, bindingID string) domain.NativeAttemptIdentity {
	if attempt != nil {
		for _, installation := range attempt.attempted.Installations {
			if installation.InstallationID == installationID {
				binding, found := installation.Clients[bindingID]
				if found && binding.NativeActivationAttempt == attempt.attemptID {
					return domain.NativeAttemptIdentity{OperationID: attempt.attemptID, InstallationID: installationID,
						BindingID: bindingID, NativeRoot: binding.NativeProfileRoot, StateDigest: attempt.attemptedDigest}
				}
			}
		}
	}
	return domain.NativeAttemptIdentity{}
}

func (service Service) beginNativeRemovalAttempt(installationID, bindingID string, client domain.ClientBinding, predecessor domain.StateFileV2) (*nativeRemovalAttempt, error) {
	return service.beginNativeAttemptWithPredecessor(installationID, bindingID, domain.NativeIntentRemove, client.SelectedDelivery, client.LocalEntryObservation.Clone(), &predecessor)
}

// acknowledgeCertainRemoval closes an owned attempt whose failed callback
// positively proved its effect: no native effect restores the predecessor, and
// settled partial progress keeps its exact ownership but records that the
// client is no longer verified. Unknown errors retain the recovery guard.
// The mutation lock is held by Remove/RemoveGroup across comparison and save;
// persistence uses the kernel's existing visibility and durability handling.
func (service Service) acknowledgeCertainRemoval(attempt *nativeRemovalAttempt, outcome domain.DeactivationOutcome, removalErr error) error {
	settled := outcome.NativeEffect == domain.NativeEffectCommitted && outcome.SettledPartialRemoval
	if removalErr == nil || attempt == nil || outcome.NativeEffect != domain.NativeEffectUnchanged && !settled {
		return removalErr
	}
	current, err := service.StateStore.Load()
	if err != nil {
		return errors.Join(removalErr, fmt.Errorf("inspect certain native removal %s: %w", attempt.attemptID, err))
	}
	if !reflect.DeepEqual(current, attempt.attempted) {
		return errors.Join(removalErr, fmt.Errorf("certain native removal %s changed frozen predecessor; retain uncertainty", attempt.attemptID))
	}
	decision := attempt.before
	if settled {
		decision = attempt.partialRemovalDecision(service.now())
	}
	kernel := service.Kernel
	kernel.StateStore = service.StateStore
	if err := kernel.PersistStateDecision(attempt.attempted, decision); err != nil {
		return errors.Join(removalErr, fmt.Errorf("acknowledge certain native removal %s: %w", attempt.attemptID, err))
	}
	return removalErr
}

// partialRemovalDecision is the predecessor state with the attempted binding
// marked failed: its client registration changed, its recorded objects did not.
func (attempt *nativeRemovalAttempt) partialRemovalDecision(now time.Time) domain.StateFileV2 {
	decision := attempt.before
	decision.Installations = append([]domain.Installation(nil), decision.Installations...)
	for i, installation := range decision.Installations {
		key := attempt.bindingID(installation.InstallationID)
		if _, found := installation.Clients[key]; !found {
			continue
		}
		installation.Clients = cloneClientBindings(installation.Clients)
		client := installation.Clients[key]
		client.Activation, client.Verification = domain.ActivationFailed, domain.VerificationFailed
		client.UpdatedAt = now.Format(time.RFC3339Nano)
		installation.Clients[key], installation.UpdatedAt = client, client.UpdatedAt
		decision.Installations[i] = installation
	}
	return decision
}

// bindingID names the binding that carries this attempt's marker.
func (attempt *nativeRemovalAttempt) bindingID(installationID string) string {
	for _, installation := range attempt.attempted.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		for key, binding := range installation.Clients {
			if binding.NativeActivationAttempt == attempt.attemptID {
				return key
			}
		}
	}
	return ""
}
