//go:build linux

package grok

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func changeRecordedObservation(observed *ports.NativeTransitionObservation, field string) {
	switch field {
	case "root":
		observed.Root += "-different"
	case "digest":
		observed.Digest = enablementDigest([]byte("different raw bytes"))
	case "header":
		observed.AuthorityDigest = enablementDigest([]byte("different header"))
	case "binding":
		observed.BindingID += "-different"
	case "target":
		observed.TargetPath += "-different"
	case "phase":
		observed.Phase = "native_state_decision"
	}
}

func TestRecordedRecoveryV3ObservationCannotAdoptLaterAuthority(t *testing.T) {
	for _, field := range []string{"root", "digest", "header", "binding", "target", "phase", "raw-bytes"} {
		t.Run(field, func(t *testing.T) {
			fixture := newEnablementFixture(t, 3)
			fixture.attempt(t, true, false)
			observed := recordedObservation(t, fixture.recovery)
			journal := observed.Root
			if field == "raw-bytes" {
				body, err := os.ReadFile(journal)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(journal, append([]byte(" "), body...), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				changeRecordedObservation(&observed, field)
			}
			originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal)
			cold, files := coldEnablementRecovery(fixture), &parentRefusalFileIO{}
			cold.Kernel = nativeconfig.NewWithFileIO(files)
			if err := cold.recoverObservedRecord(t.Context(), observed); err == nil || files.reads != 0 || files.writes != 0 || files.removes != 0 {
				t.Fatalf("observation drift reached config IO: %v %+v", err, files)
			}
			assertPrototypeFilesUnchanged(t, originals)
		})
	}
}

func TestRecordedRecoveryV3ObservationRejectsChangedFooter(t *testing.T) {
	fixture, desired := removalEnablementFixture(t, 3)
	observed := recordedObservation(t, fixture.recovery)
	tail, err := decisionTail(fixture.record, enablementStateDigest(desired))
	if err != nil {
		t.Fatal(err)
	}
	appendFixtureFooter(t, fixture, tail)
	originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, observed.Root)
	cold, files := coldEnablementRecovery(fixture), &parentRefusalFileIO{}
	cold.Kernel = nativeconfig.NewWithFileIO(files)
	if err := cold.recoverObservedRecord(t.Context(), observed); err == nil || files.reads != 0 {
		t.Fatal("a later complete footer replaced the original observation", err)
	}
	assertPrototypeFilesUnchanged(t, originals)
}

func TestRecordedRecoveryV3OriginalParentPrecedesEveryConfigRead(t *testing.T) {
	for _, stage := range []string{"predecessor", "attempt", "committed"} {
		t.Run(stage, func(t *testing.T) {
			fixture := newEnablementFixture(t, 3)
			if stage != "predecessor" {
				fixture.attempt(t, true, stage == "committed")
			}
			journal, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
			originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal)
			replaceEnablementParent(t, fixture)
			cold, files := coldEnablementRecovery(fixture), &parentRefusalFileIO{}
			cold.Kernel = nativeconfig.NewWithFileIO(files)
			err := cold.recoverRecord(t.Context(), fixture.record.OperationID)
			if !errors.Is(err, directoryidentity.ErrChanged) || files.reads != 0 || files.writes != 0 || files.removes != 0 {
				t.Fatalf("replacement parent reached config IO: %v %+v", err, files)
			}
			assertPrototypeFilesUnchanged(t, originals)
		})
	}
}

func TestRecordedRecoveryV3FooterRequiresOriginalParentBeforeConfigRead(t *testing.T) {
	fixture, desired := removalEnablementFixture(t, 3)
	applyRemovalFixture(t, fixture)
	journal, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
	originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal)
	replaceEnablementParent(t, fixture)
	cold, files := coldEnablementRecovery(fixture), &parentRefusalFileIO{}
	cold.Kernel = nativeconfig.NewWithFileIO(files)
	handled, err := cold.PrepareNativeRemovalDecision(t.Context(), fixture.record.InstallationID, fixture.record.BindingID, fixture.loaded(t), desired)
	if !handled || !errors.Is(err, directoryidentity.ErrChanged) || files.reads != 0 || files.writes != 0 || files.removes != 0 {
		t.Fatalf("footer adopted a replacement parent before config IO: %t %v %+v", handled, err, files)
	}
	assertPrototypeFilesUnchanged(t, originals)
}

func TestRecordedRecoveryV3AllCompletionPathsRetainNativeLock(t *testing.T) {
	for _, stage := range []string{"predecessor", "attempt", "committed"} {
		t.Run(stage, func(t *testing.T) {
			fixture := newEnablementFixture(t, 3)
			if stage != "predecessor" {
				fixture.attempt(t, true, stage == "committed")
			}
			observed := recordedObservation(t, fixture.recovery)
			originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, observed.Root)
			release, err := lockNativeEnablement(fixture.record.UserRoot)
			if err != nil {
				t.Fatal(err)
			}
			cold := coldEnablementRecovery(fixture)
			if err := cold.recoverObservedRecord(t.Context(), observed); err == nil {
				t.Fatal("recovery passed a busy original native lock")
			}
			assertPrototypeFilesUnchanged(t, originals)
			if err := release(); err != nil {
				t.Fatal(err)
			}
			if err := coldEnablementRecovery(fixture).recoverObservedRecord(t.Context(), observed); err != nil {
				t.Fatal("same exact boundary did not recover after lock release", err)
			}
		})
	}
}

func TestRecordedRecoveryV3CancellationRetainsObservedRecord(t *testing.T) {
	fixture := newEnablementFixture(t, 3)
	fixture.attempt(t, true, false)
	observed := recordedObservation(t, fixture.recovery)
	originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, observed.Root)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := coldEnablementRecovery(fixture).recoverObservedRecord(ctx, observed); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled recovery lost the original deadline", err)
	}
	assertPrototypeFilesUnchanged(t, originals)
}

func TestRecordedRecoveryV3RefusesBareAttempt(t *testing.T) {
	fixture := newEnablementFixture(t, 3)
	fixture.attempt(t, true, false)
	record, err := fixture.recovery.readRecord(fixture.record.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.recovery.removeRecord(record.OperationID, record.rawDigest); err != nil {
		t.Fatal(err)
	}
	observed := recordedObservation(t, fixture.recovery)
	if observed.Root != "" {
		t.Fatal("bare fixture unexpectedly has a record root")
	}
	originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path)
	if err := coldEnablementRecovery(fixture).recoverObservedRecord(t.Context(), observed); !errors.Is(err, errEnablementUnqualified) {
		t.Fatal("bare attempt was inferred safe from a future caller", err)
	}
	assertPrototypeFilesUnchanged(t, originals)
}

func TestRecordedRecoveryV3StateFailureLeavesRetryablePredecessor(t *testing.T) {
	fixture := newEnablementFixture(t, 3)
	fixture.attempt(t, true, false)
	observed := recordedObservation(t, fixture.recovery)
	fixture.state.fail = true
	if err := fixture.recovery.recoverObservedRecord(t.Context(), observed); err == nil {
		t.Fatal("state failure became successful recovery")
	}
	body, err := os.ReadFile(fixture.config)
	if err != nil || !bytes.Equal(body, fixture.original) {
		t.Fatal("owned rollback did not retain its visible predecessor", err)
	}
	if _, err := os.Stat(observed.Root); err != nil {
		t.Fatal("failed state settlement discarded the journal", err)
	}
	fixture.state.fail = false
	observed = recordedObservation(t, fixture.recovery)
	if err := coldEnablementRecovery(fixture).recoverObservedRecord(t.Context(), observed); err != nil {
		t.Fatal("cold retry could not settle the exact predecessor", err)
	}
	assertRecordedSettlement(t, fixture, fixture.original, fixture.record.BeforeStateDigest)
}

func TestRecordedRecoveryV3UnlinkFailureDoesNotInventRetention(t *testing.T) {
	fixture := newEnablementFixture(t, 3)
	fixture.attempt(t, true, false)
	observed := recordedObservation(t, fixture.recovery)
	injected := errors.New("inert post-unlink failure")
	cold := coldEnablementRecovery(fixture)
	cold.journalPoint = func(point string) error {
		if point == "record_removed" {
			return injected
		}
		return nil
	}
	if err := cold.recoverObservedRecord(t.Context(), observed); !errors.Is(err, injected) {
		t.Fatal("late cleanup failure was reported as success", err)
	}
	assertRecordedSettlement(t, fixture, fixture.original, fixture.record.BeforeStateDigest)
	if _, err := os.Stat(filepath.Join(fixture.recovery.OperationsDir, enablementStagePrefix+fixture.record.OperationID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed cleanup recreated a stage", err)
	}
}
