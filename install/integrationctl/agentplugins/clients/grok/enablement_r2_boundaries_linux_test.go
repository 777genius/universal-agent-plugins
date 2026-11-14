//go:build linux

package grok

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type r2MutatingMetadataRunner struct {
	path  string
	calls int
}

type r2MutatingReadbackRunner struct {
	*enablementNativeFixture
	path string
}

func (runner *r2MutatingReadbackRunner) RunReadonlyDuplexWithPlannedShutdown(ctx context.Context, command legacyports.Command, inputs legacyports.ReadonlyDuplexInputs, exchange func(io.Writer, io.Reader) error) error {
	if runner.calls == 1 {
		runner.calls++
		if err := os.WriteFile(runner.path, []byte("native auxiliary postimage\n"), 0600); err != nil {
			return err
		}
		return errors.New("inert readback failed after native auxiliary mutation")
	}
	return runner.enablementNativeFixture.RunReadonlyDuplexWithPlannedShutdown(ctx, command, inputs, exchange)
}

func TestR2ReadbackRecoveryCannotDeclareAuxiliaryEffectRestored(t *testing.T) {
	fixture := newLifecycleEnablementFixture(t)
	env, request, native := lifecycleEnablementInput(t, fixture)
	path := filepath.Join(fixture.record.UserRoot, "observed-log")
	before := []byte("original native auxiliary bytes\n")
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	env.Runner = &r2MutatingReadbackRunner{enablementNativeFixture: native, path: path}
	outcome, err := enableProject(t.Context(), env, request, domain.ActivationOutcome{
		Activation: domain.ActivationPrepared, NativeEffect: domain.NativeEffectUnchanged,
	})
	if err == nil || native.calls != 2 || outcome.NativeEffect != domain.NativeEffectUncertain {
		t.Fatalf("causal readback fixture did not fail at second ACP: %+v %v calls=%d", outcome, err, native.calls)
	}
	journal, err := fixture.recovery.recordRoot(fixture.record.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal, path)
	recoveryErr := fixture.recovery.Recover(t.Context())
	if !errors.Is(recoveryErr, errEnablementUnqualified) || native.calls != 2 ||
		fixture.loaded(t).Installations[0].Clients[fixture.record.BindingID].NativeActivationAttempt == "" {
		t.Fatalf("incomplete auxiliary recovery was acknowledged: %v", recoveryErr)
	}
	assertPrototypeFilesUnchanged(t, originals)
}

func (*r2MutatingMetadataRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{}, errors.New("unexpected non-duplex command")
}

func (*r2MutatingMetadataRunner) DuplexCapability() error { return nil }

func (runner *r2MutatingMetadataRunner) RunReadonlyDuplexWithPlannedShutdown(context.Context, legacyports.Command, legacyports.ReadonlyDuplexInputs, func(io.Writer, io.Reader) error) error {
	runner.calls++
	if err := os.WriteFile(runner.path, []byte("observed auxiliary startup effect\n"), 0600); err != nil {
		return err
	}
	return errors.New("inert first ACP refusal after auxiliary write")
}

func TestR2FirstACPRefusalMustNotReportNativeUnchanged(t *testing.T) {
	fixture := newLifecycleEnablementFixture(t)
	env, request, _ := lifecycleEnablementInput(t, fixture)
	runner := &r2MutatingMetadataRunner{path: filepath.Join(fixture.record.UserRoot, "observed-log")}
	env.Runner = runner
	state := fixture.loaded(t)
	binding := state.Installations[0].Clients[fixture.record.BindingID]
	binding.NativeActivationAttempt = ""
	if err := fixture.state.Save(enablementState(state, 0, binding)); err != nil {
		t.Fatal(err)
	}
	originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path)
	if err := New().PreflightActivation(env, request); !errors.Is(err, errEnablementUnqualified) {
		t.Fatal("direct preflight admitted native startup", err)
	}
	outcome, err := New().Activate(t.Context(), env, request)
	if !errors.Is(err, errEnablementUnqualified) || runner.calls != 0 || outcome.NativeEffect != domain.NativeEffectUnchanged {
		t.Fatalf("direct adapter did not refuse before ACP: %+v %v calls=%d", outcome, err, runner.calls)
	}
	if _, err := os.Stat(runner.path); !os.IsNotExist(err) {
		t.Fatal("refused ACP changed native auxiliary state", err)
	}
	records, recordErr := fixture.recovery.pendingNames()
	if recordErr != nil || len(records) != 0 {
		t.Fatalf("unexpected pre-ACP recovery record: %v %v", records, recordErr)
	}
	if New().SupportsPluginEnablement(domain.ScopeProject) || New().SupportsPluginEnablement(domain.ScopeUser) {
		t.Fatal("unqualified enablement is advertised")
	}
	assertPrototypeFilesUnchanged(t, originals)
}

func TestR2ColdEnablementRecoveryRejectsReplacedUserParent(t *testing.T) {
	fixture := newEnablementFixture(t)
	fixture.attempt(t, true, false)
	parentBefore, err := os.Stat(fixture.record.UserRoot)
	if err != nil {
		t.Fatal(err)
	}
	configBefore, err := os.Stat(fixture.config)
	if err != nil {
		t.Fatal(err)
	}
	lockBefore, err := os.Stat(filepath.Join(fixture.record.UserRoot, ".config-init.lock"))
	if err != nil {
		t.Fatal(err)
	}
	stateBefore, err := os.ReadFile(fixture.state.Path)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := fixture.recovery.recordRoot(fixture.record.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	journalBefore, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := fixture.recovery.Pending()
	if err != nil || len(pending) != 1 {
		t.Fatal("original pending record unavailable", err)
	}
	originals := capturePrototypeFiles(t, fixture.config, fixture.state.Path, journal)
	retained := fixture.record.UserRoot + "-retained"
	if err := os.Rename(fixture.record.UserRoot, retained); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(fixture.record.UserRoot, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config.toml", ".config-init.lock"} {
		if err := os.Rename(filepath.Join(retained, name), filepath.Join(fixture.record.UserRoot, name)); err != nil {
			t.Fatal(err)
		}
	}
	parentAfter, err := os.Stat(fixture.record.UserRoot)
	if err != nil {
		t.Fatal(err)
	}
	configAfter, err := os.Stat(fixture.config)
	if err != nil || os.SameFile(parentBefore, parentAfter) || !os.SameFile(configBefore, configAfter) {
		t.Fatalf("fixture did not replace only the original User parent: %v", err)
	}
	lockAfter, err := os.Stat(filepath.Join(fixture.record.UserRoot, ".config-init.lock"))
	if err != nil || !os.SameFile(lockBefore, lockAfter) {
		t.Fatal("fixture did not preserve the original native lock leaf", err)
	}
	recoveryErr := fixture.recovery.Recover(t.Context())
	configBytes, configErr := os.ReadFile(fixture.config)
	stateBytes, stateErr := os.ReadFile(fixture.state.Path)
	journalBytes, journalErr := os.ReadFile(journal)
	if recoveryErr == nil || configErr != nil || stateErr != nil || journalErr != nil ||
		!bytes.Equal(configBytes, fixture.next) || !bytes.Equal(stateBytes, stateBefore) || !bytes.Equal(journalBytes, journalBefore) {
		t.Fatalf("replacement parent was not refused without effects: recovery=%v config_changed=%t state_changed=%t journal_error=%v",
			recoveryErr, !bytes.Equal(configBytes, fixture.next), !bytes.Equal(stateBytes, stateBefore), journalErr)
	}
	if err := fixture.recovery.ReconcileObserved(t.Context(), pending[0], fixture.recovery.State); !errors.Is(err, errEnablementUnqualified) {
		t.Fatal("observed recovery admitted replacement parent", err)
	}
	assertPrototypeFilesUnchanged(t, originals)
}

type prototypeFileSnapshot struct {
	body []byte
	info os.FileInfo
}

func capturePrototypeFiles(t *testing.T, paths ...string) map[string]prototypeFileSnapshot {
	t.Helper()
	result := make(map[string]prototypeFileSnapshot, len(paths))
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		result[path] = prototypeFileSnapshot{body: body, info: info}
	}
	return result
}

func assertPrototypeFilesUnchanged(t *testing.T, originals map[string]prototypeFileSnapshot) {
	t.Helper()
	for path, before := range originals {
		after := capturePrototypeFiles(t, path)[path]
		if !bytes.Equal(before.body, after.body) || !os.SameFile(before.info, after.info) || before.info.Mode() != after.info.Mode() || !before.info.ModTime().Equal(after.info.ModTime()) {
			t.Fatalf("prototype refusal changed original %s", path)
		}
	}
}
