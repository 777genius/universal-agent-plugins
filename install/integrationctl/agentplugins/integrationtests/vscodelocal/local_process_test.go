package vscodelocal_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Regression: exit91 happens without a real native effect, or public recovery
// adopts a selector/resends registration. Existing direct reconciler controls
// could not prove the actual child effect or Engine's locked acknowledgement.
func TestLocalActualProcessCrashRetainsReadyRecoveryFixture(t *testing.T) {
	for _, direction := range []domain.NativeIntentDirection{domain.NativeIntentRegister, domain.NativeIntentRemove} {
		t.Run(string(direction), func(t *testing.T) {
			f := freshLocal(t, false)
			id := ""
			if direction == domain.NativeIntentRemove {
				installed := applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, ""))
				id = installed.InstallationID
				facts, _ := installed.Binding.SelectedDelivery.LocalFacts()
				disableLocal(t, f, facts.Registration.Selector)
				applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, id))
				assertLocalObservation(t, f, false)
			}
			child := exec.CommandContext(t.Context(), f.runtime, "--TEST-local-crash")
			child.Env = []string{"HOME=" + f.root, "USERPROFILE=" + f.root, "TMPDIR=" + f.root, "AN_LOCAL_TEST_ROOT=" + f.root, "AN_LOCAL_DIRECTION=" + string(direction), "AN_LOCAL_ID=" + id, "PATH=/usr/bin:/bin"}
			output, err := runLocalProcess(t, child)
			var exited *exec.ExitError
			if !errors.As(err, &exited) || exited.ExitCode() != 91 {
				t.Fatalf("actual child crash: %v / %s", err, output)
			}
			var witness localCrashWitness
			must(t, json.Unmarshal(output, &witness))
			binding := onlyLocalBinding(t, loadLocalState(t, f.state))
			intent := binding.PendingNativeIntent
			if intent == nil {
				t.Fatal("actual effect lost pending intent")
			}
			must(t, intent.Validate(binding))
			facts, _ := intent.Delivery.LocalFacts()
			if witness.Attempt != intent.AttemptID || witness.Direction != direction || witness.Selector != facts.Registration.Selector || witness.StateDigest != testDigest(readLocal(t, filepath.Join(f.state, "state-v2.json"))) || witness.ProfileDigest != testDigest(readLocal(t, f.settings)) {
				t.Fatal("parent could not independently match child effect/intent witness")
			}
			if direction == domain.NativeIntentRegister {
				if binding.LocalEntryObservation != nil || binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) {
					t.Fatal("unacknowledged first registration invented ownership")
				}
				if !strings.Contains(string(readLocal(t, f.settings)), quoteLocal(facts.Registration.Selector)+":true") {
					t.Fatal("child did not register actual selector")
				}
				disableLocal(t, f, facts.Registration.Selector)
			} else if strings.Contains(string(readLocal(t, f.settings)), quoteLocal(facts.Registration.Selector)+":") {
				t.Fatal("child did not remove selector")
			}
			late := strings.Replace(string(readLocal(t, f.settings)), `"foreign":`, `"TEST-recovery-late":false,"foreign":`, 1)
			writeLocal(t, f.settings, []byte(late), 0600)
			assertForeign(t, readLocal(t, f.settings))
			observer := &lockedLocalRecovery{LocalAdapter: f.adapter, t: t, state: f.state}
			f.registry, err = clients.NewRegistry(observer)
			must(t, err)
			engine := f.engine(t, true)
			before := snapshotLocalFiles(t, f.state, f.settings, filepath.Dir(f.settings))
			view, err := engine.Inspect(t.Context())
			must(t, err)
			assertLocalFiles(t, before)
			if !view.Recovery.Required || len(view.Recovery.NativeIntents) != 1 || view.Recovery.NativeIntents[0].Intent.AttemptID != intent.AttemptID {
				t.Fatal("public Inspect lost exact pending attempt")
			}
			profile := snapshotLocalFiles(t, f.settings)
			recovered, err := engine.Recover(t.Context(), view)
			must(t, err)
			if recovered.Outcome != installer.OutcomeCompleted || len(recovered.Recovery.Resolved) != 1 || len(recovered.Recovery.Remaining) != 0 || len(recovered.Recovery.Unknown) != 0 || observer.calls != 1 || observer.effects != 0 {
				t.Fatalf("public acknowledgement differs: %+v calls=%d resends=%d", recovered, observer.calls, observer.effects)
			}
			assertLocalFiles(t, profile)
			b := onlyLocalBinding(t, loadLocalState(t, f.state))
			if b.PendingNativeIntent != nil || b.NativeActivationAttempt != "" {
				t.Fatal("durable acknowledgement left intent")
			}
			if direction == domain.NativeIntentRegister {
				assertLocalObservation(t, f, false)
			} else if b.LocalEntryObservation != nil || b.SelectedDelivery.OwnsProfileEntry(b.NativeObjects) {
				t.Fatal("certain removal retained entry authority")
			}
			after := snapshotLocalFiles(t, filepath.Join(f.state, "state-v2.json"), f.settings, filepath.Dir(f.settings))
			current, err := engine.Inspect(t.Context())
			must(t, err)
			again, err := engine.Recover(t.Context(), current)
			must(t, err)
			if again.Outcome != installer.OutcomeUnchanged || observer.calls != 1 {
				t.Fatal("second Recover resent or mutated")
			}
			assertLocalFiles(t, after)
			t.Logf("actual child witness=%s; public Recover acknowledged %s under existing lock; zero registration resends", output, direction)
		})
	}
}

type localCrashWitness struct {
	Attempt, Selector, StateDigest, ProfileDigest string
	Direction                                     domain.NativeIntentDirection
}

// TestMain dispatch keeps the child off m.Run, so ordinary runs add no skips.
func TestLocalCrashChild(t *testing.T) {
	root := os.Getenv("AN_LOCAL_TEST_ROOT")
	if root == "" {
		return
	} // Actual child dispatch is qualified by the parent test.
	f := &localFixture{root: root, pkg: filepath.Join(root, "package"), settings: filepath.Join(root, "selected-profile", "settings.json"), state: filepath.Join(root, "state"), runtime: filepath.Join(root, "TEST runtime ' Ω $(touch sentinel)")}
	cfg := vscode.LocalConfig{ProfileSettingsPath: f.settings, QualifiedTuple: vscode.SourceQualifiedTESTTuple("linux"), TargetShell: linuxTarget(), NativeStop: true, HookSpecs: localSpecs(f.runtime), DeclaredHookDigest: testDigest(readLocal(t, filepath.Join(f.pkg, hookPath())))}
	a, err := vscode.NewLocal(cfg)
	must(t, err)
	wrapped := &crashLocal{LocalAdapter: a, t: t, state: f.state}
	f.registry, err = clients.NewRegistry(wrapped)
	must(t, err)
	op := installer.OpInstall
	if os.Getenv("AN_LOCAL_DIRECTION") == string(domain.NativeIntentRemove) {
		op = installer.OpRemove
	}
	applyLocal(t, f.engine(t, true), f.request(op, os.Getenv("AN_LOCAL_ID")))
	t.Fatal("genuine native effect did not exit")
}

type crashLocal struct {
	*vscode.LocalAdapter
	t     *testing.T
	state string
}

func (a *crashLocal) pending(direction domain.NativeIntentDirection) domain.ClientBinding {
	b := onlyLocalBinding(a.t, loadLocalState(a.t, a.state))
	if b.PendingNativeIntent == nil || b.PendingNativeIntent.Direction != direction {
		a.t.Fatal("effect before matching durable intent")
	}
	must(a.t, b.PendingNativeIntent.Validate(b))
	return b
}
func (a *crashLocal) die(b domain.ClientBinding) {
	after := onlyLocalBinding(a.t, loadLocalState(a.t, a.state))
	if !reflect.DeepEqual(b, after) {
		a.t.Fatal("child acknowledged before witness")
	}
	f, _ := b.SelectedDelivery.LocalFacts()
	body := readLocal(a.t, f.SettingsPath)
	register := b.PendingNativeIntent.Direction == domain.NativeIntentRegister
	if register != strings.Contains(string(body), quoteLocal(f.Registration.Selector)+":true") {
		a.t.Fatal("actual effect not witnessed")
	}
	if !register && strings.Contains(string(body), quoteLocal(f.Registration.Selector)+":") {
		a.t.Fatal("owned removal left selector")
	}
	assertForeign(a.t, body)
	w := localCrashWitness{Attempt: b.PendingNativeIntent.AttemptID, Direction: b.PendingNativeIntent.Direction, Selector: f.Registration.Selector, StateDigest: testDigest(readLocal(a.t, filepath.Join(a.state, "state-v2.json"))), ProfileDigest: testDigest(body)}
	must(a.t, json.NewEncoder(os.Stdout).Encode(w))
	os.Exit(91)
}
func (a *crashLocal) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if req.VerifyOnly {
		return a.LocalAdapter.Activate(ctx, env, req)
	}
	b := a.pending(domain.NativeIntentRegister)
	if !b.PendingNativeIntent.LocalEntryObservation.Equal(req.Plan.LocalEntryObservation) {
		a.t.Fatal("register predecessor differs")
	}
	out, err := a.LocalAdapter.Activate(ctx, env, req)
	if err == nil && out.NativeEffect == domain.NativeEffectCommitted && out.LocalEntryObservation != nil {
		a.die(b)
	}
	return out, err
}
func (a *crashLocal) Deactivate(ctx context.Context, env clients.Env, req domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	b := a.pending(domain.NativeIntentRemove)
	if req.LocalEntryObservation == nil || !b.LocalEntryObservation.Equal(req.LocalEntryObservation) {
		a.t.Fatal("remove lost frozen observation")
	}
	out, err := a.LocalAdapter.Deactivate(ctx, env, req)
	if err == nil && out.ExternalRemovalComplete {
		a.die(b)
	}
	return out, err
}

type lockedLocalRecovery struct {
	*vscode.LocalAdapter
	t              *testing.T
	state          string
	calls, effects int
}

func (a *lockedLocalRecovery) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if !req.VerifyOnly {
		a.effects++
		a.t.Error("Recover resent registration")
	}
	return a.LocalAdapter.Activate(ctx, env, req)
}
func (a *lockedLocalRecovery) ReconcileNativeIntent(ctx context.Context, intent domain.PendingNativeIntent) (domain.ActivationOutcome, error) {
	a.calls++
	// Probe the existing lock: no second lock is acquired or wraps recovery.
	release, err := (processlock.Lock{Path: filepath.Join(a.state, "mutation.lock")}).Acquire(ctx)
	if !errors.Is(err, processlock.ErrActive) {
		if release != nil {
			must(a.t, release())
		}
		a.t.Fatal("public Recover did not retain existing lock")
	}
	b := onlyLocalBinding(a.t, loadLocalState(a.t, a.state))
	must(a.t, intent.Validate(b))
	return a.LocalAdapter.ReconcileNativeIntent(ctx, intent)
}

func loadLocalState(t *testing.T, root string) domain.StateFileV2 {
	t.Helper()
	state, err := (statev2.Store{Path: filepath.Join(root, "state-v2.json")}).Load()
	must(t, err)
	return state
}

func onlyLocalBinding(t *testing.T, state domain.StateFileV2) domain.ClientBinding {
	t.Helper()
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
		t.Fatal("unexpected TEST binding scope")
	}
	for _, binding := range state.Installations[0].Clients {
		return binding
	}
	t.Fatal("missing TEST binding")
	return domain.ClientBinding{}
}

// Red: reverse reconciliation clears foreign entries, enables false, uses the
// current constructor or repeats an effect. Strongest available boundary is
// the actual selected reconciler; public reverse handoff is separately owned.
func TestLocalRecordedReverseReconciliationPreservesLateForeign(t *testing.T) {
	f := freshLocal(t, false)
	installed := applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, ""))
	before := readLocal(t, f.settings)
	facts, _ := installed.Binding.SelectedDelivery.LocalFacts()
	edited := strings.Replace(string(before), facts.Registration.Selector+`":true`, facts.Registration.Selector+`":false`, 1)
	edited = strings.Replace(edited, `"foreign":`, `"TEST-late-foreign":true,"foreign":`, 1)
	writeLocal(t, f.settings, []byte(edited), 0600)
	other := freshLocal(t, false)
	otherBefore := readLocal(t, other.settings)
	intent := domain.PendingNativeIntent{AttemptID: "TEST-owned-reverse", Direction: domain.NativeIntentRemove, RemoveOwnedEntry: true, Delivery: installed.Binding.SelectedDelivery, LocalEntryObservation: onlyLocalBinding(t, loadLocalState(t, f.state)).LocalEntryObservation}
	_, err := other.adapter.ReconcileNativeIntent(t.Context(), intent)
	must(t, err)
	after := readLocal(t, f.settings)
	if strings.Contains(string(after), facts.Registration.Selector) || !strings.Contains(string(after), `"TEST-late-foreign":true`) {
		t.Fatal("reverse reconciliation changed foreign scope")
	}
	assertForeign(t, after)
	_, err = other.adapter.ReconcileNativeIntent(t.Context(), intent)
	must(t, err)
	if string(after) != string(readLocal(t, f.settings)) || string(otherBefore) != string(readLocal(t, other.settings)) {
		t.Fatal("reverse retry changed either profile")
	}
}

// Red: recorded recovery admits an unsupported or foreign executor despite
// constructor refusal. Boundary: actual adapter and real selected native effect;
// no current constructor/environment may silently reinterpret recorded facts.
func TestLocalRecordedExecutorAdmissionFailsClosed(t *testing.T) {
	for _, goos := range []string{"darwin", "windows"} {
		t.Run(goos, func(t *testing.T) {
			f := freshLocal(t, false)
			installed := applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, ""))
			facts, _ := installed.Binding.SelectedDelivery.LocalFacts()
			facts.Tuple = vscode.SourceQualifiedTESTTuple(goos)
			selected, err := domain.NewLocalDelivery(facts)
			must(t, err)
			before := readLocal(t, f.settings)
			intent := domain.PendingNativeIntent{AttemptID: "TEST-rejected-executor", Direction: domain.NativeIntentRemove, RemoveOwnedEntry: true, Delivery: selected}
			_, err = f.adapter.ReconcileNativeIntent(t.Context(), intent)
			if err == nil || string(before) != string(readLocal(t, f.settings)) {
				t.Fatal("recorded unsupported executor changed native registration")
			}
		})
	}
}
