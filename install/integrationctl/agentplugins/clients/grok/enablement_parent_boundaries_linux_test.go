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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
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

func (native parentReplacingInventory) RunReadonlyDuplexWithPlannedShutdown(ctx context.Context, command legacyports.Command, inputs legacyports.ReadonlyDuplexInputs, exchange func(io.Writer, io.Reader) error) error {
	if err := native.enablementNativeFixture.RunReadonlyDuplexWithPlannedShutdown(ctx, command, inputs, exchange); err != nil {
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

func TestEnablementReadbackRefusesLostParentBeforeConfigIO(t *testing.T) {
	for _, action := range []string{"enable", "remove"} {
		t.Run(action, func(t *testing.T) {
			fixture := newLifecycleEnablementFixture(t)
			env, request, native := lifecycleEnablementInput(t, fixture)
			if action == "remove" {
				if _, err := enableProject(t.Context(), env, request, domain.ActivationOutcome{NativeEffect: domain.NativeEffectUnchanged}); err != nil {
					t.Fatal("private prototype setup", err)
				}
				env, request, native = lifecycleEnablementInput(t, fixture)
			}
			recovery := env.NativeTransitions.(*EnablementRecovery)
			files := &parentRefusalFileIO{allow: true}
			recovery.Kernel = nativeconfig.NewWithFileIO(files)
			var originals map[string]prototypeFileSnapshot
			native.afterSecond = func() {
				journal, err := recovery.recordRoot(fixture.record.OperationID)
				if err != nil {
					t.Fatal(err)
				}
				originals = capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal,
					filepath.Join(fixture.record.UserRoot, ".config-init.lock"))
				replaceEnablementParent(t, fixture)
				files.allow = false
			}
			var uncertain bool
			var resultErr error
			if action == "enable" {
				outcome, err := enableProject(t.Context(), env, request, domain.ActivationOutcome{NativeEffect: domain.NativeEffectUnchanged})
				uncertain, resultErr = outcome.NativeEffect == domain.NativeEffectUncertain, err
			} else {
				state := fixture.loaded(t)
				binding := state.Installations[0].Clients[fixture.record.BindingID]
				identity := request.NativeAttempt
				identity.StateDigest = enablementStateDigest(state)
				removal := domain.DeactivationRequest{Client: request.Client, TargetContext: request.Plan.TargetContext,
					NativeRegistryRoot: binding.NativeProfileRoot, PhysicalArtifactID: binding.PhysicalArtifact, ManagedArtifactPath: binding.TargetLocator,
					DeclaredName: request.DeclaredName, NativeObjects: binding.NativeObjects, NativeAttempt: identity,
					BackendExecutable: request.BackendExecutable, Confirmed: true}
				outcome, err := removeEnabledProject(t.Context(), env, removal, domain.DeactivationOutcome{NativeEffect: domain.NativeEffectUnchanged})
				uncertain, resultErr = outcome.NativeEffect == domain.NativeEffectUncertain && !outcome.ExternalRemovalComplete, err
			}
			if !errors.Is(resultErr, directoryidentity.ErrChanged) || !uncertain || native.calls != 2 ||
				originals == nil || files.reads != 0 || files.writes != 0 || files.removes != 0 {
				t.Fatalf("lost parent reached config IO: error=%v uncertain=%t native=%d calls=%+v", resultErr, uncertain, native.calls, files)
			}
			assertPrototypeFilesUnchanged(t, originals)
		})
	}
}

func TestEnablementRepeatRefusesPostVerifierParentReplacement(t *testing.T) {
	fixture := newLifecycleEnablementFixture(t)
	env, request, _ := lifecycleEnablementInput(t, fixture)
	baseline := domain.ActivationOutcome{Activation: domain.ActivationPrepared, NativeEffect: domain.NativeEffectUnchanged,
		UserActions: []string{"Preserve existing guidance"}}
	first, err := enableProject(t.Context(), env, request, baseline)
	if err != nil || first.NativeEffect != domain.NativeEffectCommitted {
		t.Fatal("private prototype first commit", err)
	}
	env, request, native := lifecycleEnablementInput(t, fixture)
	recovery := env.NativeTransitions.(*EnablementRecovery)
	originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path,
		filepath.Join(fixture.record.UserRoot, ".config-init.lock"))
	replaced := false
	recovery.PackageVerifier = replacingParentVerifier{
		enablementTestPackage: enablementTestPackage{path: fixture.record.ArtifactPath, digest: fixture.record.PackageDigest},
		replace: func() {
			replaceEnablementParent(t, fixture)
			replaced = true
		},
	}
	outcome, err := enableProject(t.Context(), env, request, baseline)
	if !errors.Is(err, directoryidentity.ErrChanged) || !replaced || native.calls != 1 ||
		outcome.NativeEffect != domain.NativeEffectUnchanged || len(outcome.UserActions) != 1 || outcome.UserActions[0] != baseline.UserActions[0] {
		t.Fatalf("repeat adopted replacement parent or reported enabled success: %+v %v replaced=%t calls=%d", outcome, err, replaced, native.calls)
	}
	assertPrototypeFilesUnchanged(t, originals)
	entries, err := os.ReadDir(recovery.OperationsDir)
	if err != nil || len(entries) != 0 {
		t.Fatal("repeat changed the settled empty journal namespace", err)
	}
}

func TestEnablementRepeatStateChangeDoesNotReportEnabled(t *testing.T) {
	fixture := newLifecycleEnablementFixture(t)
	env, request, _ := lifecycleEnablementInput(t, fixture)
	baseline := domain.ActivationOutcome{Activation: domain.ActivationPrepared, NativeEffect: domain.NativeEffectUnchanged,
		UserActions: []string{"Preserve existing guidance"}}
	first, err := enableProject(t.Context(), env, request, baseline)
	if err != nil || first.NativeEffect != domain.NativeEffectCommitted {
		t.Fatal("private prototype first commit", err)
	}
	env, request, native := lifecycleEnablementInput(t, fixture)
	recovery := env.NativeTransitions.(*EnablementRecovery)
	expected := fixture.loaded(t)
	originals := capturePrototypeFiles(t, fixture.config, filepath.Join(fixture.record.UserRoot, ".config-init.lock"))
	var changedState map[string]prototypeFileSnapshot
	recovery.PackageVerifier = replacingParentVerifier{
		enablementTestPackage: enablementTestPackage{path: fixture.record.ArtifactPath, digest: fixture.record.PackageDigest},
		replace: func() {
			binding := expected.Installations[0].Clients[fixture.record.BindingID]
			binding.NativeActivationAttempt = ""
			if err := fixture.state.Save(enablementState(expected, 0, binding)); err != nil {
				t.Fatal(err)
			}
			changedState = capturePrototypeFiles(t, fixture.state.Path)
		},
	}
	outcome, err := enableProject(t.Context(), env, request, baseline)
	if err == nil || err.Error() != "grok native transition complete state changed before its effect" ||
		changedState == nil || native.calls != 1 || outcome.NativeEffect != domain.NativeEffectUnchanged ||
		len(outcome.UserActions) != 1 || outcome.UserActions[0] != baseline.UserActions[0] {
		t.Fatalf("refused changed state reported enabled success: %+v %v changed=%t calls=%d", outcome, err, changedState != nil, native.calls)
	}
	if enablementStateDigest(fixture.loaded(t)) == enablementStateDigest(expected) {
		t.Fatal("fixture did not change the frozen state")
	}
	if err := fixture.record.revalidateParent(); err != nil {
		t.Fatal("fixture changed the original parent", err)
	}
	assertPrototypeFilesUnchanged(t, originals)
	assertPrototypeFilesUnchanged(t, changedState)
	entries, err := os.ReadDir(recovery.OperationsDir)
	if err != nil || len(entries) != 0 {
		t.Fatal("refused repeat changed the settled empty journal namespace", err)
	}
}

func TestEnablementRemovalDecisionRefusesLostParentBeforeConfigIO(t *testing.T) {
	fixture, desired := removalEnablementFixture(t)
	applyRemovalFixture(t, fixture)
	before := fixture.loaded(t)
	beforeDigest, desiredDigest := enablementStateDigest(before), enablementStateDigest(desired)
	journal, err := fixture.recovery.recordRoot(fixture.record.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal,
		filepath.Join(fixture.record.UserRoot, ".config-init.lock"))
	replaceEnablementParent(t, fixture)
	files := &parentRefusalFileIO{}
	fixture.recovery.Kernel = nativeconfig.NewWithFileIO(files)
	handled, err := fixture.recovery.PrepareNativeRemovalDecision(t.Context(), fixture.record.InstallationID, fixture.record.BindingID, before, desired)
	if !handled || !errors.Is(err, directoryidentity.ErrChanged) || files.reads != 0 || files.writes != 0 || files.removes != 0 {
		t.Fatalf("removal decision reached config IO after parent replacement: handled=%t error=%v calls=%+v", handled, err, files)
	}
	if enablementStateDigest(before) != beforeDigest || enablementStateDigest(desired) != desiredDigest {
		t.Fatal("refused removal decision changed its state arguments")
	}
	assertPrototypeFilesUnchanged(t, originals)
}
