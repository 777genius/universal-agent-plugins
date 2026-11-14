//go:build linux

package grok

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type recordedStateDrift struct {
	ports.NativeTransitionState
	afterFirst func() error
	loads      int
}

func (state *recordedStateDrift) Load() (domain.StateFileV2, error) {
	value, err := state.NativeTransitionState.Load()
	state.loads++
	if err == nil && state.loads == 1 {
		err = state.afterFirst()
	}
	return value, err
}

func TestRecordedRecoveryV3WholeStateDriftPrecedesConfigRead(t *testing.T) {
	fixture := newEnablementFixture(t, 3)
	fixture.attempt(t, true, false)
	observed := recordedObservation(t, fixture.recovery)
	originals := capturePrototypeFiles(t, fixture.config, observed.Root)
	cold, files := coldEnablementRecovery(fixture), &parentRefusalFileIO{}
	cold.Kernel = nativeconfig.NewWithFileIO(files)
	var foreignState []byte
	drift := &recordedStateDrift{NativeTransitionState: cold.State, afterFirst: func() error {
		state := fixture.loaded(t)
		state.Installations[0].UpdatedAt = "2026-11-14T16:00:00Z"
		if err := fixture.state.Save(state); err != nil {
			return err
		}
		var err error
		foreignState, err = os.ReadFile(fixture.state.Path)
		return err
	}}
	cold.State = drift
	if err := cold.recoverObservedRecord(t.Context(), observed); err == nil || drift.loads != 2 || files.reads != 0 || files.writes != 0 || files.removes != 0 {
		t.Fatalf("unrelated state drift reached config IO: %v loads=%d files=%+v", err, drift.loads, files)
	}
	assertPrototypeFilesUnchanged(t, originals)
	current, err := os.ReadFile(fixture.state.Path)
	if err != nil || len(foreignState) == 0 || !bytes.Equal(current, foreignState) {
		t.Fatal("recovery changed the external whole-state edit", err)
	}
}

func assertRecordedCanceledBoundary(t *testing.T, fixture enablementFixture, observed ports.NativeTransitionObservation, point string) {
	t.Helper()
	body, err := os.ReadFile(fixture.config)
	if err != nil || !bytes.Equal(body, fixture.original) {
		t.Fatal("canceled rollback lost the visible original config", err)
	}
	wantState := fixture.record.BeforeStateDigest
	if point == "decision_cleared" {
		wantState = fixture.record.AttemptStateDigest
	}
	if enablementStateDigest(fixture.loaded(t)) != wantState {
		t.Fatal("cancellation claimed a state decision that had not occurred")
	}
	if point == "record_removed" {
		assertRecordedSettlement(t, fixture, fixture.original, wantState)
		return
	}
	current, err := fixture.recovery.readRecord(fixture.record.OperationID)
	if err != nil || current.Hash != observed.AuthorityDigest || len(current.footer) != 0 {
		t.Fatal("cancellation lost the retained original header boundary", err)
	}
	if err := coldEnablementRecovery(fixture).recoverObservedRecord(t.Context(), observed); err == nil {
		t.Fatal("a stale pre-truncation observation adopted the new raw journal")
	}
	fresh := recordedObservation(t, fixture.recovery)
	if err := coldEnablementRecovery(fixture).recoverObservedRecord(t.Context(), fresh); err != nil {
		t.Fatal("cold retry could not settle the actual retained boundary", err)
	}
	assertRecordedSettlement(t, fixture, fixture.original, fixture.record.BeforeStateDigest)
}

func TestRecordedRecoveryV3CancellationAfterOwnedEffects(t *testing.T) {
	for _, point := range []string{"decision_cleared", "before_record_remove", "record_removed"} {
		t.Run(point, func(t *testing.T) {
			fixture, _ := recordedRemovalBoundary(t, "complete", "attempt", true)
			observed := recordedObservation(t, fixture.recovery)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cold := coldEnablementRecovery(fixture)
			hits := 0
			cold.journalPoint = func(current string) error {
				if current == point {
					hits++
					cancel()
				}
				return nil
			}
			if err := cold.recoverObservedRecord(ctx, observed); !errors.Is(err, context.Canceled) || hits != 1 {
				t.Fatalf("late cancellation became success or missed its owned boundary: %v hits=%d", err, hits)
			}
			assertRecordedCanceledBoundary(t, fixture, observed, point)
		})
	}
}

func recordedNoopRemovalFixture(t *testing.T) (enablementFixture, domain.StateFileV2) {
	t.Helper()
	fixture := newEnablementFixture(t, 3)
	fixture.attempt(t, true, true)
	if err := recoverPrototypeRecords(t.Context(), fixture.recovery); err != nil {
		t.Fatal(err)
	}
	// The user has already disabled this ID before the new removal starts.
	if err := os.WriteFile(fixture.config, fixture.original, 0600); err != nil {
		t.Fatal(err)
	}
	before := fixture.loaded(t)
	binding := before.Installations[0].Clients[fixture.record.BindingID]
	owned, record := fixture.record.object(), fixture.record
	record.Action, record.OperationID, record.PreviousObject = "remove", "op-remove-noop", &owned
	var err error
	record.Edit, fixture.next, err = prepareDisablementEdit(nativeconfig.FileSnapshot{Body: fixture.original, Mode: 0600, Exists: true}, record.PluginID)
	if err != nil || record.Edit.BeforeDigest != record.Edit.AfterDigest || record.Edit.Added != "" || record.Edit.Removed != "" {
		t.Fatal("fixture did not generate an actual no-op removal", err)
	}
	binding.NativeActivationAttempt = record.OperationID
	attempt := enablementState(before, 0, binding)
	if err := fixture.state.Save(attempt); err != nil {
		t.Fatal(err)
	}
	record.BeforeStateDigest, record.AttemptStateDigest, record.DesiredStateDigest = enablementStateDigest(before), enablementStateDigest(attempt), ""
	if err := fixture.recovery.writeRecord(record); err != nil {
		t.Fatal(err)
	}
	fixture.record, err = fixture.recovery.readRecord(record.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	binding.NativeActivationAttempt = ""
	binding.NativeObjects = binding.NativeObjects[:1]
	binding.Activation, binding.Verification = domain.ActivationNotRequired, domain.VerificationPackageValid
	binding.UpdatedAt = "2026-11-14T16:00:00.123456789Z"
	desired := enablementState(attempt, 0, binding)
	desired.Installations[0].UpdatedAt = binding.UpdatedAt
	return fixture, desired
}

func TestRecordedRecoveryV3NoopRemovalStillUsesStateDecision(t *testing.T) {
	for _, committed := range []bool{false, true} {
		name := "rollback"
		if committed {
			name = "complete"
		}
		t.Run(name, func(t *testing.T) {
			fixture, desired := recordedNoopRemovalFixture(t)
			handled, err := fixture.recovery.PrepareNativeRemovalDecision(t.Context(), fixture.record.InstallationID, fixture.record.BindingID, fixture.loaded(t), desired)
			if err != nil || !handled {
				t.Fatal("exact no-op removal decision refused", handled, err)
			}
			wantState := fixture.record.BeforeStateDigest
			if committed {
				if err := fixture.state.Save(desired); err != nil {
					t.Fatal(err)
				}
				wantState = enablementStateDigest(desired)
			}
			observed := recordedObservation(t, fixture.recovery)
			if err := coldEnablementRecovery(fixture).recoverObservedRecord(t.Context(), observed); err != nil {
				t.Fatal("identical config images obscured the actual state decision", err)
			}
			assertRecordedSettlement(t, fixture, fixture.original, wantState)
		})
	}
}
