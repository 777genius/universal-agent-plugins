package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

// Use the same indented JSON boundary as the durable StateStore. This focused
// store tests representation and state decisions, not physical proof issuance.
type nativeRemovalProofStore struct {
	body []byte
}

func (store *nativeRemovalProofStore) Save(state domain.StateFileV2) error {
	body, err := json.MarshalIndent(state, "", "  ")
	if err == nil {
		store.body = body
	}
	return err
}

func (store *nativeRemovalProofStore) Load() (domain.StateFileV2, error) {
	var state domain.StateFileV2
	err := json.Unmarshal(store.body, &state)
	return state, err
}

func nativeRemovalProofState(attempt string) domain.StateFileV2 {
	return domain.StateFileV2{
		SchemaVersion: domain.StateSchemaVersion,
		Installations: []domain.Installation{{
			InstallationID: "installation",
			Clients: map[string]domain.ClientBinding{
				"binding": {
					ClientBindingID:         "binding",
					NativeActivationAttempt: attempt,
					Receipts: []domain.MutationReceipt{{
						OperationID:    "owned",
						MutationType:   "directory_swap",
						DirectoryProof: json.RawMessage(`{"schema_version":7,"operation_id":"owned"}`),
					}},
				},
			},
		}},
	}
}

func changeNativeRemovalProof(store *nativeRemovalProofStore) error {
	state, err := store.Load()
	if err != nil {
		return err
	}
	binding := state.Installations[0].Clients["binding"]
	binding.Receipts[0].DirectoryProof = json.RawMessage(`{"schema_version":7,"operation_id":"foreign"}`)
	state.Installations[0].Clients["binding"] = binding
	return store.Save(state)
}

type nativeRemovalProofRecorder struct {
	store   *nativeRemovalProofStore
	foreign bool
}

func (r nativeRemovalProofRecorder) PrepareNativeRemovalDecision(context.Context, string, string, domain.StateFileV2, domain.StateFileV2) (bool, error) {
	if r.foreign {
		return false, changeNativeRemovalProof(r.store)
	}
	return false, nil
}

func (nativeRemovalProofRecorder) Recover(context.Context) error {
	return errors.New("unhandled native decision must not recover")
}

func TestNativeRemovalDecisionComparesPersistedDirectoryProof(t *testing.T) {
	for _, name := range []string{"unchanged", "foreign"} {
		t.Run(name, func(t *testing.T) {
			store := &nativeRemovalProofStore{}
			if err := store.Save(nativeRemovalProofState("attempt")); err != nil {
				t.Fatal(err)
			}
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			frozen, err := snapshotNativeRemovalState(before)
			if err != nil || reflect.DeepEqual(before, frozen) {
				t.Fatal("fixture did not reproduce the raw-proof representation mismatch", err)
			}
			service := Service{StateStore: store, Kernel: transaction.Kernel{
				NativeRecovery: nativeRemovalProofRecorder{store: store, foreign: name == "foreign"},
			}}
			err = service.persistNativeRemovalDecision(t.Context(), "installation", "binding", before, nativeRemovalProofState(""))
			if name == "unchanged" && err != nil {
				t.Fatal("unchanged durable proof refused", err)
			}
			if name == "foreign" && (err == nil || !strings.Contains(err.Error(), "changed durable predecessor")) {
				t.Fatal("foreign proof change was not refused", err)
			}
			checkNativeRemovalProofState(t, store, name)
		})
	}
}

func TestCertainNativeRemovalComparesPersistedDirectoryProof(t *testing.T) {
	for _, name := range []string{"unchanged", "foreign"} {
		t.Run(name, func(t *testing.T) {
			store := &nativeRemovalProofStore{}
			attempt, err := freezeNativeRemovalAttempt(nativeRemovalProofState(""), nativeRemovalProofState("attempt"), "attempt")
			if err != nil {
				t.Fatal(err)
			}
			if err := store.Save(attempt.attempted); err != nil {
				t.Fatal(err)
			}
			if name == "foreign" {
				if err := changeNativeRemovalProof(store); err != nil {
					t.Fatal(err)
				}
			}
			service := Service{StateStore: store}
			refusal := errors.New("native removal refused without effect")
			err = service.acknowledgeCertainRemoval(attempt, domain.DeactivationOutcome{NativeEffect: domain.NativeEffectUnchanged}, refusal)
			// The reverse check requires the original unwrapped sentinel on success.
			if !errors.Is(err, refusal) || (name == "unchanged" && !errors.Is(refusal, err)) {
				t.Fatal("original refusal was not preserved", err)
			}
			if name == "foreign" && !strings.Contains(err.Error(), "changed frozen predecessor") {
				t.Fatal("foreign proof change was not refused", err)
			}
			checkNativeRemovalProofState(t, store, name)
		})
	}
}

func checkNativeRemovalProofState(t *testing.T, store *nativeRemovalProofStore, name string) {
	t.Helper()
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := state.Installations[0].Clients["binding"]
	var proof struct {
		OperationID string `json:"operation_id"`
	}
	if err := json.Unmarshal(binding.Receipts[0].DirectoryProof, &proof); err != nil {
		t.Fatal(err)
	}
	if name == "unchanged" {
		if binding.NativeActivationAttempt != "" || proof.OperationID != "owned" {
			t.Fatal("exact native decision did not preserve owned proof")
		}
	} else if binding.NativeActivationAttempt != "attempt" || proof.OperationID != "foreign" {
		t.Fatal("refusal overwrote the changed durable predecessor")
	}
}
