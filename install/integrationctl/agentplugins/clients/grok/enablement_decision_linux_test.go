//go:build linux

package grok

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func removalEnablementFixture(t *testing.T) (enablementFixture, domain.StateFileV2) {
	t.Helper()
	fixture := newEnablementFixture(t)
	fixture.attempt(t, true, true)
	if err := fixture.recovery.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	original := append(bytes.Clone(fixture.next), []byte("[foreign]\nlate='retained'\n")...)
	if err := os.WriteFile(fixture.config, original, 0600); err != nil {
		t.Fatal(err)
	}
	before := fixture.loaded(t)
	binding := before.Installations[0].Clients[fixture.record.BindingID]
	owned, record := fixture.record.object(), fixture.record
	record.Action, record.OperationID, record.PreviousObject = "remove", "op-remove", &owned
	var err error
	record.Edit, fixture.next, err = prepareDisablementEdit(nativeconfig.FileSnapshot{Body: original, Mode: 0600, Exists: true}, record.PluginID)
	if err != nil {
		t.Fatal(err)
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
	fixture.original = original
	binding.NativeActivationAttempt = ""
	binding.NativeObjects = binding.NativeObjects[:1]
	binding.Activation, binding.Verification = domain.ActivationNotRequired, domain.VerificationPackageValid
	binding.UpdatedAt = "2026-11-13T23:00:00.123456789Z"
	desired := enablementState(attempt, 0, binding)
	desired.Installations[0].UpdatedAt = binding.UpdatedAt
	return fixture, desired
}

func applyRemovalFixture(t *testing.T, fixture enablementFixture) {
	t.Helper()
	if err := os.WriteFile(fixture.config, fixture.next, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestEnablementRemovalDecisionReopensExactCoreBoundaries(t *testing.T) {
	for _, boundary := range []string{"journal", "native", "decision", "state", "cleanup"} {
		t.Run(boundary, func(t *testing.T) {
			fixture, desired := removalEnablementFixture(t)
			if boundary != "journal" {
				applyRemovalFixture(t, fixture)
			}
			if boundary == "decision" || boundary == "state" || boundary == "cleanup" {
				handled, err := fixture.recovery.PrepareNativeRemovalDecision(t.Context(), fixture.record.InstallationID, fixture.record.BindingID, fixture.loaded(t), desired)
				if err != nil || !handled {
					t.Fatal("exact core decision refused", handled, err)
				}
			}
			if boundary == "state" || boundary == "cleanup" {
				if err := fixture.state.Save(desired); err != nil {
					t.Fatal(err)
				}
			}
			if err := fixture.recovery.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
			want, stateDigest := fixture.original, fixture.record.BeforeStateDigest
			if boundary == "state" || boundary == "cleanup" {
				want, stateDigest = fixture.next, enablementStateDigest(desired)
			}
			body, err := os.ReadFile(fixture.config)
			if err != nil || !bytes.Equal(body, want) || enablementStateDigest(fixture.loaded(t)) != stateDigest {
				t.Fatal("config/state decision differs", err)
			}
			if _, err := os.Stat(fixture.record.ArtifactPath); err != nil {
				t.Fatal("package removed before core directory transaction", err)
			}
			if err := fixture.recovery.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func appendFixtureFooter(t *testing.T, fixture enablementFixture, footer []byte) {
	t.Helper()
	path, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(footer)
	if err := errors.Join(writeErr, file.Sync(), file.Close()); err != nil {
		t.Fatal(err)
	}
}

func TestEnablementPartialRemovalDecisionOnlyRollsBackExactPendingState(t *testing.T) {
	for _, length := range []int{1, len(enablementDecisionPrefix) - 1, len(enablementDecisionPrefix), enablementDecisionLength / 2, enablementDecisionLength - 1} {
		for _, commit := range []bool{false, true} {
			t.Run(fmt.Sprintf("prefix-%d/committed-%t", length, commit), func(t *testing.T) {
				fixture, desired := removalEnablementFixture(t)
				applyRemovalFixture(t, fixture)
				tail, err := decisionTail(fixture.record, enablementStateDigest(desired))
				if err != nil {
					t.Fatal(err)
				}
				appendFixtureFooter(t, fixture, tail[:length])
				if commit {
					if err := fixture.state.Save(desired); err != nil {
						t.Fatal(err)
					}
				}
				err = fixture.recovery.Recover(t.Context())
				if commit && err == nil || !commit && err != nil {
					t.Fatal("partial footer invented a state decision", err)
				}
				want := fixture.original
				if commit {
					want = fixture.next
				}
				body, readErr := os.ReadFile(fixture.config)
				if readErr != nil || !bytes.Equal(body, want) {
					t.Fatal("partial footer changed unowned config", readErr)
				}
			})
		}
	}
}

func TestEnablementRemovalDecisionRetryRequiresIdenticalDigestPrefix(t *testing.T) {
	fixture, desired := removalEnablementFixture(t)
	applyRemovalFixture(t, fixture)
	digest := enablementStateDigest(desired)
	tail, err := decisionTail(fixture.record, digest)
	if err != nil {
		t.Fatal(err)
	}
	appendFixtureFooter(t, fixture, tail[:enablementDecisionLength/2])
	partial, err := fixture.recovery.readRecord(fixture.record.OperationID)
	if err != nil || partial.decisionDigest != "" {
		t.Fatal("partial has commit authority", err)
	}
	if err := fixture.recovery.appendDecision(partial, enablementDigest([]byte("different target"))); err == nil {
		t.Fatal("different decision accepted")
	}
	if err := fixture.recovery.appendDecision(partial, digest); err != nil {
		t.Fatal(err)
	}
	complete, err := fixture.recovery.readRecord(fixture.record.OperationID)
	if err != nil || complete.decisionDigest != digest {
		t.Fatal("complete digest differs", err)
	}
	if err := fixture.recovery.appendDecision(complete, digest); err != nil {
		t.Fatal("idempotent confirmation", err)
	}
	if err := fixture.state.Save(desired); err != nil {
		t.Fatal(err)
	}
	if err := fixture.recovery.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestEnablementRemovalDecisionWriteFailuresAndSubstitutionsRetainAttempt(t *testing.T) {
	for _, point := range []string{"before_decision_write", "decision_written", "decision_synced"} {
		t.Run(point, func(t *testing.T) {
			fixture, desired := removalEnablementFixture(t)
			applyRemovalFixture(t, fixture)
			fixture.recovery.journalPoint = func(observed string) error {
				if observed == point {
					return errors.New("inert native decision durability failure")
				}
				return nil
			}
			handled, err := fixture.recovery.PrepareNativeRemovalDecision(t.Context(), fixture.record.InstallationID, fixture.record.BindingID, fixture.loaded(t), desired)
			if !handled || err == nil || enablementStateDigest(fixture.loaded(t)) != fixture.record.AttemptStateDigest {
				t.Fatal("failed footer changed state", handled, err)
			}
			fixture.recovery.journalPoint = nil
			if err := fixture.recovery.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
		})
	}
	fixture, desired := removalEnablementFixture(t)
	applyRemovalFixture(t, fixture)
	path, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
	fixture.recovery.journalPoint = replacementHook(t, path, "before_decision_write", false)
	if err := fixture.recovery.appendDecision(fixture.record, enablementStateDigest(desired)); err == nil {
		t.Fatal("replacement admitted")
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "foreign retained" {
		t.Fatal("footer changed replacement", err)
	}
}
func TestEnablementRemovalProcessDeathAndRealPartialAppend(t *testing.T) {
	for _, point := range []string{"before_callback", "short_footer", "decision_written", "decision_synced", "decision_cleared", "state_committed", "before_package_delete"} {
		t.Run(point, func(t *testing.T) {
			fixture, desired := removalEnablementFixture(t)
			applyRemovalFixture(t, fixture)
			runEnablementCrashChild(t, fixture, point, desired)
			if point == "short_footer" {
				record, err := fixture.recovery.readRecord(fixture.record.OperationID)
				if err != nil || len(record.footer) != 17 || record.decisionDigest != "" {
					t.Fatal("actual limited append did not leave a bounded non-authoritative prefix", err)
				}
			}
			if err := fixture.recovery.Recover(t.Context()); err != nil {
				t.Fatal("cannot reopen removal crash", err)
			}
			want, digest := fixture.original, fixture.record.BeforeStateDigest
			if point == "state_committed" || point == "before_package_delete" {
				want, digest = fixture.next, enablementStateDigest(desired)
			}
			body, err := os.ReadFile(fixture.config)
			if err != nil || !bytes.Equal(body, want) || enablementStateDigest(fixture.loaded(t)) != digest {
				t.Fatal("real process death lost config or exact persisted state", err)
			}
		})
	}
}
