package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// A guard around the existing store, not an alternate recovery/state engine.
// The facade retains the actual lock. Each kernel read and acknowledgement
// write must still see the complete last approved or recorded-successful scope.
type nativeRecoveryScope struct {
	engine   *Engine
	expected RecoveryObservation
	journals *journalRecoveryFence
}

func (s *nativeRecoveryScope) confirm() error {
	view, err := s.engine.observe()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPlanChanged, err)
	}
	if !reflect.DeepEqual(view.Recovery, s.expected) {
		return ErrPlanChanged
	}
	return nil
}
func (s *nativeRecoveryScope) RequireMutationReady() error {
	if guard, ok := s.engine.store.(interface{ RequireMutationReady() error }); ok {
		if err := guard.RequireMutationReady(); err != nil {
			return err
		}
	}
	return s.confirm()
}
func (s *nativeRecoveryScope) Load() (domain.StateFileV2, error) {
	if err := s.confirm(); err != nil {
		return domain.StateFileV2{}, err
	}
	state, err := s.engine.store.Load()
	if err != nil {
		return state, err
	}
	digest, err := recoveryStateDigest(state)
	if err != nil {
		return domain.StateFileV2{}, err
	}
	if digest != s.expected.StateDigest {
		return domain.StateFileV2{}, ErrPlanChanged
	}
	if err := s.confirm(); err != nil {
		return domain.StateFileV2{}, err
	}
	return state, nil
}
func (s *nativeRecoveryScope) Save(state domain.StateFileV2) error {
	if err := s.advanceJournals(); err != nil {
		return err
	}
	if err := s.confirm(); err != nil {
		return err
	}
	if s.journals != nil {
		if err := s.journals.validateState(state); err != nil {
			return err
		}
	}
	digest, err := recoveryStateDigest(state)
	if err != nil {
		return err
	}
	if err := s.engine.store.Save(state); err != nil {
		return err
	}
	after, err := s.engine.observe()
	if err != nil {
		return err
	}
	if after.Recovery.StateDigest != digest || !reflect.DeepEqual(after.Recovery.Journals, s.expected.Journals) {
		return ErrPlanChanged
	}
	s.expected = after.Recovery
	return nil
}

// MarshalJSON renders the existing domain intent for diagnostics. Unmarshal
// does not restore Intent or Binding.SelectedDelivery (both json:"-"); a JSON
// round trip cannot create the typed observation accepted by Recover.
func (pending PendingNativeIntent) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Binding           BindingFacts
		NativeProfileRoot string
		Digest            string
		Intent            domain.PendingNativeIntent
	}{pending.Binding, pending.NativeProfileRoot, pending.Digest, pending.Intent})
}

// Fence the reconciler's return before Service acknowledges ownership. A late
// foreign binding edit must retain its attempt, even after a native read/write.
type scopedNativeReconciler struct {
	scope   *nativeRecoveryScope
	inner   usecase.NativeIntentReconciler
	pending PendingNativeIntent
}

func (r scopedNativeReconciler) ReconcileNativeIntent(ctx context.Context, intent domain.PendingNativeIntent) (domain.ActivationOutcome, error) {
	if !reflect.DeepEqual(intent, r.pending.Intent) {
		return domain.ActivationOutcome{}, ErrPlanChanged
	}
	if err := r.confirm(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome, err := r.inner.ReconcileNativeIntent(ctx, intent)
	if scopeErr := r.confirm(); scopeErr != nil {
		return outcome, errors.Join(scopeErr, err)
	}
	return outcome, err
}
func (r scopedNativeReconciler) confirm() error { return r.scope.confirm() }
