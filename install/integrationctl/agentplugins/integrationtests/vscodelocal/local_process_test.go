package vscodelocal_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Red: native profile bytes precede committed pending intent; child death loses
// the exact selected native decision. Wrapper DELEGATES unchanged genuine
// NewLocal.Activate, then exits 91. No synthetic lifecycle or profile parser.
func TestLocalActualProcessCrashRetainsReadyRecoveryFixture(t *testing.T) {
	f := freshLocal(t, false)
	child := exec.CommandContext(t.Context(), f.runtime, "-test.run=^TestLocalCrashChild$", "-test.v")
	child.Env = []string{"HOME=" + f.root, "USERPROFILE=" + f.root, "TMPDIR=" + f.root, "AN_LOCAL_TEST_ROOT=" + f.root, "PATH=/usr/bin:/bin"}
	output, err := runLocalProcess(t, child)
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 91 {
		t.Fatalf("actual child crash: %v / %s", err, output)
	}
	state := loadLocalState(t, f.state)
	binding := onlyLocalBinding(t, state)
	intent := binding.PendingNativeIntent
	if intent == nil || intent.AttemptID != binding.NativeActivationAttempt || intent.Direction != domain.NativeIntentRegister {
		t.Fatal("actual native effect lacks durable matching intent")
	}
	facts, _ := intent.Delivery.LocalFacts()
	if facts.SettingsPath != f.settings || facts.Registration.Selector != binding.TargetLocator || facts.ProjectionDigest == "" {
		t.Fatal("pending physical projection authority differs")
	}
	assertForeign(t, readLocal(t, f.settings))
	// Inspect is the genuine public base API; it currently omits native attempts.
	view, err := f.engine(t, true).Inspect(t.Context())
	must(t, err)
	if view.Recovery.Required {
		t.Log("public native recovery now observable; compose approved facade reconciler control")
	} else {
		t.Log("FOUNDATION GAP: Inspect omitted actual pending Local native attempt; Recover has no selected reconciler input")
	}
	// Existing adapter contract, explicitly NOT public facade recovery. It reads
	// the pending entry under recorded authority despite another constructor.
	other := freshLocal(t, false)
	before := readLocal(t, f.settings)
	reconciled, err := other.adapter.ReconcileNativeIntent(t.Context(), *intent)
	must(t, err)
	if reconciled.NativeEffect != domain.NativeEffectUnchanged || len(reconciled.NativeObjects) != 1 || string(before) != string(readLocal(t, f.settings)) {
		t.Fatal("recorded registration reconciliation redirected or wrote bytes")
	}
	encoded, err := json.Marshal(intent)
	must(t, err)
	t.Logf("actual exit91 pending intent %s; state_sha256=%s profile_sha256=%s", encoded, testDigest(readLocal(t, filepath.Join(f.state, "state-v2.json"))), testDigest(before))
}

func TestLocalCrashChild(t *testing.T) {
	root := os.Getenv("AN_LOCAL_TEST_ROOT")
	if root == "" {
		t.Skip("owned child only")
	}
	f := &localFixture{root: root, pkg: filepath.Join(root, "package"), settings: filepath.Join(root, "selected-profile", "settings.json"), state: filepath.Join(root, "state"), runtime: filepath.Join(root, "TEST runtime ' Ω $(touch sentinel)")}
	cfg := vscode.LocalConfig{ProfileSettingsPath: f.settings, QualifiedTuple: vscode.SourceQualifiedTESTTuple("linux"), TargetShell: linuxTarget(), NativeStop: true, HookSpecs: localSpecs(f.runtime), DeclaredHookDigest: testDigest(readLocal(t, filepath.Join(f.pkg, hookPath())))}
	a, err := vscode.NewLocal(cfg)
	must(t, err)
	wrapped := &crashLocal{LocalAdapter: a, t: t, state: f.state}
	f.registry, err = clients.NewRegistry(wrapped)
	must(t, err)
	applyLocal(t, f.engine(t, true), f.request(installer.OpInstall, ""))
	t.Fatal("genuine native effect did not exit")
}

type crashLocal struct {
	*vscode.LocalAdapter
	t     *testing.T
	state string
}

func (a *crashLocal) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	binding := onlyLocalBinding(a.t, loadLocalState(a.t, a.state))
	if binding.PendingNativeIntent == nil || binding.PendingNativeIntent.Direction != domain.NativeIntentRegister {
		a.t.Fatal("native effect before durable pending intent")
	}
	outcome, err := a.LocalAdapter.Activate(ctx, env, req)
	if err == nil && !req.VerifyOnly && outcome.NativeEffect == domain.NativeEffectCommitted {
		os.Exit(91)
	}
	return outcome, err
}
func loadLocalState(t *testing.T, root string) domain.StateFileV2 {
	t.Helper()
	var state domain.StateFileV2
	must(t, json.Unmarshal(readLocal(t, filepath.Join(root, "state-v2.json")), &state))
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
	intent := domain.PendingNativeIntent{AttemptID: "TEST-owned-reverse", Direction: domain.NativeIntentRemove, RemoveOwnedEntry: true, Delivery: installed.Binding.SelectedDelivery}
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
