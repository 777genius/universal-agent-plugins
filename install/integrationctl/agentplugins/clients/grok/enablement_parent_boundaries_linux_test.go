//go:build linux

package grok

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/directoryidentity"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func parentJournalFixture(t *testing.T, action string) enablementFixture {
	t.Helper()
	if action == "append" || action == "clear" {
		fixture, desired := removalEnablementFixture(t)
		applyRemovalFixture(t, fixture)
		digest := enablementStateDigest(desired)
		if action == "append" {
			return fixture
		}
		if err := fixture.recovery.appendDecision(fixture.record, digest); err != nil {
			t.Fatal(err)
		}
		var err error
		fixture.record, err = fixture.recovery.readRecord(fixture.record.OperationID)
		if err != nil {
			t.Fatal(err)
		}
		return fixture
	}
	fixture := newEnablementFixture(t)
	record, err := fixture.recovery.readRecord(fixture.record.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if action == "remove" {
		fixture.record = record
		return fixture
	}
	journal, _ := fixture.recovery.recordRoot(record.OperationID)
	if err := os.Remove(journal); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestEnablementColdParentGuardsJournalMutationBoundaries(t *testing.T) {
	for _, test := range []struct{ action, point string }{
		{"publish", "stage_created"}, {"publish", "record_written"}, {"publish", "record_synced"}, {"publish", "record_published"},
		{"remove", "before_record_remove"}, {"append", "before_decision_write"}, {"clear", "record_read"},
	} {
		t.Run(test.action+"/"+test.point, func(t *testing.T) {
			fixture := parentJournalFixture(t, test.action)
			var originals map[string]prototypeFileSnapshot
			fixture.recovery.journalPoint = func(point string) error {
				if point != test.point || originals != nil {
					return nil
				}
				paths := []string{fixture.config, fixture.state.Path}
				for _, prefix := range []string{enablementRecordPrefix, enablementStagePrefix} {
					path := filepath.Join(fixture.recovery.OperationsDir, prefix+fixture.record.OperationID)
					if _, err := os.Stat(path); err == nil {
						paths = append(paths, path)
					} else if !os.IsNotExist(err) {
						t.Fatal(err)
					}
				}
				originals = capturePrototypeFiles(t, paths...)
				replaceEnablementParent(t, fixture)
				return nil
			}
			if err := mutateParentJournal(fixture, test.action); !errors.Is(err, directoryidentity.ErrChanged) || originals == nil {
				t.Fatal("journal mutation admitted replaced parent or missed its boundary", err)
			}
			assertPrototypeFilesUnchanged(t, originals)
		})
	}
}

func mutateParentJournal(fixture enablementFixture, action string) error {
	switch action {
	case "append":
		return fixture.recovery.appendDecision(fixture.record, enablementDigest([]byte("bounded removal target")))
	case "clear":
		_, err := fixture.recovery.clearDecision(fixture.record)
		return err
	case "remove":
		return fixture.recovery.removeRecord(fixture.record.OperationID, fixture.record.rawDigest)
	default:
		return fixture.recovery.writeRecord(fixture.record)
	}
}

type parentReplacingInventory struct {
	*enablementNativeFixture
	replace func()
}

func (native parentReplacingInventory) RunDuplexWithPlannedShutdown(ctx context.Context, command legacyports.Command, exchange func(io.Writer, io.Reader) error) error {
	if err := native.enablementNativeFixture.RunDuplexWithPlannedShutdown(ctx, command, exchange); err != nil {
		return err
	}
	native.replace()
	return nil
}

func TestEnablementColdParentIsCapturedBeforeFirstPrototypeInventory(t *testing.T) {
	fixture := newLifecycleEnablementFixture(t)
	env, request, native := lifecycleEnablementInput(t, fixture)
	originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path)
	env.Runner = parentReplacingInventory{enablementNativeFixture: native, replace: func() { replaceEnablementParent(t, fixture) }}
	if _, err := enableProject(t.Context(), env, request, domain.ActivationOutcome{NativeEffect: domain.NativeEffectUnchanged}); !errors.Is(err, directoryidentity.ErrChanged) || native.calls != 1 {
		t.Fatal("prototype adopted the post-inventory replacement parent", err, native.calls)
	}
	assertPrototypeFilesUnchanged(t, originals)
	entries, err := os.ReadDir(fixture.recovery.OperationsDir)
	if err != nil || len(entries) != 0 {
		t.Fatal("replacement parent received new journal authority", err)
	}
}

func TestEnablementColdParentReadbackReplacementRetainsPendingDecision(t *testing.T) {
	fixture := newLifecycleEnablementFixture(t)
	env, request, native := lifecycleEnablementInput(t, fixture)
	var originals map[string]prototypeFileSnapshot
	native.afterSecond = func() {
		journal, _ := fixture.recovery.recordRoot(fixture.record.OperationID)
		originals = capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal)
		replaceEnablementParent(t, fixture)
	}
	outcome, err := enableProject(t.Context(), env, request, domain.ActivationOutcome{NativeEffect: domain.NativeEffectUnchanged})
	if !errors.Is(err, directoryidentity.ErrChanged) || native.calls != 2 || originals == nil || outcome.NativeEffect != domain.NativeEffectUncertain {
		t.Fatal("readback replacement received a settled ownership decision", err, native.calls, outcome.NativeEffect)
	}
	assertPrototypeFilesUnchanged(t, originals)
	if fixture.loaded(t).Installations[0].Clients[fixture.record.BindingID].NativeActivationAttempt != fixture.record.OperationID {
		t.Fatal("readback replacement cleared the pending attempt")
	}
}

func TestEnablementColdParentGuardsConfigApplyAndRollback(t *testing.T) {
	for _, action := range []string{"apply", "rollback"} {
		t.Run(action, func(t *testing.T) {
			fixture := newEnablementFixture(t)
			file, err := fixture.recovery.Kernel.BeginExactFile(fixture.config)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			}()
			if action == "rollback" {
				if err := fixture.record.applyConfig(file, fixture.next); err != nil {
					t.Fatal(err)
				}
			}
			originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path)
			replaceEnablementParent(t, fixture)
			if action == "apply" {
				err = fixture.record.applyConfig(file, fixture.next)
			} else {
				err = fixture.record.rollbackConfig(file)
			}
			if !errors.Is(err, directoryidentity.ErrChanged) {
				t.Fatal("native config mutation admitted replacement parent", err)
			}
			assertPrototypeFilesUnchanged(t, originals)
		})
	}
}
