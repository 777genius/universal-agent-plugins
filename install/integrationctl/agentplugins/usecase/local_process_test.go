package usecase_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"io/fs"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cursor"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/cursorhooks"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// Red: the ClientID-only native-attempt path leaves a Local profile write
// without pending selector/value authority; recovery then strands or adopts it.
// This is a TEST adapter, plain JSON fixture and real ExactFile/process death.
// It proves preparation machinery, not the native VS Code JSONC adapter.
func TestLocalProcessCrashReconcilesPersistedOwnedSelector(t *testing.T) {
	for _, direction := range []domain.NativeIntentDirection{domain.NativeIntentRegister, domain.NativeIntentRemove} {
		t.Run(string(direction), func(t *testing.T) {
			root := localProcessRoot(t)
			runLocalCrash(t, root, direction)
			store := statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := localOnlyBinding(t, before)
			if binding.PendingNativeIntent == nil || binding.NativeActivationAttempt == "" || binding.PendingNativeIntent.Direction != direction {
				t.Fatalf("profile effect lacked durable owned decision: %+v", binding)
			}
			facts, _ := binding.SelectedDelivery.LocalFacts()
			// Simulate a late native editor. Both unrelated settings and another location
			// must survive; no document preimage is available to the recovering process.
			doc := readLocalDocument(t, facts.SettingsPath)
			doc["late.foreign"] = "keep this late edit"
			locations := doc["chat.pluginLocations"].(map[string]any)
			locations[filepath.Join(root, "foreign-plugin")] = false
			writeLocalDocument(t, facts.SettingsPath, doc)
			late, _ := os.ReadFile(facts.SettingsPath)
			service := localRecoveryService(t, root)
			observer := &testLocalReconciler{kernel: nativeconfig.New()}
			// A stale observation must not reach even the selected native adapter.
			if err := service.RecoverNativeIntent(t.Context(), before.Installations[0].InstallationID, binding.ClientBindingID, "stale-attempt", observer); err == nil || observer.seen.AttemptID != "" {
				t.Fatal("stale recovery reached native effects")
			}
			// Owned-key drift retains the pending receipt and current foreign bytes.
			locations[facts.Registration.Selector] = false
			writeLocalDocument(t, facts.SettingsPath, doc)
			drift, _ := os.ReadFile(facts.SettingsPath)
			if err := service.RecoverNativeIntent(t.Context(), before.Installations[0].InstallationID, binding.ClientBindingID, binding.NativeActivationAttempt, observer); err == nil {
				t.Fatal("recovery adopted owned-key drift")
			}
			retained, _ := store.Load()
			retainedBytes, _ := os.ReadFile(facts.SettingsPath)
			if localOnlyBinding(t, retained).NativeActivationAttempt != binding.NativeActivationAttempt || string(retainedBytes) != string(drift) {
				t.Fatal("conflict erased pending authority or foreign bytes")
			}
			if direction == domain.NativeIntentRemove {
				delete(locations, facts.Registration.Selector)
			} else {
				locations[facts.Registration.Selector] = true
			}
			writeLocalDocument(t, facts.SettingsPath, doc)
			if err := service.RecoverNativeIntent(t.Context(), before.Installations[0].InstallationID, binding.ClientBindingID, binding.NativeActivationAttempt, observer); err != nil {
				t.Fatal(err)
			}
			after, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			recovered := localOnlyBinding(t, after)
			current, _ := os.ReadFile(facts.SettingsPath)
			if string(current) != string(late) || observer.seen.Delivery.Mode() != domain.DeliveryVSCodeLocalV1 || !reflect.DeepEqual(observer.seen.Delivery, binding.SelectedDelivery) {
				t.Fatal("recovery used constructor facts or overwrote a late foreign edit")
			}
			if recovered.PendingNativeIntent != nil || recovered.NativeActivationAttempt != "" {
				t.Fatalf("recovery left settled ownership pending: %+v", recovered)
			}
			if direction == domain.NativeIntentRegister && len(recovered.NativeObjects) != 2 {
				t.Fatal("registration recovery failed to acknowledge predeclared owned entry")
			}
			if direction == domain.NativeIntentRemove && len(recovered.NativeObjects) != 1 {
				t.Fatal("reverse recovery retained removed entry ownership")
			}
		})
	}
}

// The child is launched only by runLocalCrash with fresh TEST roots and minimal
// env. Its existing client Lifecycle seam supplies the crash point; production
// code receives no test hook, env switch, parser or process-kill path.
func TestLocalProcessChild(t *testing.T) {
	root := os.Getenv("U4A_TEST_ROOT")
	if root == "" {
		return
	}
	direction := domain.NativeIntentDirection(os.Getenv("U4A_TEST_DIRECTION"))
	adapter := &testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root, crash: direction}
	eng := localTestEngine(t, filepath.Join(root, "state"), adapter)
	req := installerRequest(root)
	handle, err := eng.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	result, err := eng.Apply(t.Context(), handle, confirmedDecision())
	if err != nil {
		t.Fatal(err)
	}
	if direction == domain.NativeIntentRemove {
		service := localRecoveryService(t, root)
		registry, err := clients.NewRegistry(adapter)
		if err != nil {
			t.Fatal(err)
		}
		kernel := nativeconfig.New()
		service.Activator = providers.Activator{Registry: registry, NativeConfig: &kernel}
		state, err := service.StateStore.Load()
		if err != nil {
			t.Fatal(err)
		}
		binding := localOnlyBinding(t, state)
		_, err = service.Remove(t.Context(), usecase.RemoveInput{Selector: result.InstallationID, Client: domain.DetectedClient{ClientID: domain.ClientVSCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(root, "profile")}, Scope: domain.ScopeUser, Confirmed: true, SelectedDelivery: binding.SelectedDelivery})
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("native effect did not terminate the child")
}

type testEffectLocalAdapter struct {
	testLocalAdapter
	root  string
	crash domain.NativeIntentDirection
}

func (a *testEffectLocalAdapter) Activate(_ context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if request.VerifyOnly {
		return domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationPackageValid}, nil
	}
	facts, _ := request.Plan.SelectedDelivery.LocalFacts()
	store := statev2.Store{Path: filepath.Join(a.root, "state", "state-v2.json")}
	state, err := store.Load()
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	binding := onlyLocalBinding(state)
	if binding.PendingNativeIntent == nil || binding.PendingNativeIntent.Direction != domain.NativeIntentRegister {
		return domain.ActivationOutcome{}, fmt.Errorf("profile write preceded durable register authority")
	}
	if err := patchLocalEntry(env.NativeConfig, facts, false, request.Plan.SelectedDelivery.OwnsProfileEntry(request.PreviousNativeObjects)); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if a.crash == domain.NativeIntentRegister {
		os.Exit(91)
	}
	return domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled, NativeEffect: domain.NativeEffectCommitted, NativeObjects: []domain.NativeObjectOwnership{facts.Registration.Ownership(facts.SettingsPath)}}, nil
}

func (a *testEffectLocalAdapter) Deactivate(_ context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	facts, _ := request.SelectedDelivery.LocalFacts()
	store := statev2.Store{Path: filepath.Join(a.root, "state", "state-v2.json")}
	state, err := store.Load()
	if err != nil {
		return domain.DeactivationOutcome{}, err
	}
	binding := onlyLocalBinding(state)
	if binding.PendingNativeIntent == nil || binding.PendingNativeIntent.Direction != domain.NativeIntentRemove {
		return domain.DeactivationOutcome{}, fmt.Errorf("profile write preceded durable reverse authority")
	}
	if !request.RemoveOwnedEntry || !binding.PendingNativeIntent.RemoveOwnedEntry {
		return domain.DeactivationOutcome{}, fmt.Errorf("reverse request lacks owned entry authority")
	}
	if err := patchLocalEntry(env.NativeConfig, facts, true, false); err != nil {
		return domain.DeactivationOutcome{}, err
	}
	if a.crash == domain.NativeIntentRemove {
		os.Exit(91)
	}
	return domain.DeactivationOutcome{ArtifactRemovalAllowed: true, ExternalRemovalComplete: true}, nil
}

type testLocalReconciler struct {
	kernel nativeconfig.Kernel
	seen   domain.PendingNativeIntent
}

func (r *testLocalReconciler) ReconcileNativeIntent(_ context.Context, intent domain.PendingNativeIntent) (domain.ActivationOutcome, error) {
	r.seen = intent
	facts, _ := intent.Delivery.LocalFacts()
	remove := intent.Direction == domain.NativeIntentRemove
	if remove && !intent.RemoveOwnedEntry {
		return domain.ActivationOutcome{NativeEffect: domain.NativeEffectUnchanged}, nil
	}
	if err := patchLocalEntry(r.kernel, facts, remove, true); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome := domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled, NativeEffect: domain.NativeEffectUnchanged}
	if !remove {
		outcome.NativeObjects = []domain.NativeObjectOwnership{facts.Registration.Ownership(facts.SettingsPath)}
	}
	return outcome, nil
}

// The tiny TEST document grammar intentionally accepts only fixture JSON. Native
// Local parsing/qualification remains the U3/U4b owner; nothing here ships.
func patchLocalEntry(kernel nativeconfig.Kernel, facts domain.LocalDeliveryFacts, remove, recovering bool) error {
	file, err := kernel.BeginExactFile(facts.SettingsPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	var doc map[string]any
	if err := json.Unmarshal(file.Original().Body, &doc); err != nil {
		return err
	}
	locations, ok := doc["chat.pluginLocations"].(map[string]any)
	if !ok {
		return fmt.Errorf("TEST locations are malformed")
	}
	value, exists := locations[facts.Registration.Selector]
	if exists && value != *facts.Registration.DesiredValue {
		return fmt.Errorf("owned entry drift; retain settings")
	}
	if recovering && !remove && !exists {
		return fmt.Errorf("pending desired entry is absent; retain uncertainty")
	}
	if recovering && (remove && !exists || !remove && exists) {
		return nil
	}
	if remove {
		delete(locations, facts.Registration.Selector)
	} else if exists {
		return fmt.Errorf("unowned identical entry cannot be adopted")
	} else {
		locations[facts.Registration.Selector] = *facts.Registration.DesiredValue
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return file.Apply(raw)
}

func runLocalCrash(t *testing.T, root string, direction domain.NativeIntentDirection) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestLocalProcessChild$", "-test.timeout=20s")
	cmd.Env = []string{"HOME=" + filepath.Join(root, "TEST-home"), "USERPROFILE=" + filepath.Join(root, "TEST-home"), "TMPDIR=" + root, "U4A_TEST_ROOT=" + root, "U4A_TEST_DIRECTION=" + string(direction)}
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 91 {
		t.Fatalf("child did not die after owned profile write: %v %s", err, output)
	}
}

func localRecoveryService(t *testing.T, root string) usecase.Service {
	t.Helper()
	stateRoot := filepath.Join(root, "state")
	store := statev2.Store{Path: filepath.Join(stateRoot, "state-v2.json")}
	registry, err := clients.NewRegistry(vscode.New())
	if err != nil {
		t.Fatal(err)
	}
	plan := planner.Planner{Registry: registry, Paths: pathpolicy.Policy{}, ManagedRoot: filepath.Join(stateRoot, "managed")}
	return usecase.Service{Planner: plan, Targets: plan, PluginData: providers.PluginDataManager{Base: filepath.Join(stateRoot, "plugin-data")}, StateStore: store, Lock: processlock.Lock{Path: filepath.Join(stateRoot, "mutation.lock")}, Stager: providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}}, Paths: pathpolicy.Policy{}, Kernel: transaction.Kernel{StateStore: store, Directory: dirswap.Manager{JournalDir: filepath.Join(stateRoot, "operations")}}}
}
func onlyLocalBinding(state domain.StateFileV2) domain.ClientBinding {
	for _, binding := range state.Installations[0].Clients {
		return binding
	}
	return domain.ClientBinding{}
}
func localOnlyBinding(t *testing.T, state domain.StateFileV2) domain.ClientBinding {
	t.Helper()
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
		t.Fatalf("unexpected TEST state: %+v", state)
	}
	return onlyLocalBinding(state)
}
func readLocalDocument(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}
func writeLocalDocument(t *testing.T, path string, doc map[string]any) {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func localProcessRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Cleanup(func() {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err == nil && d.IsDir() {
				return os.Chmod(path, 0700)
			}
			return err
		})
	})
	for _, dir := range []string{"profile", "package", "TEST-home"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "package", "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"local-test","version":"1.0.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "TEST-executable"), []byte("TEST fixture never executed"), 0700); err != nil {
		t.Fatal(err)
	}
	writeLocalDocument(t, filepath.Join(root, "profile", "settings.json"), map[string]any{"foreign.setting": "original", "chat.pluginLocations": map[string]any{}})
	return root
}
func installerRequest(root string) installer.Request {
	return installer.Request{Operation: installer.OpInstall, PackageRoot: filepath.Join(root, "package"), ClientID: "vscode", ClientExecutable: filepath.Join(root, "TEST-executable"), ClientConfigRoot: filepath.Join(root, "profile")}
}
func confirmedDecision() installer.Decision { return installer.Decision{Confirmed: true} }

// Breaking behavior: a matching foreign Stop command could be adopted as owned
// and receive a pending grant. The real pure planner must refuse before state.
func TestCursorSelectedUnownedCollisionBeforeGrant(t *testing.T) {
	root := localProcessRoot(t)
	a := &testCursorIntentAdapter{Adapter: cursor.New(), root: root, kernel: nativeconfig.New()}
	spec := a.spec()
	planned, err := cursorhooks.Plan(cursorhooks.Request{Operation: cursorhooks.Install, Specs: []cursorhooks.HookSpec{spec}, Shell: cursorhooks.LinuxUserShell32212, ExecutableVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "profile", "hooks.json")
	if err := os.WriteFile(path, planned.Desired, 0600); err != nil {
		t.Fatal(err)
	}
	e := lifecycleFacadeEngine(t, root, a, false)
	h, err := e.Prepare(t.Context(), cursorIntentRequest(root))
	if h != nil {
		defer func() {
			if err := h.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	if !errors.Is(err, cursorhooks.ErrConflict) {
		t.Fatalf("expected actual pure-planner collision before grant: %v", err)
	}
	state, err := (statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 0 || a.effects != 0 {
		t.Fatal("collision granted pending/native authority")
	}
	body, _ := os.ReadFile(path)
	if string(body) != string(planned.Desired) {
		t.Fatal("collision changed foreign bytes")
	}
}

// Breaking behavior: death after the first real register effect loses the
// actual planned receipt, or recovery recaptures/resends instead of reading it.
func TestCursorSelectedProcessRecovery(t *testing.T) {
	root := localProcessRoot(t)
	if err := os.WriteFile(filepath.Join(root, "profile", "hooks.json"), []byte(`{"version":1,"hooks":{"stop":[{"type":"command","command":"TEST-foreign-before"}]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), exe, "-test.run=^TestCursorSelectedProcessChild$", "-test.timeout=20s")
	cmd.Env = []string{"HOME=" + filepath.Join(root, "TEST-home"), "TMPDIR=" + root, "CURSOR_INTENT_TEST_ROOT=" + root}
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 91 {
		t.Fatalf("child must die after effect: %v %s", err, out)
	}
	store := statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := localOnlyBinding(t, state)
	intent := binding.PendingNativeIntent
	if intent == nil || intent.PreviousCursorObject != (domain.NativeObjectOwnership{}) || len(binding.NativeObjects) != 1 {
		t.Fatal("first effect lacked separate durable planned authority")
	}
	f, _ := intent.Delivery.CursorFacts()
	body, err := os.ReadFile(f.HooksPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	hooks := doc["hooks"].(map[string]any)
	stop := hooks["stop"].([]any)
	if len(stop) != 2 || stop[0].(map[string]any)["command"] != "TEST-foreign-before" {
		t.Fatal("first effect removed preexisting foreign entry")
	}
	hooks["stop"] = append(stop, map[string]any{"type": "command", "command": "TEST-foreign-after"})
	doc["late.foreign"] = "retained"
	writeLocalDocument(t, f.HooksPath, doc)
	late, _ := os.ReadFile(f.HooksPath)
	a := &testCursorIntentAdapter{Adapter: cursor.New(), root: root, kernel: nativeconfig.New()}
	e := lifecycleFacadeEngine(t, root, a, false)
	view, err := e.Inspect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Recovery.NativeIntents) != 1 {
		t.Fatal("public inspection lost Cursor pending intent")
	}
	// A genuine unresolved intent cannot be consumed by read-only Update.
	pendingBytes, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	pendingRequest := cursorIntentRequest(root)
	pendingRequest.Operation, pendingRequest.InstallationID = installer.OpUpdate, state.Installations[0].InstallationID
	pendingHandle, pendingErr := e.Prepare(t.Context(), pendingRequest)
	if pendingErr == nil {
		_, pendingErr = e.Apply(t.Context(), pendingHandle, confirmedDecision())
		if err := pendingHandle.Close(); err != nil {
			t.Fatal(err)
		}
	}
	stillPending, err := os.ReadFile(store.Path)
	if pendingErr == nil || err != nil || string(stillPending) != string(pendingBytes) || a.effects != 0 || a.readbacks != 0 {
		t.Fatal("pending read-only Update saved or dispatched activation")
	}
	result, err := e.Recover(t.Context(), view)
	if err != nil {
		t.Fatal(err)
	}
	final, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	recovered := localOnlyBinding(t, final)
	after, _ := os.ReadFile(f.HooksPath)
	if result.Outcome != installer.OutcomeCompleted || recovered.PendingNativeIntent != nil || recovered.NativeActivationAttempt != "" || a.effects != 0 || a.readbacks != 1 || string(after) != string(late) || !recovered.SelectedDelivery.OwnsProfileEntry(recovered.NativeObjects) {
		t.Fatal("recovery resent, changed foreign entries or failed to acknowledge")
	}
	for _, o := range recovered.NativeObjects {
		if o.Kind == "cursor_user_stop" && o.CursorReceipt != f.PlannedReceipt {
			t.Fatal("receipt recaptured from late foreign document")
		}
	}
	// Breaking behavior: an unchanged projection recaptures the original receipt
	// from a late foreign remainder, or resends an already owned effect.
	h, err := e.Prepare(t.Context(), cursorIntentRequest(root))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := h.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := e.Apply(t.Context(), h, confirmedDecision()); err != nil {
		t.Fatal(err)
	}
	if a.effects != 0 {
		t.Fatal("unchanged verified projection resent the native effect")
	}

	// Breaking behavior: an update grants a new basis but loses the separately
	// acknowledged predecessor. Use the same real Store and lifecycle again.
	if err := os.WriteFile(filepath.Join(root, "package", "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"local-test","version":"1.0.1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	req := cursorIntentRequest(root)
	req.Operation = installer.OpUpdate
	req.InstallationID = state.Installations[0].InstallationID
	update, err := e.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := update.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := e.Apply(t.Context(), update, confirmedDecision()); err != nil {
		t.Fatal(err)
	}
	if a.effects != 1 {
		t.Fatal("updated packet did not reach real lifecycle")
	}

}

// Regression: manual lifecycle resume used a fresh remainder packet against
// independently acknowledged ownership. This vendor fixture is unqualified;
// the physical positive and its full-state/namespace assertions remain required.
func TestCursorSelectedReadOnlySealing(t *testing.T) {
	for _, manual := range []bool{true, false} {
		t.Run(fmt.Sprintf("manual=%t", manual), func(t *testing.T) {
			root := localProcessRoot(t)
			a := &testCursorIntentAdapter{Adapter: cursor.New(), root: root, kernel: nativeconfig.New(), manual: manual}
			registry, err := clients.NewRegistry(a)
			if err != nil {
				t.Fatal(err)
			}
			// Coordination lock metadata is transient; keep its real lock outside
			// the TEST namespace whose complete bytes must remain unchanged.
			e, err := installer.New(installer.Config{StateRoot: filepath.Join(root, "state"), LockFile: filepath.Join(t.TempDir(), "TEST-mutation.lock"), Registry: registry, TrustedLocalPackages: true})
			if err != nil {
				t.Fatal(err)
			}
			req := cursorIntentRequest(root)
			h, err := e.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			installed, err := e.Apply(t.Context(), h, confirmedDecision())
			closeErr := h.Close()
			if err != nil || closeErr != nil || a.effects != 1 {
				t.Fatalf("real install: %+v %v %v", installed, err, closeErr)
			}
			store := statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := localOnlyBinding(t, before)
			if binding.PendingNativeIntent != nil || binding.NativeActivationAttempt != "" || binding.SelectedDelivery.ValidateCursorObjects(binding.NativeObjects) != nil {
				t.Fatal("no independent acknowledgement")
			}
			f, _ := binding.SelectedDelivery.CursorFacts()
			body, err := os.ReadFile(f.HooksPath)
			if err != nil {
				t.Fatal(err)
			}
			var doc map[string]any
			if err := json.Unmarshal(body, &doc); err != nil {
				t.Fatal(err)
			}
			doc["TEST-foreign-after"] = true
			writeLocalDocument(t, f.HooksPath, doc)
			snapshot := func() map[string]string {
				t.Helper()
				files := map[string]string{}
				if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !entry.IsDir() {
						body, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						files[path] = cursorRawDigest(body)
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				return files
			}
			files := snapshot()
			req.Operation, req.InstallationID = installer.OpUpdate, installed.InstallationID
			update, err := e.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			if reflect.DeepEqual(update.Plan().Client.SelectedDelivery, binding.SelectedDelivery) {
				t.Fatal("late remainder did not change planned attempt basis")
			}
			result, err := e.Apply(t.Context(), update, confirmedDecision())
			closeErr = update.Close()
			after, loadErr := store.Load()
			if err != nil || closeErr != nil || loadErr != nil || result.Mutated {
				t.Fatalf("read-only update: %+v %v %v %v", result, err, closeErr, loadErr)
			}
			if !reflect.DeepEqual(before, after) || !reflect.DeepEqual(files, snapshot()) {
				t.Fatal("read-only update changed full state/authority/receipt/namespace/hooks/package")
			}
			if a.effects != 1 || a.readbacks != 1 {
				t.Fatal("read-only update resent or skipped actual hook verification")
			}
			if result.Delivery == nil || !reflect.DeepEqual(result.Delivery.SelectedDelivery, binding.SelectedDelivery) || !reflect.DeepEqual(result.Binding.SelectedDelivery, binding.SelectedDelivery) || !reflect.DeepEqual(result.Client.SelectedDelivery, binding.SelectedDelivery) {
				t.Fatal("result exposed fresh attempt instead of acknowledged packet")
			}
			// Corrupt TEST bytes after genuine acknowledgement; no positive Store.Save.
			if !manual {
				return
			}
			stateBytes, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"selection", "profile", "projection", "package-receipt", "owned-entry", "receipt", "attempt", "package-bytes", "hook-bytes"} {
				t.Run(name, func(t *testing.T) {
					var corrupted domain.StateFileV2
					if err := json.Unmarshal(stateBytes, &corrupted); err != nil {
						t.Fatal(err)
					}
					b := localOnlyBinding(t, corrupted)
					facts, _ := b.SelectedDelivery.CursorFacts()
					switch name {
					case "selection":
						facts.QualificationID = "TEST-other-selection"
					case "profile":
						b.NativeProfileRoot = filepath.Join(root, "TEST-other-profile")
					case "projection":
						facts.ProjectionDigest = cursorRawDigest([]byte("TEST-other-projection"))
					case "package-receipt":
						b.NativeObjects[0].ManagedDigest = cursorRawDigest([]byte("TEST-other-package"))
					case "owned-entry":
						b.NativeObjects = b.NativeObjects[:1]
					case "receipt":
						b.NativeObjects[1].CursorReceipt.RemainderDigest = cursorRawDigest([]byte("TEST-other-receipt"))
					case "attempt":
						b.NativeActivationAttempt = "TEST-unresolved"
					}
					b.SelectedDelivery, err = domain.NewCursorDelivery(facts)
					if err != nil {
						t.Fatal(err)
					}
					corrupted.Installations[0].Clients[b.ClientBindingID] = b
					body, err := json.Marshal(corrupted)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(store.Path, body, 0600); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := os.WriteFile(store.Path, stateBytes, 0600); err != nil {
							t.Error(err)
						}
					})
					if name == "package-bytes" || name == "hook-bytes" {
						path := filepath.Join(b.TargetLocator, "plugin.json")
						if name == "hook-bytes" {
							path = facts.HooksPath
						}
						original, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, []byte(`{"TEST-unowned":true}`), 0600); err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() {
							if err := os.WriteFile(path, original, 0600); err != nil {
								t.Error(err)
							}
						})
					}
					beforeRefusal := snapshot()
					handle, refusal := e.Prepare(t.Context(), req)
					if refusal == nil {
						_, refusal = e.Apply(t.Context(), handle, confirmedDecision())
						if err := handle.Close(); err != nil {
							t.Fatal(err)
						}
					}
					if refusal == nil || a.effects != 1 || a.readbacks != 1 || !reflect.DeepEqual(beforeRefusal, snapshot()) {
						t.Fatalf("%s did not refuse before save/effect: %v", name, refusal)
					}
				})
			}

		})
	}
}

func TestCursorSelectedProcessChild(t *testing.T) {
	root := os.Getenv("CURSOR_INTENT_TEST_ROOT")
	if root == "" {
		return
	}
	a := &testCursorIntentAdapter{Adapter: cursor.New(), root: root, kernel: nativeconfig.New(), crash: true}
	e := lifecycleFacadeEngine(t, root, a, false)
	h, err := e.Prepare(t.Context(), cursorIntentRequest(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Apply(t.Context(), h, confirmedDecision()); err != nil {
		t.Fatal(err)
	}
	t.Fatal("child survived actual register effect")
}

// Injected vendor fixture uses only existing adapter/refiner/lifecycle seams.
// It executes cursorhooks and the real ExactFile kernel; no native app runs.
type testCursorIntentAdapter struct {
	*cursor.Adapter
	root               string
	kernel             nativeconfig.Kernel
	crash, manual      bool
	effects, readbacks int
}

func cursorIntentRequest(root string) installer.Request {
	req := installerRequest(root)
	req.ClientID = "cursor"
	return req
}
func (a *testCursorIntentAdapter) spec() cursorhooks.HookSpec {
	return cursorhooks.HookSpec{Executable: filepath.Join(a.root, "TEST-executable"), Selector: filepath.Join(a.root, "TEST-binding")}
}
func cursorValueReceipt(r *cursorhooks.Receipt) domain.CursorHookReceipt {
	return domain.CursorHookReceipt{Version: r.Version, Event: r.Event, Executable: r.Spec.Executable, Selector: r.Spec.Selector, Shell: string(r.Shell), EntryDigest: r.EntryDigest, RemainderDigest: r.RemainderDigest}
}
func cursorPureReceipt(r domain.CursorHookReceipt) *cursorhooks.Receipt {
	return &cursorhooks.Receipt{Version: r.Version, Event: r.Event, Spec: cursorhooks.HookSpec{Executable: r.Executable, Selector: r.Selector}, Shell: cursorhooks.ShellContract(r.Shell), EntryDigest: r.EntryDigest, RemainderDigest: r.RemainderDigest}
}
func cursorRawDigest(body []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(body)) }
func (a *testCursorIntentAdapter) RefinePlan(ctx context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := filepath.Join(in.Client.ConfigRoot, "hooks.json")
	file, err := a.kernel.BeginExactFile(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	original := file.Original()
	if original.Exists && len(original.Body) == 0 {
		return fmt.Errorf("present empty hook document")
	}
	var previous *cursorhooks.Receipt
	for _, o := range in.PreviousNativeObjects {
		if o.Kind == "cursor_user_stop" {
			previous = cursorPureReceipt(o.CursorReceipt)
		}
	}
	op := cursorhooks.Install
	if previous != nil {
		op = cursorhooks.Update
	}
	result, err := cursorhooks.Plan(cursorhooks.Request{Document: original.Body, Operation: op, Specs: []cursorhooks.HookSpec{a.spec()}, Previous: previous, Shell: cursorhooks.LinuxUserShell32212, ExecutableVerified: true})
	if err != nil {
		return err
	}
	receipt := cursorValueReceipt(result.Receipt)
	plan.NativeRegistryRoot = in.Client.ConfigRoot
	plan.SelectedDelivery, err = domain.NewCursorDelivery(domain.CursorDeliveryFacts{ProfileRoot: in.Client.ConfigRoot, HooksPath: path, ProfileIdentity: "TEST-Cursor-profile", CursorVersion: "2026.09.28-64d2043", TargetOS: "linux", TargetArch: "amd64", QualificationID: "TEST-injected-contract", Executable: receipt.Executable, Selector: receipt.Selector, Shell: receipt.Shell, ObjectID: "TEST-Cursor-Stop", EntryDigest: receipt.EntryDigest, CanonicalDigest: in.Envelope.TreeDigest, PlannedReceipt: receipt, OriginalExists: original.Exists, OriginalRawDigest: cursorRawDigest(original.Body)})
	return err
}
func (a *testCursorIntentAdapter) plannedFile(selected domain.SelectedDelivery, objects []domain.NativeObjectOwnership, kernel nativeconfig.Kernel, apply bool) (err error) {
	f, _ := selected.CursorFacts()
	file, err := kernel.BeginExactFile(f.HooksPath)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	original := file.Original()
	if original.Exists != f.OriginalExists || cursorRawDigest(original.Body) != f.OriginalRawDigest {
		return fmt.Errorf("stale exact Cursor original")
	}
	var previous *cursorhooks.Receipt
	for _, o := range objects {
		if o.Kind == "cursor_user_stop" {
			previous = cursorPureReceipt(o.CursorReceipt)
		}
	}
	op := cursorhooks.Install
	if previous != nil {
		op = cursorhooks.Update
	}
	planned, err := cursorhooks.Plan(cursorhooks.Request{Document: original.Body, Operation: op, Specs: []cursorhooks.HookSpec{a.spec()}, Previous: previous, Shell: cursorhooks.LinuxUserShell32212, ExecutableVerified: true})
	if err != nil {
		return err
	}
	if cursorValueReceipt(planned.Receipt) != f.PlannedReceipt {
		return fmt.Errorf("plan receipt changed")
	}
	if !apply {
		return nil
	}
	if err := file.Apply(planned.Desired); err != nil {
		return err
	}
	a.effects++
	if a.crash {
		// Return immediately after the real effect, without readback or an
		// acknowledgement. Activate exits once this ExactFile scope closes.
		return nil
	}
	actual, err := os.ReadFile(f.HooksPath)
	if err != nil {
		return err
	}
	return cursorhooks.VerifyOwned(actual, cursorPureReceipt(f.PlannedReceipt))
}
func (a *testCursorIntentAdapter) PreflightActivation(env clients.Env, req domain.ActivationRequest) error {
	if req.VerifyOnly && req.Plan.SelectedDelivery.OwnsProfileEntry(req.Plan.PreviousNativeObjects) {
		f, _ := req.Plan.SelectedDelivery.CursorFacts()
		body, err := os.ReadFile(f.HooksPath)
		if err != nil {
			return err
		}
		return cursorhooks.VerifyOwned(body, cursorPureReceipt(f.PlannedReceipt))
	}
	return a.plannedFile(req.Plan.SelectedDelivery, req.Plan.PreviousNativeObjects, env.NativeConfig, false)
}
func (a *testCursorIntentAdapter) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if req.VerifyOnly {
		a.readbacks++
		f, _ := req.Plan.SelectedDelivery.CursorFacts()
		body, err := os.ReadFile(f.HooksPath)
		if err == nil {
			err = cursorhooks.VerifyOwned(body, cursorPureReceipt(f.PlannedReceipt))
		}
		outcome := domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled}
		if a.manual {
			outcome, _ = a.Adapter.Activate(ctx, env, req)
		}
		outcome.NativeEffect, outcome.NativeObjects = domain.NativeEffectUnchanged, []domain.NativeObjectOwnership{req.Plan.SelectedDelivery.CursorOwnership(f.PlannedReceipt)}
		if a.manual {
			outcome.NativeObjects = nil
		}
		return outcome, err
	}
	state, err := (statev2.Store{Path: filepath.Join(a.root, "state", "state-v2.json")}).Load()
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	binding := onlyLocalBinding(state)
	if binding.PendingNativeIntent == nil || !reflect.DeepEqual(binding.PendingNativeIntent.Delivery, req.Plan.SelectedDelivery) {
		return domain.ActivationOutcome{}, fmt.Errorf("effect before exact pending packet")
	}
	var predecessor domain.NativeObjectOwnership
	for _, object := range req.Plan.PreviousNativeObjects {
		if object.Kind == "cursor_user_stop" {
			predecessor = object
		}
	}
	if binding.PendingNativeIntent.PreviousCursorObject != predecessor {
		return domain.ActivationOutcome{}, fmt.Errorf("acknowledged predecessor was recaptured")
	}
	if err := a.plannedFile(req.Plan.SelectedDelivery, req.Plan.PreviousNativeObjects, env.NativeConfig, true); err != nil {
		return domain.ActivationOutcome{NativeEffect: domain.NativeEffectUncertain}, err
	}
	if a.crash {
		os.Exit(91)
	}
	f, _ := req.Plan.SelectedDelivery.CursorFacts()
	outcome := domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled}
	if a.manual {
		outcome, _ = a.Adapter.Activate(ctx, env, req)
	}
	outcome.NativeEffect, outcome.NativeObjects = domain.NativeEffectCommitted, []domain.NativeObjectOwnership{req.Plan.SelectedDelivery.CursorOwnership(f.PlannedReceipt)}
	return outcome, nil
}
func (a *testCursorIntentAdapter) ReconcileNativeIntent(ctx context.Context, intent domain.PendingNativeIntent) (domain.ActivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActivationOutcome{}, err
	}
	f, _ := intent.Delivery.CursorFacts()
	body, err := os.ReadFile(f.HooksPath)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := cursorhooks.VerifyOwned(body, cursorPureReceipt(f.PlannedReceipt)); err != nil {
		return domain.ActivationOutcome{}, err
	}
	a.readbacks++
	return domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled, NativeEffect: domain.NativeEffectUnchanged, NativeObjects: []domain.NativeObjectOwnership{intent.Delivery.CursorOwnership(f.PlannedReceipt)}}, nil
}
