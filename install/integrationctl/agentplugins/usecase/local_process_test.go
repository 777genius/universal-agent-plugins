package usecase_test

import (
	"context"
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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
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
