package nativeconfig

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

// An error is insufficient restoration authority. Desired/unknown state must
// leave the target intact; definitely-old restores exact bytes and mode.
func TestTransitionRecoveryClosedStateAuthority(t *testing.T) {
	for _, scenario := range []string{"source-old", "source-desired", "target-old", "target-desired", "target-unknown", "target-desired-error", "target-foreign-edit", "target-mode-edit", "alternate", "wrong-mode"} {
		t.Run(scenario, func(t *testing.T) {
			req, path := transitionFixture(t, CodecOpenCode, "jsonc", "owned")
			if err := os.Chmod(path, 0640); err != nil {
				t.Fatal(err)
			}
			var prepared PreparedTransition
			req.PersistPrepared = func(p PreparedTransition) error { prepared = p; return nil }
			if _, err := New().ApplyDialectTransition(req); err != nil {
				t.Fatal(err)
			}
			if scenario == "source-old" || scenario == "source-desired" {
				if err := os.WriteFile(path, prepared.Original.Body, 0640); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "alternate" {
				mustWrite(t, req.Paths.JSON, `{}`)
			}
			if scenario == "wrong-mode" {
				if err := os.Chmod(path, 0600); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			complete := 0
			observation, err := New().ReconcileDialectTransition(TransitionRecoveryRequest{Paths: req.Paths, Prepared: prepared,
				Decide: func(o TransitionObservation) (TransitionStateDecision, error) {
					calls++
					switch scenario {
					case "source-old":
						return TransitionStateOld, nil
					case "source-desired", "target-desired":
						return TransitionStateDesired, nil
					case "target-old":
						return TransitionStateOld, errors.New("state Save failed with exact old reload")
					case "target-desired-error":
						return TransitionStateDesired, errors.New("durability unresolved")
					case "target-mode-edit":
						if err := os.Chmod(path, 0600); err != nil {
							t.Fatal(err)
						}
						return TransitionStateOld, errors.New("exact old")
					case "target-foreign-edit":
						mustWrite(t, path, `{"foreign":"new user bytes"}`)
						return TransitionStateOld, errors.New("exact old")
					default:
						return TransitionStateUnknown, errors.New("state read failed")
					}
				}, Complete: func(o TransitionObservation) error { complete++; return nil }})
			got, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			switch scenario {
			case "source-old":
				if err != nil || observation != TransitionSource || complete != 1 || !bytes.Equal(got, prepared.Original.Body) {
					t.Fatalf("source: %s %v", observation, err)
				}
			case "target-old":
				if err == nil || observation != TransitionSource || complete != 1 || !bytes.Equal(got, prepared.Original.Body) {
					t.Fatalf("restore: %s %v", observation, err)
				}
				info, _ := os.Stat(path)
				if info.Mode().Perm() != 0640 {
					t.Fatal("restore lost exact mode")
				}
			case "target-desired":
				if err != nil || observation != TransitionTarget || complete != 1 || !bytes.Equal(got, prepared.TargetBytes) {
					t.Fatalf("target: %s %v", observation, err)
				}
			case "target-mode-edit":
				info, _ := os.Stat(path)
				if !errors.Is(err, ErrConcurrentChange) || complete != 0 || info.Mode().Perm() != 0600 || !bytes.Equal(got, prepared.TargetBytes) {
					t.Fatal("foreign mode edit overwritten")
				}
			case "target-foreign-edit":
				if !errors.Is(err, ErrConcurrentChange) || complete != 0 || string(got) != `{"foreign":"new user bytes"}` {
					t.Fatal("foreign edit overwritten")
				}
			default:
				if err == nil || complete != 0 {
					t.Fatal("unsafe facts admitted")
				}
				if scenario == "alternate" || scenario == "wrong-mode" {
					if calls != 0 {
						t.Fatal("state callback granted authority before native classification")
					}
				}
			}
		})
	}
}

// Stored target hash alone cannot authorize an arbitrary path/receipt/document.
func TestTransitionRecoveryRejectsUnboundNativeFacts(t *testing.T) {
	for _, scenario := range []string{"hash", "path", "codec", "receipt", "logical-id", "oversize", "preimage"} {
		t.Run(scenario, func(t *testing.T) {
			req, path := transitionFixture(t, CodecOpenCode, "json", "owned")
			var p PreparedTransition
			req.PersistPrepared = func(v PreparedTransition) error { p = v; return nil }
			if _, err := New().ApplyDialectTransition(req); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "hash":
				p.TargetHash = "sha256:invalid"
			case "path":
				p.Path = path + "-foreign"
			case "codec":
				p.SourceCodec = p.TargetCodec
			case "receipt":
				p.Entries[0].TargetReceipt.Digest = p.Entries[0].SourceReceipt.Digest
			case "logical-id":
				p.Entries[0].LogicalID = "foreign"
			case "oversize":
				p.Original.Body = make([]byte, MaxTransitionConfigBytes+1)
			case "preimage":
				p.Original.Body = []byte(`{}`)
			}
			before := mustRead(t, path)
			if _, err := New().ReconcileDialectTransition(TransitionRecoveryRequest{Paths: req.Paths, Prepared: p, Decide: func(TransitionObservation) (TransitionStateDecision, error) {
				t.Fatal("invalid record reached state authority")
				return TransitionStateOld, nil
			}, Complete: func(TransitionObservation) error { t.Fatal("invalid record reached effect"); return nil }}); err == nil {
				t.Fatal("invalid facts accepted")
			}
			if before != mustRead(t, path) {
				t.Fatal("invalid facts mutated native bytes")
			}
		})
	}
}

// The independent compiled TEST writer uses production candidate locks. Both
// publication and finalization run under the reconciliation lease.
func TestTransitionRecoveryHoldsLeaseAcrossStateDecision(t *testing.T) {
	req, _ := transitionFixture(t, CodecOpenCode, "json", "owned")
	var p PreparedTransition
	req.PersistPrepared = func(v PreparedTransition) error { p = v; return nil }
	if _, err := New().ApplyDialectTransition(req); err != nil {
		t.Fatal(err)
	}
	var child *writerLockChild
	_, err := New().ReconcileDialectTransition(TransitionRecoveryRequest{Paths: req.Paths, Prepared: p,
		Decide: func(TransitionObservation) (TransitionStateDecision, error) {
			child = startWriterLockChild(t, "write", req.Paths, CodecOpenCodeV2, "")
			assertWriterLockChildBlocked(t, child)
			return TransitionStateDesired, nil
		},
		Complete: func(TransitionObservation) error { assertWriterLockChildBlocked(t, child); return nil }})
	if err != nil {
		t.Fatal(err)
	}
	child.wait(t)
	present, owned, err := New().Inspect(req.Paths, req.TargetCodec, "owned", &p.Entries[0].TargetReceipt)
	if err != nil || !present || !owned {
		t.Fatal("cooperating writer invalidated confirmed target", err)
	}
	// A stale source receipt cannot mutate or republish ownership after release.
	if _, err := New().Apply(Request{Paths: req.Paths, Codec: req.SourceCodec, Action: ActionUpdate, Name: "owned", Owned: &p.Entries[0].SourceReceipt, Server: Server{Type: "remote", URL: "https://stale.invalid"}}); err == nil {
		t.Fatal("stale source writer accepted")
	}
	present, owned, err = New().Inspect(req.Paths, req.TargetCodec, "owned", &p.Entries[0].TargetReceipt)
	if err != nil || !present || !owned {
		t.Fatal("stale source writer damaged target")
	}
}
