package installer

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
)

// These adversarial observational callbacks exercise the actual common sealed
// handle guard used by both public verification methods. Callback success is
// inert test input, never a native or product acceptance observation.
func TestPreparedProjectionObservationRetainsSealedGuards(t *testing.T) {
	for _, fault := range []string{"none", "cancel-before", "cancel-during", "state-during", "profile-during", "closed", "busy", "applied", "wrong-engine"} {
		t.Run(fault, func(t *testing.T) {
			engine, request := physicalEngine(t)
			installed, err := engine.Prepare(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			result, err := engine.Apply(t.Context(), installed, Decision{Confirmed: true})
			if closeErr := installed.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if err != nil {
				t.Fatal("real fixture install", result, err)
			}
			request.Operation = OpRepair
			request.OperationID = "projection-observe"
			prepared, err := engine.Prepare(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = prepared.Close() }()
			before, err := engine.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			calls := 0
			var restoreProfile func()
			defer func() {
				if restoreProfile != nil {
					restoreProfile()
				}
			}()
			observe := func(_ preparedBindingObservation) (bool, error) {
				calls++
				switch fault {
				case "cancel-during":
					cancel()
				case "state-during":
					changed, loadErr := engine.store.Load()
					if loadErr != nil {
						t.Fatal(loadErr)
					}
					for key, binding := range changed.Installations[0].Clients {
						binding.UpdatedAt = "2000-01-01T00:00:00Z"
						changed.Installations[0].Clients[key] = binding
					}
					if saveErr := engine.store.Save(changed); saveErr != nil {
						t.Fatal(saveErr)
					}
				case "profile-during":
					original := request.ClientConfigRoot + "-original"
					if renameErr := os.Rename(request.ClientConfigRoot, original); renameErr != nil {
						t.Fatal(renameErr)
					}
					if mkdirErr := os.Mkdir(request.ClientConfigRoot, 0700); mkdirErr != nil {
						t.Fatal(mkdirErr)
					}
					restoreProfile = func() {
						if removeErr := os.Remove(request.ClientConfigRoot); removeErr != nil {
							t.Fatal(removeErr)
						}
						if renameErr := os.Rename(original, request.ClientConfigRoot); renameErr != nil {
							t.Fatal(renameErr)
						}
					}
				}
				return true, nil
			}
			switch fault {
			case "cancel-before":
				cancel()
			case "closed":
				prepared.closed = true
				defer func() { prepared.closed = false }()
			case "busy":
				prepared.busy = true
				defer func() { prepared.busy = false }()
			case "applied":
				prepared.applied = true
			case "wrong-engine":
				prepared.engine = &Engine{}
			}
			complete, err := engine.observePreparedBinding(ctx, prepared, observe)
			switch fault {
			case "none":
				if err != nil || !complete || calls != 1 {
					t.Fatal("valid sealed observation refused", complete, calls, err)
				}
			case "cancel-before":
				if !errors.Is(err, context.Canceled) || calls != 0 {
					t.Fatal("canceled context reached callback", calls, err)
				}
			case "cancel-during":
				if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrPlanChanged) || complete {
					t.Fatal("callback cancellation accepted", complete, err)
				}
			case "state-during":
				if !errors.Is(err, ErrPlanChanged) || complete {
					t.Fatal("changed ownership state accepted", complete, err)
				}
			case "profile-during":
				if !errors.Is(err, ErrPlanChanged) || complete {
					t.Fatal("replaced profile accepted", complete, err)
				}
			default:
				if !errors.Is(err, ErrInvalidRequest) || calls != 0 {
					t.Fatal("invalid sealed handle reached callback", calls, err)
				}
			}
			if fault != "state-during" {
				after, loadErr := engine.store.Load()
				if loadErr != nil || !reflect.DeepEqual(before, after) {
					t.Fatal("read-only guard changed receipt state", loadErr)
				}
			}
		})
	}
}
