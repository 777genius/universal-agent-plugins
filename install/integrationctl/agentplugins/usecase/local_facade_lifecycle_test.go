package usecase_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tailscale/hujson"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Observable RED boundaries: the old facade forgets persisted removal authority,
// hides an actual child's durable native intent, and requires a helper for a
// selected native-only package. These assertions compile on exact 04034324.
type lifecycleFacadeAdapter struct {
	testEffectLocalAdapter
	reconciliations, removals, previews int
	disabled, chooseMCP                 bool
	beforeReconcile                     func()
	beforeWrite                         func()
}

func (a *lifecycleFacadeAdapter) ReconcileNativeIntent(ctx context.Context, in domain.PendingNativeIntent) (domain.ActivationOutcome, error) {
	a.reconciliations++
	if a.beforeReconcile != nil {
		a.beforeReconcile()
	}
	facts, _ := in.Delivery.LocalFacts()
	remove := in.Direction == domain.NativeIntentRemove
	if !remove || in.RemoveOwnedEntry {
		if err := lifecycleProfileEffect(nativeconfig.New(), facts, remove, true, a.beforeWrite); err != nil {
			return domain.ActivationOutcome{}, err
		}
	}
	return lifecycleRegisteredOutcome(in.Delivery, remove), nil
}
func lifecycleFacadeEngine(t *testing.T, root string, a clients.Adapter, helper bool) *installer.Engine {
	t.Helper()
	registry, err := clients.NewRegistry(a)
	if err != nil {
		t.Fatal(err)
	}
	cfg := installer.Config{StateRoot: filepath.Join(root, "state"), Registry: registry, TrustedLocalPackages: true}
	if helper {
		cfg.HelperExecutable, err = os.Executable()
		if err != nil {
			t.Fatal(err)
		}
	}
	e, err := installer.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func lifecycleAdapter(root string) *lifecycleFacadeAdapter {
	return &lifecycleFacadeAdapter{testEffectLocalAdapter: testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root}}
}
func lifecycleInstall(t *testing.T, e *installer.Engine, root string) installer.Result {
	t.Helper()
	h, err := e.Prepare(t.Context(), installerRequest(root))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()
	r, err := e.Apply(t.Context(), h, confirmedDecision())
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestFacadeLifecycleSelectedRemove(t *testing.T) {
	root := localProcessRoot(t)
	a := lifecycleAdapter(root)
	e := lifecycleFacadeEngine(t, root, a, true)
	installed := lifecycleInstall(t, e, root)
	req := installerRequest(root)
	req.Operation = installer.OpRemove
	req.InstallationID = installed.InstallationID
	h, err := e.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()
	result, err := e.Apply(t.Context(), h, confirmedDecision())
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != installer.OutcomeCompleted || !reflect.DeepEqual(h.Plan().Client.SelectedDelivery, installed.Binding.SelectedDelivery) || !reflect.DeepEqual(result.Client.SelectedDelivery, installed.Binding.SelectedDelivery) || !reflect.DeepEqual(result.Binding.SelectedDelivery, installed.Binding.SelectedDelivery) {
		t.Fatalf("lost persisted removal authority: %+v", result)
	}
}

// This child uses public Engine for both directions, including removal. Death
// follows the real nativeconfig ExactFile write, after the service persisted
// intent; no production test switch or command runner simulates the crash.
func TestFacadeLifecycleChild(t *testing.T) {
	root := os.Getenv("FACADE_LIFECYCLE_TEST_ROOT")
	if root == "" {
		return
	}
	a := lifecycleAdapter(root)
	a.crash = domain.NativeIntentDirection(os.Getenv("FACADE_LIFECYCLE_TEST_DIRECTION"))
	a.disabled = os.Getenv("FACADE_LIFECYCLE_TEST_DISABLED") == "true"
	e := lifecycleFacadeEngine(t, root, a, true)
	installed := lifecycleInstall(t, e, root)
	req := installerRequest(root)
	req.Operation = installer.OpRemove
	req.InstallationID = installed.InstallationID
	h, err := e.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.Apply(t.Context(), h, confirmedDecision()); err != nil {
		t.Fatal(err)
	}
	t.Fatal("public profile effect did not terminate child")
}
func runFacadeLifecycleCrash(t *testing.T, root string, direction domain.NativeIntentDirection, disabled ...bool) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestFacadeLifecycleChild$", "-test.timeout=20s")
	cmd.Env = []string{"HOME=" + filepath.Join(root, "TEST-home"), "USERPROFILE=" + filepath.Join(root, "TEST-home"), "TMPDIR=" + root, "FACADE_LIFECYCLE_TEST_ROOT=" + root, "FACADE_LIFECYCLE_TEST_DIRECTION=" + string(direction)}
	if len(disabled) > 0 && disabled[0] {
		cmd.Env = append(cmd.Env, "FACADE_LIFECYCLE_TEST_DISABLED=true")
	}
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 91 {
		t.Fatalf("public child expected exit 91: %v %s", err, output)
	}
}

func TestFacadeLifecyclePublicChildScopes(t *testing.T) {
	for _, direction := range []domain.NativeIntentDirection{domain.NativeIntentRegister, domain.NativeIntentRemove} {
		t.Run(string(direction), func(t *testing.T) {
			root := localProcessRoot(t)
			runFacadeLifecycleCrash(t, root, direction)
			a := lifecycleAdapter(root)
			e := lifecycleFacadeEngine(t, root, a, false)
			state, err := localRecoveryService(t, root).StateStore.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := localOnlyBinding(t, state)
			facts := mustFacts(t, binding)
			before := lifecycleFiles(t, root)
			view, err := e.Inspect(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, lifecycleFiles(t, root)) || len(view.Recovery.NativeIntents) != 1 {
				t.Fatal("Inspect changed files or omitted scope")
			}
			rawView, err := json.Marshal(view)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(rawView, []byte(binding.NativeActivationAttempt)) || !bytes.Contains(rawView, []byte(facts.Tuple.QualificationID)) {
				t.Fatal("Inspect diagnostic omitted native attempt/tuple")
			}
			pending := view.Recovery.NativeIntents[0]
			if pending.Digest == "" || pending.Intent.AttemptID != binding.NativeActivationAttempt || pending.Intent.Direction != direction || pending.Binding.BindingID != binding.ClientBindingID || pending.Binding.InstallationID != state.Installations[0].InstallationID || pending.NativeProfileRoot != facts.ProfileRoot || !reflect.DeepEqual(pending.Intent, *binding.PendingNativeIntent) || !reflect.DeepEqual(pending.Binding.SelectedDelivery, binding.SelectedDelivery) {
				t.Fatalf("inexact Inspect native scope: %+v", pending)
			}
			// Constructor facts and ambient HOME change after observation are irrelevant.
			a.targetShell = "TEST-ambient-unqualified-shell"
			a.beforeReconcile = func() {
				release, err := (processlock.Lock{Path: filepath.Join(root, "state", "mutation.lock")}).Acquire(t.Context())
				if release != nil {
					_ = release()
				}
				if !errors.Is(err, processlock.ErrActive) {
					t.Fatalf("Recover did not retain one mutation lock: %v", err)
				}
			}
			doc := readLocalDocument(t, facts.SettingsPath)
			doc["late.foreign"] = "TEST-preserve"
			writeLocalDocument(t, facts.SettingsPath, doc)
			profile, _ := os.ReadFile(facts.SettingsPath)
			result, err := e.Recover(t.Context(), view)
			if err != nil {
				t.Fatal(err)
			}
			after, _ := os.ReadFile(facts.SettingsPath)
			if result.Outcome != installer.OutcomeCompleted || len(result.Recovery.Resolved) != 1 || a.reconciliations != 1 || string(profile) != string(after) {
				t.Fatalf("scoped recovery changed foreign data: %+v %v", result, err)
			}
			final, err := e.Inspect(t.Context())
			if err != nil || final.Recovery.Required {
				t.Fatalf("unreconciled intent: %+v %v", final, err)
			}
			if direction == domain.NativeIntentRemove {
				req := installerRequest(root)
				req.Operation = installer.OpRemove
				req.InstallationID = pending.Binding.InstallationID
				h, err := e.Prepare(t.Context(), req)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = h.Close() }()
				result, err := e.Apply(t.Context(), h, confirmedDecision())
				if err != nil || result.Outcome != installer.OutcomeCompleted {
					t.Fatalf("post-recovery managed cleanup: %+v %v", result, err)
				}
				if _, err := os.Lstat(facts.Registration.Selector); !os.IsNotExist(err) {
					t.Fatal("recovered removal left owned managed artifact")
				}
			}
		})
	}
}

func lifecycleFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[path] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFacadeLifecycleRecoveryStaleScopes(t *testing.T) {
	changes := map[string]func(*installer.Inspection){
		"json-roundtrip": func(v *installer.Inspection) {
			raw, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			var decoded installer.Inspection
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			*v = decoded
		},
		"digest":        func(v *installer.Inspection) { v.Recovery.NativeIntents[0].Digest += "stale" },
		"attempt":       func(v *installer.Inspection) { v.Recovery.NativeIntents[0].Intent.AttemptID += "stale" },
		"mode":          func(v *installer.Inspection) { v.Recovery.NativeIntents[0].Intent.Delivery = domain.SelectedDelivery{} },
		"root":          func(v *installer.Inspection) { v.StateRoot += "foreign" },
		"missing-root":  func(v *installer.Inspection) { v.StateRoot = "" },
		"binding":       func(v *installer.Inspection) { v.Recovery.NativeIntents[0].Binding.BindingID += "foreign" },
		"missing-scope": func(v *installer.Inspection) { v.Recovery.NativeIntents = nil },
		"profile":       func(v *installer.Inspection) { v.Recovery.NativeIntents[0].NativeProfileRoot += "foreign" },
	}
	root := localProcessRoot(t)
	runFacadeLifecycleCrash(t, root, domain.NativeIntentRegister)
	a := lifecycleAdapter(root)
	e := lifecycleFacadeEngine(t, root, a, false)
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			view, err := e.Inspect(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			change(&view)
			before := lifecycleFiles(t, root)
			result, err := e.Recover(t.Context(), view)
			if !errors.Is(err, installer.ErrPlanChanged) || result.Outcome != installer.OutcomeConflict || a.reconciliations != 0 || !reflect.DeepEqual(before, lifecycleFiles(t, root)) {
				t.Fatalf("stale scope had effects: %+v %v", result, err)
			}
		})
	}
}

func (a *lifecycleFacadeAdapter) RefinePlan(ctx context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	selected := facadeLocalAdapter{testEffectLocalAdapter: a.testEffectLocalAdapter, chooseMCP: a.chooseMCP}
	if a.disabled {
		selected.change = func(f *domain.LocalDeliveryFacts) { disabled := false; f.Registration.DesiredValue = &disabled }
	}
	return selected.RefinePlan(ctx, in, plan)
}
func (a *lifecycleFacadeAdapter) Activate(_ context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if req.VerifyOnly {
		return domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationPackageValid}, nil
	}
	if err := a.requireDurableIntent(req.Plan.SelectedDelivery, domain.NativeIntentRegister); err != nil {
		return domain.ActivationOutcome{}, err
	}
	facts, _ := req.Plan.SelectedDelivery.LocalFacts()
	if err := lifecycleProfileEffect(env.NativeConfig, facts, false, false, a.beforeWrite); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if a.crash == domain.NativeIntentRegister {
		os.Exit(91)
	}
	return lifecycleRegisteredOutcome(req.Plan.SelectedDelivery, false), nil
}
func (a *lifecycleFacadeAdapter) Deactivate(_ context.Context, env clients.Env, req domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	facts, ok := req.SelectedDelivery.LocalFacts()
	if !ok {
		return domain.DeactivationOutcome{}, fmt.Errorf("TEST removal lost selection")
	}
	if !req.RemoveOwnedEntry {
		snapshot, err := env.NativeConfig.ReadExactFile(facts.SettingsPath)
		if err != nil {
			return domain.DeactivationOutcome{}, err
		}
		doc, err := lifecycleDocument(snapshot.Body)
		if err != nil {
			return domain.DeactivationOutcome{}, err
		}
		locations, ok := doc["chat.pluginLocations"].(map[string]any)
		if !ok {
			return domain.DeactivationOutcome{}, fmt.Errorf("TEST locations malformed")
		}
		if _, present := locations[facts.Registration.Selector]; present {
			return domain.DeactivationOutcome{}, fmt.Errorf("TEST unowned selector cannot be removed")
		}
		return domain.DeactivationOutcome{ArtifactRemovalAllowed: true, ExternalRemovalComplete: true}, nil
	}
	if !req.Confirmed {
		a.previews++
		snapshot, err := env.NativeConfig.ReadExactFile(facts.SettingsPath)
		if err != nil {
			return domain.DeactivationOutcome{}, err
		}
		doc, err := lifecycleDocument(snapshot.Body)
		if err != nil {
			return domain.DeactivationOutcome{}, err
		}
		locations, ok := doc["chat.pluginLocations"].(map[string]any)
		if !ok || locations[facts.Registration.Selector] != *facts.Registration.DesiredValue || !req.RemoveOwnedEntry {
			return domain.DeactivationOutcome{}, fmt.Errorf("TEST preview refuses foreign ownership")
		}
		return domain.DeactivationOutcome{ArtifactRemovalAllowed: true}, nil
	}
	if err := a.requireDurableIntent(req.SelectedDelivery, domain.NativeIntentRemove); err != nil {
		return domain.DeactivationOutcome{}, err
	}
	a.removals++
	if err := lifecycleProfileEffect(env.NativeConfig, facts, true, false, a.beforeWrite); err != nil {
		return domain.DeactivationOutcome{}, err
	}
	if a.crash == domain.NativeIntentRemove {
		os.Exit(91)
	}
	return domain.DeactivationOutcome{ArtifactRemovalAllowed: true, ExternalRemovalComplete: true}, nil
}
func (a *lifecycleFacadeAdapter) requireDurableIntent(selected domain.SelectedDelivery, direction domain.NativeIntentDirection) error {
	state, err := (statev2.Store{Path: filepath.Join(a.root, "state", "state-v2.json")}).Load()
	if err != nil {
		return err
	}
	for _, binding := range state.Installations[0].Clients {
		if !reflect.DeepEqual(binding.SelectedDelivery, selected) {
			continue
		}
		if binding.PendingNativeIntent != nil && binding.PendingNativeIntent.Direction == direction {
			return binding.PendingNativeIntent.Validate(binding)
		}
	}
	return fmt.Errorf("TEST native effect has no exact durable authority")
}
func lifecycleRegisteredOutcome(selected domain.SelectedDelivery, remove bool) domain.ActivationOutcome {
	outcome := domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled, NativeEffect: domain.NativeEffectCommitted}
	if !remove {
		facts, _ := selected.LocalFacts()
		outcome.NativeObjects = []domain.NativeObjectOwnership{facts.Registration.Ownership(facts.SettingsPath)}
	}
	return outcome
}
func lifecycleDocument(raw []byte) (map[string]any, error) {
	standard, err := hujson.Standardize(bytes.Clone(raw))
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	err = json.Unmarshal(standard, &doc)
	return doc, err
}

// TEST JSONC selector adapter over the actual nativeconfig lock/CAS/rollback
// kernel. This is public facade proof, never native VS Code qualification.
func lifecycleProfileEffect(kernel nativeconfig.Kernel, facts domain.LocalDeliveryFacts, remove, recovering bool, beforeWrite func()) error {
	file, err := kernel.BeginExactFile(facts.SettingsPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	raw := file.Original().Body
	doc, err := lifecycleDocument(raw)
	if err != nil {
		return err
	}
	locations, ok := doc["chat.pluginLocations"].(map[string]any)
	if !ok {
		return fmt.Errorf("TEST locations malformed")
	}
	value, exists := locations[facts.Registration.Selector]
	if exists && value != *facts.Registration.DesiredValue {
		return fmt.Errorf("TEST selector drift; retain uncertainty")
	}
	if recovering && !remove && !exists {
		return fmt.Errorf("TEST registration disappeared; retain uncertainty")
	}
	if recovering && (remove && !exists || !remove && exists) {
		return nil
	}
	if !remove && exists {
		return fmt.Errorf("TEST identical unowned selector cannot be adopted")
	}
	op := "add"
	if remove {
		op = "remove"
	}
	pointer := strings.NewReplacer("~", "~0", "/", "~1").Replace(facts.Registration.Selector)
	patch, err := json.Marshal([]map[string]any{{"op": op, "path": "/chat.pluginLocations/" + pointer, "value": *facts.Registration.DesiredValue}})
	if err != nil {
		return err
	}
	ast, err := hujson.Parse(raw)
	if err != nil {
		return err
	}
	if err := ast.Patch(patch); err != nil {
		return err
	}
	if beforeWrite != nil {
		beforeWrite()
	}
	if err := file.Apply(ast.Pack()); err != nil {
		return errors.Join(err, file.Rollback())
	}
	return nil
}

func TestFacadeLifecycleRecoveryForeignAndFalse(t *testing.T) {
	for _, name := range []string{"preserve-false", "late-disabled", "changed-package", "missing-intent", "late-binding"} {
		t.Run(name, func(t *testing.T) {
			root := localProcessRoot(t)
			settings := filepath.Join(root, "profile", "settings.json")
			if err := os.WriteFile(settings, []byte("{\n // TEST foreign comment\n \"foreign.setting\": false,\n \"chat.pluginLocations\": {},\n}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			runFacadeLifecycleCrash(t, root, domain.NativeIntentRegister, name == "preserve-false")
			a := lifecycleAdapter(root)
			e := lifecycleFacadeEngine(t, root, a, false)
			view, err := e.Inspect(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			facts, _ := view.Recovery.NativeIntents[0].Intent.Delivery.LocalFacts()
			if name == "changed-package" {
				if err := os.WriteFile(filepath.Join(facts.Registration.Selector, "foreign-edit"), []byte("TEST-foreign"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if name == "missing-intent" {
				store := statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}
				state, err := store.Load()
				if err != nil {
					t.Fatal(err)
				}
				binding := localOnlyBinding(t, state)
				binding.PendingNativeIntent = nil
				state.Installations[0].Clients[binding.ClientBindingID] = binding
				if err := store.Save(state); err != nil {
					t.Fatal(err)
				}
			}
			if name == "late-binding" {
				a.beforeReconcile = func() {
					lifecycleChangeBinding(t, root, func(b *domain.ClientBinding) {
						f, _ := b.SelectedDelivery.LocalFacts()
						f.Tuple.TargetShell += "-foreign"
						b.SelectedDelivery, err = domain.NewLocalDelivery(f)
						if err != nil {
							t.Fatal(err)
						}
						b.PendingNativeIntent.Delivery = b.SelectedDelivery
					})
				}
			}
			if name == "late-disabled" {
				a.beforeReconcile = func() { lifecycleSetSelector(t, settings, facts.Registration.Selector, false) }
			}
			beforeState, _ := os.ReadFile(filepath.Join(root, "state", "state-v2.json"))
			result, err := e.Recover(t.Context(), view)
			profile, _ := os.ReadFile(settings)
			if name == "preserve-false" {
				doc, parseErr := lifecycleDocument(profile)
				if err != nil || parseErr != nil || result.Outcome != installer.OutcomeCompleted || doc["chat.pluginLocations"].(map[string]any)[facts.Registration.Selector] != false || !bytes.Contains(profile, []byte("TEST foreign comment")) {
					t.Fatalf("false/comment lost: %s %+v %v", profile, result, err)
				}
				return
			}
			afterState, _ := os.ReadFile(filepath.Join(root, "state", "state-v2.json"))
			if err == nil || result.Reason == "already_recovered" || (name != "late-binding" && !bytes.Equal(beforeState, afterState)) {
				t.Fatalf("uncertainty cleared: %+v %v", result, err)
			}
			if name == "late-binding" {
				final, err := e.Inspect(t.Context())
				if err != nil || !final.Recovery.Required {
					t.Fatalf("late binding uncertainty cleared: %+v %v", final, err)
				}
			}
			if name == "late-disabled" {
				doc, _ := lifecycleDocument(profile)
				if doc["chat.pluginLocations"].(map[string]any)[facts.Registration.Selector] != false {
					t.Fatal("foreign disabled selector enabled")
				}
			}
		})
	}
}
func lifecycleSetSelector(t *testing.T, path, selector string, value bool) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ast, err := hujson.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	pointer := strings.NewReplacer("~", "~0", "/", "~1").Replace(selector)
	patch, err := json.Marshal([]map[string]any{{"op": "add", "path": "/chat.pluginLocations/" + pointer, "value": value}})
	if err != nil {
		t.Fatal(err)
	}
	if err := ast.Patch(patch); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, ast.Pack(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestFacadeLifecycleLateForeignWrite(t *testing.T) {
	root := localProcessRoot(t)
	a := lifecycleAdapter(root)
	e := lifecycleFacadeEngine(t, root, a, false)
	installed := lifecycleInstall(t, e, root)
	facts, _ := installed.Binding.SelectedDelivery.LocalFacts()
	req := installerRequest(root)
	req.Operation = installer.OpRemove
	req.InstallationID = installed.InstallationID
	h, err := e.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()
	var foreign []byte
	a.beforeWrite = func() {
		lifecycleSetSelector(t, facts.SettingsPath, facts.Registration.Selector, false)
		foreign, _ = os.ReadFile(facts.SettingsPath)
	}
	result, err := e.Apply(t.Context(), h, confirmedDecision())
	actual, _ := os.ReadFile(facts.SettingsPath)
	if err == nil || result.Outcome == installer.OutcomeCompleted || !bytes.Equal(foreign, actual) {
		t.Fatalf("CAS overwrote late disabled entry: %+v %v", result, err)
	}
	a.beforeWrite = nil
	view, err := e.Inspect(t.Context())
	if err != nil || !view.Recovery.Required || len(view.Recovery.NativeIntents) != 1 {
		t.Fatalf("uncertain removal disappeared: %+v %v", view, err)
	}
	stateBefore, _ := os.ReadFile(filepath.Join(root, "state", "state-v2.json"))
	recovered, err := e.Recover(t.Context(), view)
	if err == nil || recovered.Reason == "already_recovered" || !bytes.Equal(stateBefore, lifecycleStateBytes(t, root)) || !bytes.Equal(foreign, lifecycleProfileBytes(t, root)) {
		t.Fatalf("foreign recovery cleared uncertainty: %+v %v", recovered, err)
	}
}

func lifecycleGroup(t *testing.T, root string, a *lifecycleFacadeAdapter, cfg installer.Config) (*installer.Engine, installer.Request) {
	t.Helper()
	installCfg := cfg
	installCfg.Runner = nil
	installCfg.EnableNativeObserver = false
	e := facadeEngine(t, root, a, installCfg, codex.New())
	installed := lifecycleInstall(t, e, root)
	codexRoot := filepath.Join(root, "TEST-codex-profile")
	if err := os.MkdirAll(codexRoot, 0700); err != nil {
		t.Fatal(err)
	}
	request := installerRequest(root)
	request.ClientID = "codex"
	request.ClientConfigRoot = codexRoot
	request.ClientExecutable = filepath.Join(root, "TEST-executable")
	request.InstallationID = installed.InstallationID
	h, err := e.Prepare(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.Apply(t.Context(), h, confirmedDecision())
	_ = h.Close()
	if err != nil {
		t.Fatal(err)
	}
	request.Operation = installer.OpRemove
	request.Targets = []installer.ClientTarget{{ClientID: "codex", ClientConfigRoot: codexRoot, ExternalUninstalled: true}, {ClientID: "vscode", ClientConfigRoot: filepath.Join(root, "profile")}}
	return facadeEngine(t, root, a, cfg, codex.New()), request
}
func TestFacadeLifecycleGroupRemove(t *testing.T) {
	root := localProcessRoot(t)
	a := lifecycleAdapter(root)
	e, req := lifecycleGroup(t, root, a, installer.Config{})
	before := lifecycleFiles(t, root)
	h, err := e.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()
	plan := h.Plan()
	if len(plan.Targets) != 2 || !plan.Targets[0].SelectedDelivery.IsZero() || plan.Targets[1].SelectedDelivery.IsZero() || !reflect.DeepEqual(before, lifecycleFiles(t, root)) {
		t.Fatal("group removal did not retain historical/selected authority read-only")
	}
	a.targetShell = "TEST-ambient-change-is-not-removal-authority"
	result, err := e.Apply(t.Context(), h, confirmedDecision())
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != installer.OutcomeCompleted || a.removals != 1 || a.previews != 1 || len(result.Targets) != 2 || !reflect.DeepEqual(result.Targets[1].SelectedDelivery, plan.Targets[1].SelectedDelivery) {
		t.Fatalf("group lost persisted removal: %+v", result)
	}
	facts, _ := plan.Targets[1].SelectedDelivery.LocalFacts()
	doc := readLocalDocument(t, facts.SettingsPath)
	if _, ok := doc["chat.pluginLocations"].(map[string]any)[facts.Registration.Selector]; ok {
		t.Fatal("owned selector survived")
	}
	repeat, err := e.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = repeat.Close() }()
	if !repeat.Plan().NoChange {
		t.Fatal("absent selected group was reinterpreted as live")
	}
	again, err := e.Apply(t.Context(), repeat, confirmedDecision())
	if err != nil || again.Outcome != installer.OutcomeUnchanged || a.removals != 1 || a.previews != 1 {
		t.Fatalf("absent group redispatched: %+v %v", again, err)
	}
}
func TestFacadeLifecycleRemovalDriftBeforeEffects(t *testing.T) {
	for _, group := range []bool{false, true} {
		for _, phase := range []installer.ProgressPhase{"", installer.ProgressPreflight, installer.ProgressStage} {
			t.Run(fmt.Sprintf("group=%t/%s", group, phase), func(t *testing.T) {
				root := localProcessRoot(t)
				a := lifecycleAdapter(root)
				var change func()
				runner := &facadeCommandObserver{}
				cfg := installer.Config{Runner: runner, EnableNativeObserver: true, Progress: func(event installer.ProgressEvent) {
					if change != nil && event.Phase == phase {
						change()
					}
				}}
				var e *installer.Engine
				var req installer.Request
				if group {
					e, req = lifecycleGroup(t, root, a, cfg)
				} else {
					e = facadeEngine(t, root, a, installer.Config{})
					installed := lifecycleInstall(t, e, root)
					req = installerRequest(root)
					req.Operation = installer.OpRemove
					req.InstallationID = installed.InstallationID
					e = facadeEngine(t, root, a, cfg)
				}
				h, err := e.Prepare(t.Context(), req)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = h.Close() }()
				change = func() {
					lifecycleChangeBinding(t, root, func(b *domain.ClientBinding) {
						f, _ := b.SelectedDelivery.LocalFacts()
						f.Tuple.TargetShell += "-changed"
						b.SelectedDelivery, err = domain.NewLocalDelivery(f)
						if err != nil {
							t.Fatal(err)
						}
					})
				}
				if phase == "" {
					change()
				}
				// Existing install effects predate this removal attempt. Remove the empty
				// process-lock file to observe any new acquisition at this boundary.
				if err := os.Remove(filepath.Join(root, "state", "mutation.lock")); err != nil {
					t.Fatal(err)
				}
				commandsBefore := len(runner.commands)
				settingsBefore, _ := os.ReadFile(filepath.Join(root, "profile", "settings.json"))
				result, err := e.Apply(t.Context(), h, confirmedDecision())
				settingsAfter, _ := os.ReadFile(filepath.Join(root, "profile", "settings.json"))
				if !errors.Is(err, installer.ErrPlanChanged) || result.Outcome != installer.OutcomeConflict || a.removals != 0 || a.previews != 0 || len(runner.commands) != commandsBefore || !bytes.Equal(settingsBefore, settingsAfter) {
					t.Fatalf("removal drift reached effects: %+v %v", result, err)
				}
				if _, err := os.Lstat(filepath.Join(root, "state", "mutation.lock")); !os.IsNotExist(err) {
					t.Fatal("drift acquired mutation lock")
				}
			})
		}
	}
}
func lifecycleChangeBinding(t *testing.T, root string, change func(*domain.ClientBinding)) {
	t.Helper()
	store := statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for i, installation := range state.Installations {
		for key, binding := range installation.Clients {
			if !binding.SelectedDelivery.IsZero() {
				change(&binding)
				state.Installations[i].Clients[key] = binding
			}
		}
	}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
}

func lifecycleStateBytes(t *testing.T, root string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "state", "state-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func lifecycleProfileBytes(t *testing.T, root string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "profile", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestFacadeLifecycleHelperAndProjectionSelection(t *testing.T) {
	for _, name := range []string{"native-only-no-server", "native-only-configured-projector", "selected-MCP-missing-helper", "selected-MCP-with-helper", "configured-helper-missing", "historical-missing-helper"} {
		t.Run(name, func(t *testing.T) {
			root := localProcessRoot(t)
			a := lifecycleAdapter(root)
			a.chooseMCP = strings.HasPrefix(name, "selected-MCP")
			if a.chooseMCP {
				facadeComponents(t, root)
			}
			var adapter clients.Adapter = a
			if name == "historical-missing-helper" {
				adapter = vscode.New()
			}
			registry, err := clients.NewRegistry(adapter)
			if err != nil {
				t.Fatal(err)
			}
			projected := 0
			cfg := installer.Config{StateRoot: filepath.Join(root, "state"), Registry: registry, TrustedLocalPackages: true, ServerName: "notify", ProjectArgs: func(installer.BindingFacts) ([]string, error) { projected++; return []string{"TEST-projected"}, nil }}
			if name == "selected-MCP-with-helper" || name == "native-only-configured-projector" {
				cfg.HelperExecutable, err = os.Executable()
				if err != nil {
					t.Fatal(err)
				}
			}
			if name == "configured-helper-missing" {
				cfg.HelperExecutable = filepath.Join(root, "missing-TEST-helper")
			}
			e, err := installer.New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			h, err := e.Prepare(t.Context(), installerRequest(root))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = h.Close() }()
			result, err := e.Apply(t.Context(), h, confirmedDecision())
			success := strings.HasPrefix(name, "native-only") || name == "selected-MCP-with-helper"
			if success {
				if err != nil || result.Outcome != installer.OutcomeCompleted {
					t.Fatalf("conditional helper/projection: %+v %v", result, err)
				}
				expected := 0
				if a.chooseMCP {
					expected = 1
				}
				if projected != expected {
					t.Fatalf("projection calls=%d want %d", projected, expected)
				}
				if a.chooseMCP {
					raw, err := os.ReadFile(filepath.Join(result.Binding.TargetPath, "mcp.json"))
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Contains(raw, []byte("TEST-projected")) {
						t.Fatal("selected host projection missing")
					}
				}
				return
			}
			if err == nil || result.Outcome == installer.OutcomeCompleted || projected != 0 {
				t.Fatalf("missing helper admitted: %+v %v", result, err)
			}
			if _, err := os.Lstat(filepath.Join(root, "state", "mutation.lock")); !os.IsNotExist(err) {
				t.Fatal("helper refusal crossed mutation lock")
			}
		})
	}
}

// Two genuine public child registration crashes establish both persisted scopes.
// The callback then introduces a concrete late journal outside the observation.
func TestIndependentRecoveryLateJournal(t *testing.T) {
	for _, late := range []bool{false, true} {
		name := "matching-scopes"
		if late {
			name = "late-journal"
		}
		t.Run(name, func(t *testing.T) {
			first, second := localProcessRoot(t), localProcessRoot(t)
			runFacadeLifecycleCrash(t, first, domain.NativeIntentRegister)
			runFacadeLifecycleCrash(t, second, domain.NativeIntentRegister)
			store := statev2.Store{Path: filepath.Join(first, "state", "state-v2.json")}
			left, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			right, err := (statev2.Store{Path: filepath.Join(second, "state", "state-v2.json")}).Load()
			if err != nil {
				t.Fatal(err)
			}
			left.Installations = append(left.Installations, right.Installations...)
			if err := store.Save(left); err != nil {
				t.Fatal(err)
			}
			a := lifecycleAdapter(first)
			e := lifecycleFacadeEngine(t, first, a, false)
			view, err := e.Inspect(t.Context())
			if err != nil || len(view.Recovery.NativeIntents) != 2 {
				t.Fatalf("fixture scopes: %+v %v", view, err)
			}
			journalDir := filepath.Join(first, "state", "operations")
			staging := ""
			a.beforeReconcile = func() {
				if !late || a.reconciliations != 1 {
					return
				}
				owned := filepath.Join(first, "TEST-late-journal-owned")
				staging = filepath.Join(owned, ".agentplugins-staging-late")
				if err := os.MkdirAll(staging, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(staging, "TEST-sentinel"), []byte("retain unobserved staging"), 0600); err != nil {
					t.Fatal(err)
				}
				manager := dirswap.Manager{JournalDir: journalDir, Fault: func(phase string) error {
					if phase == dirswap.PhaseBackupPending {
						return errors.New("TEST leave late open journal")
					}
					return nil
				}}
				receipt, err := manager.Apply(context.Background(), dirswap.Input{OperationID: "TEST-late-journal", ClientBindingID: "TEST-foreign-binding", Sequence: 1, OwnedBase: owned, ActivePath: filepath.Join(owned, "active"), StagingPath: staging, RequireAbsent: true})
				if err == nil || receipt.OperationID != "TEST-late-journal" {
					t.Fatalf("fixture journal: %+v %v", receipt, err)
				}
				receipt.Phase = dirswap.PhaseIntent
				raw, err := json.Marshal(receipt)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(journalDir, receipt.OperationID+".json"), raw, 0600); err != nil {
					t.Fatal(err)
				}
				during, err := e.Inspect(t.Context())
				if err != nil || len(during.Recovery.Journals) != 1 {
					t.Fatalf("late scope absent: %+v %v", during, err)
				}
			}
			result, recoverErr := e.Recover(t.Context(), view)
			after, inspectErr := e.Inspect(t.Context())
			if inspectErr != nil {
				t.Fatal(inspectErr)
			}
			open, err := (dirswap.Manager{JournalDir: journalDir}).ListOpen()
			if err != nil {
				t.Fatal(err)
			}
			stagePresent := false
			if staging != "" {
				_, err := os.Stat(filepath.Join(staging, "TEST-sentinel"))
				stagePresent = err == nil
			}
			release, lockErr := (processlock.Lock{Path: filepath.Join(first, "state", "mutation.lock")}).Acquire(t.Context())
			if release != nil {
				_ = release()
			}
			if lockErr != nil {
				t.Fatalf("recovery retained boundary lock: %v", lockErr)
			}
			t.Logf("late=%t outcome=%s error=%v reconciliations=%d pendingIntents=%d openJournals=%d stagePresent=%t", late, result.Outcome, recoverErr, a.reconciliations, len(after.Recovery.NativeIntents), len(open), stagePresent)
			if !late {
				if recoverErr != nil || result.Outcome != installer.OutcomeCompleted || a.reconciliations != 2 || after.Recovery.Required {
					t.Fatal("matching scopes failed")
				}
				return
			}
			if !errors.Is(recoverErr, installer.ErrPlanChanged) || result.Outcome == installer.OutcomeCompleted || a.reconciliations != 1 || len(after.Recovery.NativeIntents) != 2 || len(open) != 1 || !stagePresent {
				t.Fatal("unobserved callback journal was recovered or pending authority cleared")
			}
		})
	}
}

// Existing regressions change the binding at Stage. This changes the actual
// persisted profile selector instead, leaving complete recorded facts intact.
func TestIndependentRemovalLateProfileDrift(t *testing.T) {
	for _, group := range []bool{false, true} {
		name := "single"
		if group {
			name = "group"
		}
		t.Run(name, func(t *testing.T) {
			root := localProcessRoot(t)
			a := lifecycleAdapter(root)
			var edit func()
			cfg := installer.Config{Progress: func(event installer.ProgressEvent) {
				if edit != nil && event.Phase == installer.ProgressStage {
					edit()
				}
			}}
			var e *installer.Engine
			var req installer.Request
			if group {
				e, req = lifecycleGroup(t, root, a, cfg)
			} else {
				original := lifecycleFacadeEngine(t, root, a, true)
				installed := lifecycleInstall(t, original, root)
				req = installerRequest(root)
				req.Operation = installer.OpRemove
				req.InstallationID = installed.InstallationID
				e = facadeEngine(t, root, a, cfg)
			}
			h, err := e.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = h.Close() }()
			state := mustLoadFacadeState(t, root)
			binding := independentSelectedBinding(t, state)
			facts := mustFacts(t, binding)
			beforeState := lifecycleStateBytes(t, root)
			var foreign []byte
			edit = func() {
				doc := readLocalDocument(t, facts.SettingsPath)
				doc["chat.pluginLocations"].(map[string]any)[facts.Registration.Selector] = "TEST-foreign-value"
				writeLocalDocument(t, facts.SettingsPath, doc)
				foreign = lifecycleProfileBytes(t, root)
			}
			lockPath := filepath.Join(root, "state", "mutation.lock")
			if err := os.Remove(lockPath); err != nil {
				t.Fatal(err)
			}
			result, applyErr := e.Apply(t.Context(), h, confirmedDecision())
			_, lockErr := os.Lstat(lockPath)
			lockPresent := lockErr == nil
			after := mustLoadFacadeState(t, root)
			pending := independentSelectedBinding(t, after).PendingNativeIntent != nil
			unchangedState := bytes.Equal(beforeState, lifecycleStateBytes(t, root))
			unchangedProfile := bytes.Equal(foreign, lifecycleProfileBytes(t, root))
			release, retainedErr := (processlock.Lock{Path: lockPath}).Acquire(t.Context())
			if release != nil {
				_ = release()
			}
			if retainedErr != nil {
				t.Fatalf("Remove retained boundary lock: %v", retainedErr)
			}
			t.Logf("group=%t outcome=%s error=%v lockCreated=%t previews=%d removals=%d pending=%t stateUnchanged=%t foreignProfilePreserved=%t", group, result.Outcome, applyErr, lockPresent, a.previews, a.removals, pending, unchangedState, unchangedProfile)
			if !errors.Is(applyErr, installer.ErrPlanChanged) || result.Outcome != installer.OutcomeConflict || lockPresent || a.previews != 0 || a.removals != 0 || pending || !unchangedState || !unchangedProfile {
				t.Fatal("late native selector drift crossed the public removal mutation boundary")
			}
		})
	}
}

func independentSelectedBinding(t *testing.T, state domain.StateFileV2) domain.ClientBinding {
	t.Helper()
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if !binding.SelectedDelivery.IsZero() {
				return binding
			}
		}
	}
	t.Fatal("TEST selected binding is absent")
	return domain.ClientBinding{}
}

// Read-only selected registry capability: actual ExactFile + the same JSONC
// parser as this TEST adapter's effect. This fixture is a facade contract,
// not actual NewLocal qualification. No native dispatch acts as preflight.
func (a *lifecycleFacadeAdapter) UsesNativeRegistryExecutable() bool { return false }
func (a *lifecycleFacadeAdapter) InspectNativeRegistry(ctx context.Context, env clients.Env, _ domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	facts, ok := plan.SelectedDelivery.LocalFacts()
	if !ok || managed == nil {
		return clients.RegistryIndeterminate, fmt.Errorf("TEST selected registry requires persisted binding")
	}
	snapshot, err := env.NativeConfig.ReadExactFile(facts.SettingsPath)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	doc, err := lifecycleDocument(snapshot.Body)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	locations, ok := doc["chat.pluginLocations"].(map[string]any)
	if !ok {
		return clients.RegistryIndeterminate, fmt.Errorf("TEST locations malformed")
	}
	value, present := locations[facts.Registration.Selector]
	if !managed.SelectedDelivery.OwnsProfileEntry(managed.NativeObjects) {
		if present {
			return clients.RegistryCollision, nil
		}
		return clients.RegistryClear, nil
	}
	if !present || value != *facts.Registration.DesiredValue {
		return clients.RegistryCollision, nil
	}
	return clients.RegistryExpected, nil
}

// Genuine public child/filesystem boundary. Rejected candidate RED acknowledges
// the crashed intent despite a changed nonpending sibling or callback receipt.
// All state outside recorded kernel/service transitions must retain uncertainty.
func TestFacadeLifecycleRecoveryScopeChanges(t *testing.T) {
	for _, kind := range []string{"sibling-before", "sibling-callback", "receipt-callback"} {
		t.Run(kind, func(t *testing.T) {
			root, other := localProcessRoot(t), localProcessRoot(t)
			runFacadeLifecycleCrash(t, root, domain.NativeIntentRegister)
			sibling := lifecycleAdapter(other)
			siblingInstalled := lifecycleInstall(t, lifecycleFacadeEngine(t, other, sibling, true), other)
			store := statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}
			state := mustLoadFacadeState(t, root)
			state.Installations = append(state.Installations, mustLoadFacadeState(t, other).Installations...)
			if err := store.Save(state); err != nil {
				t.Fatal(err)
			}
			a := lifecycleAdapter(root)
			e := lifecycleFacadeEngine(t, root, a, false)
			view, err := e.Inspect(t.Context())
			if err != nil || len(view.Recovery.NativeIntents) != 1 {
				t.Fatalf("fixture intent: %+v %v", view, err)
			}
			profile := lifecycleProfileBytes(t, root)
			var foreign []byte
			change := func() {
				live := mustLoadFacadeState(t, root)
				if kind == "receipt-callback" {
					live.TransactionReceipts = append(live.TransactionReceipts, domain.MutationReceipt{OperationID: "TEST-foreign-receipt", ClientBindingID: "TEST-foreign-binding", Sequence: 1, MutationType: "directory_remove", ActivePath: filepath.Join(root, "TEST-foreign-target"), Phase: "state_committed"})
				} else {
					for i, installation := range live.Installations {
						if installation.InstallationID != siblingInstalled.InstallationID {
							continue
						}
						for key, binding := range installation.Clients {
							binding.UpdatedAt = "TEST-foreign-sibling"
							live.Installations[i].Clients[key] = binding
						}
					}
				}
				if err := store.Save(live); err != nil {
					t.Fatal(err)
				}
				foreign = lifecycleStateBytes(t, root)
			}
			if kind == "sibling-before" {
				change()
			} else {
				a.beforeReconcile = change
			}
			result, err := e.Recover(t.Context(), view)
			after, inspectErr := e.Inspect(t.Context())
			expectedCalls := 1
			if kind == "sibling-before" {
				expectedCalls = 0
			}
			if !errors.Is(err, installer.ErrPlanChanged) || result.Outcome != installer.OutcomeConflict || inspectErr != nil || len(after.Recovery.NativeIntents) != 1 || a.reconciliations != expectedCalls || !bytes.Equal(foreign, lifecycleStateBytes(t, root)) || !bytes.Equal(profile, lifecycleProfileBytes(t, root)) {
				t.Fatalf("foreign scope acknowledged: kind=%s result=%+v err=%v calls=%d after=%+v inspect=%v", kind, result, err, a.reconciliations, after.Recovery, inspectErr)
			}
		})
	}
}

// A genuine directory publication journal is already observed along with a
// real child's native intent. Only its legitimate committed phase may advance.
func TestReviewR2InitialJournalWithNativeIntent(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "matching"
		if canceled {
			name = "canceled"
		}
		t.Run(name, func(t *testing.T) {
			root := localProcessRoot(t)
			runFacadeLifecycleCrash(t, root, domain.NativeIntentRegister)
			owned := filepath.Join(root, "TEST-journal-owned")
			staging := filepath.Join(owned, ".agentplugins-staging-r2")
			if err := os.MkdirAll(staging, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(staging, "TEST-sentinel"), []byte("TEST-publication"), 0600); err != nil {
				t.Fatal(err)
			}
			journalDir := filepath.Join(root, "state", "operations")
			manager := dirswap.Manager{JournalDir: journalDir, Fault: func(phase string) error {
				if phase == dirswap.PhaseActivated {
					return errors.New("TEST interrupted publication")
				}
				return nil
			}}
			receipt, err := manager.Apply(t.Context(), dirswap.Input{OperationID: "TEST-observed-journal-r2", ClientBindingID: "TEST-journal-binding", Sequence: 1, OwnedBase: owned, ActivePath: filepath.Join(owned, "active"), StagingPath: staging, RequireAbsent: true})
			if err == nil || receipt.Phase != dirswap.PhaseActivated {
				t.Fatalf("TEST pending publication: %+v %v", receipt, err)
			}
			store := statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}
			state := mustLoadFacadeState(t, root)
			state.TransactionReceipts = append(state.TransactionReceipts, domain.MutationReceipt{OperationID: receipt.OperationID, ClientBindingID: receipt.ClientBindingID, Sequence: receipt.Sequence, MutationType: "directory_swap", ActivePath: receipt.ActivePath, StagingPath: receipt.StagingPath, BackupPath: receipt.BackupPath, Phase: "state_committed"})
			if err := store.Save(state); err != nil {
				t.Fatal(err)
			}
			a := lifecycleAdapter(root)
			e := lifecycleFacadeEngine(t, root, a, false)
			view, err := e.Inspect(t.Context())
			if err != nil || len(view.Recovery.Journals) != 1 || len(view.Recovery.NativeIntents) != 1 {
				t.Fatalf("TEST observed scope: %+v %v", view, err)
			}
			before := lifecycleFiles(t, root)
			// The first native callback must see the existing kernel's exact transition.
			a.beforeReconcile = func() {
				current := mustLoadFacadeState(t, root)
				found := false
				for _, item := range current.TransactionReceipts {
					if item.OperationID == receipt.OperationID {
						found = item.Phase == "committed"
					}
				}
				open, err := (dirswap.Manager{JournalDir: journalDir}).ListOpen()
				if err != nil || len(open) != 0 || !found {
					t.Fatalf("kernel ordering: journals=%+v committed=%t err=%v", open, found, err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if canceled {
				cancel()
			}
			result, err := e.Recover(ctx, view)
			after, inspectErr := e.Inspect(t.Context())
			if inspectErr != nil {
				t.Fatal(inspectErr)
			}
			if canceled {
				if !errors.Is(err, context.Canceled) || a.reconciliations != 0 || !bytes.Equal(lifecycleStateBytes(t, root), []byte(before[filepath.Join(root, "state", "state-v2.json")])) || len(after.Recovery.Journals) != 1 || len(after.Recovery.NativeIntents) != 1 {
					t.Fatalf("canceled recovery consumed scope: %+v %v", result, err)
				}
			} else {
				if err != nil || result.Outcome != installer.OutcomeCompleted || after.Recovery.Required || a.reconciliations != 1 {
					t.Fatalf("legitimate journal/native recovery: %+v %v", result, err)
				}
				raw, err := os.ReadFile(filepath.Join(receipt.ActivePath, "TEST-sentinel"))
				if err != nil || string(raw) != "TEST-publication" {
					t.Fatal("journal publication lost")
				}
			}
			release, lockErr := (processlock.Lock{Path: filepath.Join(root, "state", "mutation.lock")}).Acquire(t.Context())
			if release != nil {
				_ = release()
			}
			if lockErr != nil {
				t.Fatal(lockErr)
			}
		})
	}
}

// A public context check is the deterministic schedule point for a concurrent
// non-cooperating state edit during initial journal recovery. It returns the
// underlying context's real error and never injects production code or a store.
type reviewR2Context struct {
	context.Context
	check func()
}

func (c reviewR2Context) Err() error { c.check(); return c.Context.Err() }

func TestReviewR2InitialKernelLateTerminalReceipt(t *testing.T) {
	root := localProcessRoot(t)
	runFacadeLifecycleCrash(t, root, domain.NativeIntentRegister)
	owned := filepath.Join(root, "TEST-journal-owned")
	staging := filepath.Join(owned, ".agentplugins-staging-r2")
	if err := os.MkdirAll(staging, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "TEST-sentinel"), []byte("TEST-publication"), 0600); err != nil {
		t.Fatal(err)
	}
	journalDir := filepath.Join(root, "state", "operations")
	manager := dirswap.Manager{JournalDir: journalDir, Fault: func(phase string) error {
		if phase == dirswap.PhaseActivated {
			return errors.New("TEST interrupted publication")
		}
		return nil
	}}
	receipt, err := manager.Apply(t.Context(), dirswap.Input{OperationID: "TEST-observed-journal-r2", ClientBindingID: "TEST-journal-binding", Sequence: 1, OwnedBase: owned, ActivePath: filepath.Join(owned, "active"), StagingPath: staging, RequireAbsent: true})
	if err == nil || receipt.Phase != dirswap.PhaseActivated {
		t.Fatalf("TEST publication: %+v %v", receipt, err)
	}
	store := statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}
	state := mustLoadFacadeState(t, root)
	state.TransactionReceipts = append(state.TransactionReceipts, domain.MutationReceipt{OperationID: receipt.OperationID, ClientBindingID: receipt.ClientBindingID, Sequence: receipt.Sequence, MutationType: "directory_swap", ActivePath: receipt.ActivePath, StagingPath: receipt.StagingPath, BackupPath: receipt.BackupPath, Phase: "state_committed"})
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	a := lifecycleAdapter(root)
	e := lifecycleFacadeEngine(t, root, a, false)
	view, err := e.Inspect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	beforeProfile := lifecycleProfileBytes(t, root)
	lockPath := filepath.Join(root, "state", "mutation.lock")
	if err := os.Remove(lockPath); err != nil {
		t.Fatal(err)
	}
	fired := false
	var foreign []byte
	ctx := reviewR2Context{Context: t.Context(), check: func() {
		if fired {
			return
		}
		if _, err := os.Stat(lockPath); os.IsNotExist(err) {
			return
		} else if err != nil {
			t.Fatal(err)
		}
		// The real process lock is held and the observed journal is still open.
		release, lockErr := (processlock.Lock{Path: lockPath}).Acquire(context.Background())
		if release != nil {
			_ = release()
		}
		if !errors.Is(lockErr, processlock.ErrActive) {
			t.Fatalf("TEST scheduling lock: %v", lockErr)
		}
		fired = true
		live := mustLoadFacadeState(t, root)
		live.TransactionReceipts = append(live.TransactionReceipts, domain.MutationReceipt{OperationID: "TEST-late-terminal", ClientBindingID: "TEST-foreign-binding", Sequence: 1, MutationType: "directory_remove", ActivePath: filepath.Join(root, "TEST-foreign-target"), Phase: "committed"})
		if err := store.Save(live); err != nil {
			t.Fatal(err)
		}
		foreign = lifecycleStateBytes(t, root)
	}}
	result, recoverErr := e.Recover(ctx, view)
	after, inspectErr := e.Inspect(t.Context())
	if inspectErr != nil {
		t.Fatal(inspectErr)
	}
	live := mustLoadFacadeState(t, root)
	kept := false
	for _, item := range live.TransactionReceipts {
		if item.OperationID == "TEST-late-terminal" {
			kept = true
		}
	}
	t.Logf("fired=%t outcome=%s err=%v calls=%d pending=%d lateTerminalRetained=%t exactForeignStateRetained=%t profilePreserved=%t", fired, result.Outcome, recoverErr, a.reconciliations, len(after.Recovery.NativeIntents), kept, bytes.Equal(foreign, lifecycleStateBytes(t, root)), bytes.Equal(beforeProfile, lifecycleProfileBytes(t, root)))
	if !fired {
		t.Fatal("TEST state-edit schedule not reached")
	}
	if !errors.Is(recoverErr, installer.ErrPlanChanged) || result.Outcome != installer.OutcomeConflict || a.reconciliations != 0 || len(after.Recovery.NativeIntents) != 1 || !kept || !bytes.Equal(foreign, lifecycleStateBytes(t, root)) || !bytes.Equal(beforeProfile, lifecycleProfileBytes(t, root)) {
		t.Fatal("initial kernel erased foreign state and acknowledged its native intent")
	}
	release, lockErr := (processlock.Lock{Path: lockPath}).Acquire(t.Context())
	if release != nil {
		_ = release()
	}
	if lockErr != nil {
		t.Fatal(lockErr)
	}
}

func lifecycleObservedPublication(t *testing.T, root, id string, stateCommitted bool) dirswap.Receipt {
	t.Helper()
	owned := filepath.Join(root, "TEST-"+id)
	staging := filepath.Join(owned, ".agentplugins-staging-fixture")
	if err := os.MkdirAll(staging, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "TEST-sentinel"), []byte(id), 0600); err != nil {
		t.Fatal(err)
	}
	manager := dirswap.Manager{JournalDir: filepath.Join(root, "state", "operations"), Fault: func(phase string) error {
		if phase == dirswap.PhaseActivated {
			return errors.New("TEST interrupted publication")
		}
		return nil
	}}
	receipt, err := manager.Apply(t.Context(), dirswap.Input{OperationID: id, ClientBindingID: "TEST-journal-binding", Sequence: 1, OwnedBase: owned, ActivePath: filepath.Join(owned, "active"), StagingPath: staging, RequireAbsent: true})
	if err == nil || receipt.Phase != dirswap.PhaseActivated {
		t.Fatalf("TEST publication: %+v %v", receipt, err)
	}
	if stateCommitted {
		state := mustLoadFacadeState(t, root)
		state.TransactionReceipts = append(state.TransactionReceipts, domain.MutationReceipt{OperationID: receipt.OperationID, ClientBindingID: receipt.ClientBindingID, Sequence: receipt.Sequence, MutationType: "directory_swap", ActivePath: receipt.ActivePath, StagingPath: receipt.StagingPath, BackupPath: receipt.BackupPath, Phase: "state_committed"})
		if err := (statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}).Save(state); err != nil {
			t.Fatal(err)
		}
	}
	return receipt
}

// Initial journal recovery fences caller errors as well as successful callbacks;
// a changed, missing or newly pending journal must never be overwritten/adopted.
func TestFacadeLifecycleInitialKernelForeignScopes(t *testing.T) {
	for _, kind := range []string{"state", "changed-journal", "missing-journal", "new-journal"} {
		for _, canceled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/canceled=%t", kind, canceled), func(t *testing.T) {
				root := localProcessRoot(t)
				runFacadeLifecycleCrash(t, root, domain.NativeIntentRegister)
				receipt := lifecycleObservedPublication(t, root, "TEST-initial", true)
				a := lifecycleAdapter(root)
				e := lifecycleFacadeEngine(t, root, a, false)
				view, err := e.Inspect(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				profile := lifecycleProfileBytes(t, root)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				lockPath := filepath.Join(root, "state", "mutation.lock")
				if err := os.Remove(lockPath); err != nil {
					t.Fatal(err)
				}
				var foreign map[string]string
				callback := reviewR2Context{Context: ctx, check: func() {
					if foreign != nil {
						return
					}
					if _, err := os.Stat(lockPath); os.IsNotExist(err) {
						return
					}
					release, lockErr := (processlock.Lock{Path: lockPath}).Acquire(context.Background())
					if release != nil {
						_ = release()
					}
					if !errors.Is(lockErr, processlock.ErrActive) {
						t.Fatalf("TEST retained lock: %v", lockErr)
					}
					journalPath := filepath.Join(root, "state", "operations", receipt.OperationID+".json")
					switch kind {
					case "state":
						state := mustLoadFacadeState(t, root)
						state.TransactionReceipts = append(state.TransactionReceipts, domain.MutationReceipt{OperationID: "TEST-foreign-terminal", ClientBindingID: "TEST-foreign-binding", Sequence: 1, MutationType: "directory_remove", ActivePath: filepath.Join(root, "TEST-foreign-target"), Phase: "committed"})
						if err := (statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}).Save(state); err != nil {
							t.Fatal(err)
						}
					case "changed-journal":
						receipt.Phase = dirswap.PhaseCommitPending
						raw, err := json.Marshal(receipt)
						if err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(journalPath, raw, 0600); err != nil {
							t.Fatal(err)
						}
					case "missing-journal":
						if err := os.Remove(journalPath); err != nil {
							t.Fatal(err)
						}
					case "new-journal":
						lifecycleObservedPublication(t, root, "TEST-new-foreign", false)
					}
					foreign = lifecycleFiles(t, root)
					if canceled {
						cancel()
					}
				}}
				result, recoverErr := e.Recover(callback, view)
				after, inspectErr := e.Inspect(t.Context())
				stateKept := reflect.DeepEqual(foreign, lifecycleFiles(t, root))
				t.Logf("outcome=%s err=%v calls=%d exactForeignFiles=%t", result.Outcome, recoverErr, a.reconciliations, stateKept)
				if foreign == nil || !errors.Is(recoverErr, installer.ErrPlanChanged) || result.Outcome != installer.OutcomeConflict || a.reconciliations != 0 || inspectErr != nil || len(after.Recovery.NativeIntents) != 1 || !stateKept || !bytes.Equal(profile, lifecycleProfileBytes(t, root)) {
					t.Fatal("initial recovery consumed late foreign authority or native intent")
				}
				if canceled && !errors.Is(recoverErr, context.Canceled) {
					t.Fatal("caller cancellation lost")
				}
			})
		}
	}
}

func TestFacadeLifecycleInitialKernelOrderedTransitions(t *testing.T) {
	root := localProcessRoot(t)
	runFacadeLifecycleCrash(t, root, domain.NativeIntentRegister)
	first := lifecycleObservedPublication(t, root, "TEST-a-commit", true)
	rollback := lifecycleObservedPublication(t, root, "TEST-b-rollback", false)
	last := lifecycleObservedPublication(t, root, "TEST-c-commit", true)
	a := lifecycleAdapter(root)
	e := lifecycleFacadeEngine(t, root, a, false)
	view, err := e.Inspect(t.Context())
	if err != nil || len(view.Recovery.Journals) != 3 {
		t.Fatalf("initial scope: %+v %v", view, err)
	}
	a.beforeReconcile = func() {
		live, err := e.Inspect(t.Context())
		if err != nil || len(live.Recovery.Journals) != 0 || len(live.Recovery.Receipts) != 0 {
			t.Fatalf("native callback preceded journal completion: %+v %v", live, err)
		}
	}
	result, err := e.Recover(t.Context(), view)
	if err != nil || result.Outcome != installer.OutcomeCompleted || a.reconciliations != 1 {
		t.Fatalf("ordered commit/rollback/commit recovery: %+v %v", result, err)
	}
	for _, receipt := range []dirswap.Receipt{first, last} {
		raw, err := os.ReadFile(filepath.Join(receipt.ActivePath, "TEST-sentinel"))
		if err != nil || string(raw) != receipt.OperationID {
			t.Fatal("committed publication lost")
		}
	}
	if _, err := os.Stat(rollback.ActivePath); !os.IsNotExist(err) {
		t.Fatalf("uncommitted publication not rolled back: %v", err)
	}
}

// Schedule a noncooperating binding edit in the registered read-only inspector's
// Context.Err boundary after the Service has durably recorded removal intent.
// Engine, store, native projector and adapter effect remain actual, unmodified.
func TestReviewR3RemoveDispatchObserverStateDrift(t *testing.T) {
	root := localProcessRoot(t)
	a := lifecycleAdapter(root)
	e := lifecycleFacadeEngine(t, root, a, true)
	installed := lifecycleInstall(t, e, root)
	req := installerRequest(root)
	req.Operation = installer.OpRemove
	req.InstallationID = installed.InstallationID
	h, err := e.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()
	beforeProfile := lifecycleProfileBytes(t, root)
	fired := false
	var foreign []byte
	lockPath := filepath.Join(root, "state", "mutation.lock")
	callback := reviewR2Context{Context: t.Context(), check: func() {
		if fired {
			return
		}
		state := mustLoadFacadeState(t, root)
		binding := independentSelectedBinding(t, state)
		if binding.PendingNativeIntent == nil || binding.PendingNativeIntent.Direction != domain.NativeIntentRemove {
			return
		}
		release, lockErr := (processlock.Lock{Path: lockPath}).Acquire(context.Background())
		if release != nil {
			_ = release()
		}
		if !errors.Is(lockErr, processlock.ErrActive) {
			t.Fatalf("dispatch lacks retained lock: %v", lockErr)
		}
		fired = true
		lifecycleChangeBinding(t, root, func(b *domain.ClientBinding) {
			found := false
			for i := range b.NativeObjects {
				if b.NativeObjects[i].Kind == "managed_package_directory" {
					b.NativeObjects[i].ManagedDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
					found = true
				}
			}
			if !found {
				t.Fatal("no recorded managed package digest")
			}
		})
		foreign = lifecycleStateBytes(t, root)
	}}
	result, applyErr := e.Apply(callback, h, confirmedDecision())
	after, inspectErr := e.Inspect(t.Context())
	persisted := independentSelectedBinding(t, mustLoadFacadeState(t, root))
	pendingKept := persisted.PendingNativeIntent != nil && persisted.NativeActivationAttempt != ""
	profileKept := bytes.Equal(beforeProfile, lifecycleProfileBytes(t, root))
	stateKept := bytes.Equal(foreign, lifecycleStateBytes(t, root))
	t.Logf("fired=%t outcome=%s error=%v removals=%d pendingRetained=%t recoveryRequired=%t inspectError=%v exactForeignState=%t profilePreserved=%t", fired, result.Outcome, applyErr, a.removals, pendingKept, after.Recovery.Required, inspectErr, stateKept, profileKept)
	if !fired {
		t.Fatal("late inspector schedule did not execute")
	}
	if !errors.Is(applyErr, installer.ErrPlanChanged) || a.removals != 0 || !pendingKept || !after.Recovery.Required || !stateKept || !profileKept {
		t.Fatal("observer's late binding drift crossed native dispatch or was erased")
	}
	release, lockErr := (processlock.Lock{Path: lockPath}).Acquire(t.Context())
	if release != nil {
		_ = release()
	}
	if lockErr != nil {
		t.Fatal(lockErr)
	}
}

// Exercise an inspector error return after a durable Service removal attempt.
// A foreign terminal receipt is outside the binding, so comparing only that
// binding would still allow it to become the dispatch baseline.
func TestFacadeLifecycleRemovalObserverCancellation(t *testing.T) {
	for _, mode := range []string{"foreign-cancel", "foreign-journal", "foreign-journal-cancel", "matching-cancel", "matching"} {
		t.Run(mode, func(t *testing.T) {
			root := localProcessRoot(t)
			a := lifecycleAdapter(root)
			e := lifecycleFacadeEngine(t, root, a, true)
			installed := lifecycleInstall(t, e, root)
			req := installerRequest(root)
			req.Operation, req.InstallationID = installer.OpRemove, installed.InstallationID
			h, err := e.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = h.Close() }()
			beforeProfile := lifecycleProfileBytes(t, root)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var approved domain.ClientBinding
			var observed []byte
			var journalPath string
			var journalBytes []byte
			callback := reviewR2Context{Context: ctx, check: func() {
				if observed != nil {
					return
				}
				state := mustLoadFacadeState(t, root)
				binding := independentSelectedBinding(t, state)
				if binding.PendingNativeIntent == nil || binding.PendingNativeIntent.Direction != domain.NativeIntentRemove {
					return
				}
				release, lockErr := (processlock.Lock{Path: filepath.Join(root, "state", "mutation.lock")}).Acquire(context.Background())
				if release != nil {
					_ = release()
				}
				if !errors.Is(lockErr, processlock.ErrActive) {
					t.Fatalf("dispatch lacks retained lock: %v", lockErr)
				}
				approved = binding
				if mode == "foreign-cancel" {
					state.TransactionReceipts = append(state.TransactionReceipts, domain.MutationReceipt{
						OperationID: "TEST-remove-foreign", ClientBindingID: "TEST-foreign-binding", Sequence: 1,
						MutationType: "directory_remove", ActivePath: filepath.Join(root, "TEST-foreign-target"), Phase: "committed",
					})
					if err := (statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}).Save(state); err != nil {
						t.Fatal(err)
					}
				}
				if strings.HasPrefix(mode, "foreign-journal") {
					journal := lifecycleObservedPublication(t, root, "TEST-dispatch-foreign", false)
					journalPath = filepath.Join(root, "state", "operations", journal.OperationID+".json")
					journalBytes, err = os.ReadFile(journalPath)
					if err != nil {
						t.Fatal(err)
					}
				}
				observed = lifecycleStateBytes(t, root)
				if strings.HasSuffix(mode, "cancel") {
					cancel()
				}
			}}
			result, applyErr := e.Apply(callback, h, confirmedDecision())
			if observed == nil {
				t.Fatal("durable removal observer did not execute")
			}
			if mode == "matching" {
				view, inspectErr := e.Inspect(t.Context())
				if applyErr != nil || result.Outcome != installer.OutcomeCompleted || a.removals != 1 || inspectErr != nil || view.Recovery.Required {
					t.Fatalf("matching recorded removal attempt did not complete: %+v, %v, %v", result, applyErr, inspectErr)
				}
				return
			}
			view, inspectErr := e.Inspect(t.Context())
			persisted := independentSelectedBinding(t, mustLoadFacadeState(t, root))
			if errors.Is(applyErr, context.Canceled) != strings.HasSuffix(mode, "cancel") || a.removals != 0 || inspectErr != nil || !view.Recovery.Required || !reflect.DeepEqual(persisted, approved) || !bytes.Equal(observed, lifecycleStateBytes(t, root)) || !bytes.Equal(beforeProfile, lifecycleProfileBytes(t, root)) {
				t.Fatalf("canceled observer lost cause or uncertainty: outcome=%s error=%v removals=%d inspect=%v", result.Outcome, applyErr, a.removals, inspectErr)
			}
			if changed := errors.Is(applyErr, installer.ErrPlanChanged); changed != strings.HasPrefix(mode, "foreign") {
				t.Fatalf("complete dispatch scope comparison missing or adopted drift: %v", applyErr)
			}
			if journalPath != "" {
				actual, readErr := os.ReadFile(journalPath)
				if readErr != nil || !bytes.Equal(actual, journalBytes) || len(view.Recovery.Journals) != 1 {
					t.Fatalf("foreign journal was changed or consumed: %v", readErr)
				}
			}
			release, lockErr := (processlock.Lock{Path: filepath.Join(root, "state", "mutation.lock")}).Acquire(t.Context())
			if release != nil {
				_ = release()
			}
			if lockErr != nil {
				t.Fatal(lockErr)
			}
		})
	}
}

// Select the existing provider's real Context.Err boundary, after facade
// registry/target/digest/data preflight has returned. Runtime frames select a
// deterministic schedule only; assertions concern public/filesystem effects.
func reviewR4AtProviderDeactivate() bool {
	pcs := make([]uintptr, 24)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.HasSuffix(frame.Function, "/providers.Activator.Deactivate") {
			return true
		}
		if !more {
			return false
		}
	}
}

func TestReviewR4RemoveProviderContextDrift(t *testing.T) {
	for _, kind := range []string{"digest", "terminal-receipt", "terminal-receipt-cancel", "matching"} {
		t.Run(kind, func(t *testing.T) {
			root := localProcessRoot(t)
			a := lifecycleAdapter(root)
			e := lifecycleFacadeEngine(t, root, a, true)
			installed := lifecycleInstall(t, e, root)
			req := installerRequest(root)
			req.Operation, req.InstallationID = installer.OpRemove, installed.InstallationID
			h, err := e.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = h.Close() }()
			profile := lifecycleProfileBytes(t, root)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			fired := false
			var foreign []byte
			lockPath := filepath.Join(root, "state", "mutation.lock")
			callback := reviewR2Context{Context: ctx, check: func() {
				if fired || !reviewR4AtProviderDeactivate() {
					return
				}
				state := mustLoadFacadeState(t, root)
				binding := independentSelectedBinding(t, state)
				if binding.PendingNativeIntent == nil || binding.PendingNativeIntent.Direction != domain.NativeIntentRemove {
					return
				}
				release, lockErr := (processlock.Lock{Path: lockPath}).Acquire(context.Background())
				if release != nil {
					_ = release()
				}
				if !errors.Is(lockErr, processlock.ErrActive) {
					t.Fatalf("existing mutation lock is not held: %v", lockErr)
				}
				fired = true
				if kind == "digest" {
					lifecycleChangeBinding(t, root, func(b *domain.ClientBinding) {
						found := false
						for i := range b.NativeObjects {
							if b.NativeObjects[i].Kind == "managed_package_directory" {
								b.NativeObjects[i].ManagedDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
								found = true
							}
						}
						if !found {
							t.Fatal("managed directory ownership is absent")
						}
					})
				} else if strings.HasPrefix(kind, "terminal-receipt") {
					state.TransactionReceipts = append(state.TransactionReceipts, domain.MutationReceipt{
						OperationID: "TEST-r4-terminal", ClientBindingID: "TEST-foreign-binding", Sequence: 1,
						MutationType: "directory_remove", ActivePath: filepath.Join(root, "TEST-foreign-target"), Phase: "committed",
					})
					if err := (statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}).Save(state); err != nil {
						t.Fatal(err)
					}
				}
				foreign = lifecycleStateBytes(t, root)
				if strings.HasSuffix(kind, "cancel") {
					cancel()
				}
			}}
			result, applyErr := e.Apply(callback, h, confirmedDecision())
			view, inspectErr := e.Inspect(t.Context())
			pending, receiptKept := false, false
			for _, installation := range mustLoadFacadeState(t, root).Installations {
				for _, b := range installation.Clients {
					if !b.SelectedDelivery.IsZero() {
						pending = b.PendingNativeIntent != nil && b.NativeActivationAttempt != ""
					}
				}
			}
			for _, receipt := range mustLoadFacadeState(t, root).TransactionReceipts {
				if receipt.OperationID == "TEST-r4-terminal" {
					receiptKept = true
				}
			}
			stateKept := bytes.Equal(foreign, lifecycleStateBytes(t, root))
			profileKept := bytes.Equal(profile, lifecycleProfileBytes(t, root))
			release, lockErr := (processlock.Lock{Path: lockPath}).Acquire(t.Context())
			if release != nil {
				_ = release()
			}
			if lockErr != nil {
				t.Fatalf("boundary lock retained: %v", lockErr)
			}
			t.Logf("providerCallback=%t outcome=%s err=%v removals=%d pending=%t recoveryRequired=%t exactForeignState=%t profilePreserved=%t foreignReceiptRetained=%t inspectErr=%v", fired, result.Outcome, applyErr, a.removals, pending, view.Recovery.Required, stateKept, profileKept, receiptKept, inspectErr)
			if !fired {
				t.Fatal("existing provider Context.Err schedule did not run")
			}
			if kind == "matching" {
				if applyErr != nil || result.Outcome != installer.OutcomeCompleted || a.removals != 1 || view.Recovery.Required || inspectErr != nil {
					t.Fatal("matching removal failed")
				}
				return
			}
			if !errors.Is(applyErr, installer.ErrPlanChanged) || a.removals != 0 || !pending || !view.Recovery.Required || !stateKept || !profileKept {
				t.Fatal("post-inspector provider callback crossed native effects or cleared pending authority")
			}
			if strings.HasSuffix(kind, "cancel") && !errors.Is(applyErr, context.Canceled) {
				t.Fatal("real caller cancellation was lost")
			}
		})
	}
}

// The real TEST native adapter is retained. These two callbacks exercise its
// supplied context and the return boundary after a completed profile effect.
type lifecycleRemovalBoundaryAdapter struct {
	*lifecycleFacadeAdapter
	onDispatch func(context.Context) error
	onReturn   func(context.Context) error
}

func (a *lifecycleRemovalBoundaryAdapter) Deactivate(ctx context.Context, env clients.Env, req domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if a.onDispatch != nil {
		if err := a.onDispatch(ctx); err != nil {
			return domain.DeactivationOutcome{}, err
		}
	}
	outcome, err := a.lifecycleFacadeAdapter.Deactivate(ctx, env, req)
	if err == nil && a.onReturn != nil {
		err = a.onReturn(ctx)
	}
	return outcome, err
}

// A second forwarded Err inside the selected adapter must close Done with a
// standard cancellation error, preserve deadline/value, and stop native effects.
func TestFacadeLifecycleRemovalContextSemantics(t *testing.T) {
	for _, mode := range []string{"matching", "matching-cancel", "late-journal"} {
		t.Run(mode, func(t *testing.T) {
			root := localProcessRoot(t)
			a := &lifecycleRemovalBoundaryAdapter{lifecycleFacadeAdapter: lifecycleAdapter(root)}
			e := lifecycleFacadeEngine(t, root, a, true)
			installed := lifecycleInstall(t, e, root)
			req := installerRequest(root)
			req.Operation, req.InstallationID = installer.OpRemove, installed.InstallationID
			h, err := e.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = h.Close() }()
			profile := lifecycleProfileBytes(t, root)
			type contextKey struct{}
			ctx, cancel := context.WithTimeout(context.WithValue(t.Context(), contextKey{}, "TEST-value"), 30*time.Second)
			defer cancel()
			deadline, _ := ctx.Deadline()
			inAdapter, fired := false, false
			var stateBytes, journalBytes []byte
			var journalPath string
			callback := reviewR2Context{Context: ctx, check: func() {
				if !inAdapter || fired {
					return
				}
				fired = true
				if mode == "matching-cancel" {
					cancel()
				}
				if mode == "late-journal" {
					journal := lifecycleObservedPublication(t, root, "TEST-adapter-err", false)
					journalPath = filepath.Join(root, "state", "operations", journal.OperationID+".json")
					journalBytes, err = os.ReadFile(journalPath)
					if err != nil {
						t.Fatal(err)
					}
				}
				stateBytes = lifecycleStateBytes(t, root)
			}}
			var retained context.Context
			a.onDispatch = func(dispatch context.Context) error {
				retained, inAdapter = dispatch, true
				gotDeadline, ok := dispatch.Deadline()
				if !ok || !gotDeadline.Equal(deadline) || dispatch.Value(contextKey{}) != "TEST-value" || dispatch.Done() == nil {
					t.Fatal("dispatch context lost deadline, values or cancellation channel")
				}
				err := dispatch.Err()
				if mode == "matching" {
					if err != nil || context.Cause(dispatch) != nil {
						t.Fatalf("matching context canceled: %v", err)
					}
					select {
					case <-dispatch.Done():
						t.Fatal("matching context Done is closed before effect")
					default:
					}
					return nil
				}
				if err != context.Canceled || errors.Is(err, installer.ErrPlanChanged) {
					t.Fatalf("Err is not standard cancellation: %v", err)
				}
				select {
				case <-dispatch.Done():
				default:
					t.Fatal("Err cancellation is inconsistent with Done")
				}
				if errors.Is(context.Cause(dispatch), installer.ErrPlanChanged) != (mode == "late-journal") {
					t.Fatalf("dispatch cause lost precise refusal: %v", context.Cause(dispatch))
				}
				return err
			}
			result, applyErr := e.Apply(callback, h, confirmedDecision())
			view, inspectErr := e.Inspect(t.Context())
			if !fired || inspectErr != nil || retained == nil {
				t.Fatalf("adapter context boundary missing: fired=%t inspect=%v", fired, inspectErr)
			}
			select {
			case <-retained.Done():
			default:
				t.Fatal("owned dispatch child was not cleaned up")
			}
			if mode == "matching" {
				if applyErr != nil || result.Outcome != installer.OutcomeCompleted || a.removals != 1 || view.Recovery.Required {
					t.Fatalf("matching native removal: %+v %v", result, applyErr)
				}
				return
			}
			if !errors.Is(applyErr, context.Canceled) || errors.Is(applyErr, installer.ErrPlanChanged) != (mode == "late-journal") || a.removals != 0 || !view.Recovery.Required || !bytes.Equal(stateBytes, lifecycleStateBytes(t, root)) || !bytes.Equal(profile, lifecycleProfileBytes(t, root)) {
				t.Fatalf("context refusal lost state/cause or crossed effects: %+v %v removals=%d", result, applyErr, a.removals)
			}
			if journalPath != "" {
				raw, readErr := os.ReadFile(journalPath)
				if readErr != nil || !bytes.Equal(raw, journalBytes) || len(view.Recovery.Journals) != 1 {
					t.Fatal("context refusal consumed foreign journal")
				}
			}
		})
	}
}

// The native effect may complete before a foreign edit or outcome error. A
// complete return-scope fence must preserve that uncertainty before Service ack.
func TestFacadeLifecycleRemovalReturnScope(t *testing.T) {
	for _, mode := range []string{"foreign-success", "foreign-error", "foreign-cancel", "journal-success", "matching-error", "matching-success"} {
		t.Run(mode, func(t *testing.T) {
			root := localProcessRoot(t)
			a := &lifecycleRemovalBoundaryAdapter{lifecycleFacadeAdapter: lifecycleAdapter(root)}
			e := lifecycleFacadeEngine(t, root, a, true)
			installed := lifecycleInstall(t, e, root)
			req := installerRequest(root)
			req.Operation, req.InstallationID = installer.OpRemove, installed.InstallationID
			h, err := e.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = h.Close() }()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			unknown := errors.New("TEST-native-outcome-unknown")
			var stateBytes, profileBytes, journalBytes []byte
			var journalPath string
			a.onReturn = func(dispatch context.Context) error {
				state := mustLoadFacadeState(t, root)
				binding := independentSelectedBinding(t, state)
				if binding.PendingNativeIntent == nil || a.removals != 1 {
					t.Fatal("real effect did not retain its pending Remove attempt")
				}
				if strings.HasPrefix(mode, "foreign") {
					state.TransactionReceipts = append(state.TransactionReceipts, domain.MutationReceipt{
						OperationID: "TEST-return-foreign", ClientBindingID: "TEST-foreign-binding", Sequence: 1,
						MutationType: "directory_remove", ActivePath: filepath.Join(root, "TEST-foreign-target"), Phase: "committed",
					})
					if err := (statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}).Save(state); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "journal-success" {
					journal := lifecycleObservedPublication(t, root, "TEST-return-foreign", false)
					journalPath = filepath.Join(root, "state", "operations", journal.OperationID+".json")
					journalBytes, err = os.ReadFile(journalPath)
					if err != nil {
						t.Fatal(err)
					}
				}
				stateBytes, profileBytes = lifecycleStateBytes(t, root), lifecycleProfileBytes(t, root)
				if strings.HasSuffix(mode, "error") {
					return unknown
				}
				if mode == "foreign-cancel" {
					cancel()
					return dispatch.Err()
				}
				return nil
			}
			result, applyErr := e.Apply(ctx, h, confirmedDecision())
			view, inspectErr := e.Inspect(t.Context())
			t.Logf("mode=%s outcome=%s err=%v removals=%d recoveryRequired=%t", mode, result.Outcome, applyErr, a.removals, view.Recovery.Required)
			if stateBytes == nil || inspectErr != nil || a.removals != 1 {
				t.Fatalf("native return boundary missing: %v", inspectErr)
			}
			if mode == "matching-success" {
				if applyErr != nil || result.Outcome != installer.OutcomeCompleted || view.Recovery.Required {
					t.Fatalf("legitimate completion refused: %+v %v", result, applyErr)
				}
				return
			}
			if !view.Recovery.Required || !bytes.Equal(stateBytes, lifecycleStateBytes(t, root)) || !bytes.Equal(profileBytes, lifecycleProfileBytes(t, root)) {
				t.Fatal("return fence overwrote foreign state or acknowledged uncertain native outcome")
			}
			if errors.Is(applyErr, installer.ErrPlanChanged) != (mode != "matching-error") || errors.Is(applyErr, unknown) != strings.HasSuffix(mode, "error") || errors.Is(applyErr, context.Canceled) != (mode == "foreign-cancel") {
				t.Fatalf("complete return scope lost original error or drift: %v", applyErr)
			}
			if journalPath != "" {
				raw, readErr := os.ReadFile(journalPath)
				if readErr != nil || !bytes.Equal(raw, journalBytes) || len(view.Recovery.Journals) != 1 {
					t.Fatal("return fence consumed foreign journal")
				}
			}
		})
	}
}

// Forward the exact standard parent value. Frames select its existing child
// detachment callback; they are not the behavior being asserted.
type reviewR5CleanupContext struct {
	context.Context
	onDetach func()
}

func (c reviewR5CleanupContext) Value(key any) any {
	pcs := make([]uintptr, 48)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		frame, more := frames.Next()
		if frame.Function == "context.removeChild" {
			c.onDetach()
			break
		}
		if !more {
			break
		}
	}
	return c.Context.Value(key)
}

func TestIndependentR5RemovalCleanupScope(t *testing.T) {
	for _, mode := range []string{"foreign-success", "foreign-error", "matching-success"} {
		t.Run(mode, func(t *testing.T) {
			root := localProcessRoot(t)
			a := &lifecycleRemovalBoundaryAdapter{lifecycleFacadeAdapter: lifecycleAdapter(root)}
			e := lifecycleFacadeEngine(t, root, a, true)
			installed := lifecycleInstall(t, e, root)
			req := installerRequest(root)
			req.Operation, req.InstallationID = installer.OpRemove, installed.InstallationID
			h, err := e.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = h.Close() }()
			parent, cancel := context.WithCancel(t.Context())
			defer cancel()
			unknown := errors.New("TEST-r5-native-outcome-unknown")
			var retained context.Context
			a.onReturn = func(dispatch context.Context) error {
				retained = dispatch
				if mode == "foreign-error" {
					return unknown
				}
				return nil
			}
			fired := false
			var foreignState, nativeProfile []byte
			lockPath := filepath.Join(root, "state", "mutation.lock")
			callback := reviewR5CleanupContext{Context: parent, onDetach: func() {
				if fired || retained == nil {
					return
				}
				state := mustLoadFacadeState(t, root)
				binding := independentSelectedBinding(t, state)
				if binding.PendingNativeIntent == nil || binding.PendingNativeIntent.Direction != domain.NativeIntentRemove || a.removals != 1 {
					t.Fatal("TEST cleanup must follow actual profile removal with durable intent")
				}
				release, lockErr := (processlock.Lock{Path: lockPath}).Acquire(context.Background())
				if release != nil {
					_ = release()
				}
				if !errors.Is(lockErr, processlock.ErrActive) {
					t.Fatalf("existing mutation lock is not retained at child cleanup: %v", lockErr)
				}
				fired = true
				if mode != "matching-success" {
					state.TransactionReceipts = append(state.TransactionReceipts, domain.MutationReceipt{
						OperationID: "TEST-r5-cleanup-foreign", ClientBindingID: "TEST-foreign-binding", Sequence: 1,
						MutationType: "directory_remove", ActivePath: filepath.Join(root, "TEST-foreign-target"), Phase: "committed",
					})
					if err := (statev2.Store{Path: filepath.Join(root, "state", "state-v2.json")}).Save(state); err != nil {
						t.Fatal(err)
					}
				}
				foreignState, nativeProfile = lifecycleStateBytes(t, root), lifecycleProfileBytes(t, root)
			}}
			result, applyErr := e.Apply(callback, h, confirmedDecision())
			view, inspectErr := e.Inspect(t.Context())
			pending := len(view.Recovery.NativeIntents) == 1
			stateKept := bytes.Equal(foreignState, lifecycleStateBytes(t, root))
			profileKept := bytes.Equal(nativeProfile, lifecycleProfileBytes(t, root))
			t.Logf("cleanupCallback=%t outcome=%s err=%v removals=%d pending=%t recoveryRequired=%t exactForeignState=%t completedNativeProfilePreserved=%t inspectErr=%v", fired, result.Outcome, applyErr, a.removals, pending, view.Recovery.Required, stateKept, profileKept, inspectErr)
			if !fired || retained == nil || inspectErr != nil || a.removals != 1 || !profileKept {
				t.Fatal("TEST actual cleanup/complete native effect boundary did not run")
			}
			select {
			case <-retained.Done():
			default:
				t.Fatal("owned child remains live after dispatch return")
			}
			if mode == "matching-success" {
				if applyErr != nil || result.Outcome != installer.OutcomeCompleted || view.Recovery.Required {
					t.Fatal("legitimate matching removal did not complete")
				}
				return
			}
			if !errors.Is(applyErr, installer.ErrPlanChanged) || errors.Is(applyErr, unknown) != (mode == "foreign-error") || !pending || !view.Recovery.Required || !stateKept || result.Outcome == installer.OutcomeCompleted {
				t.Fatal("child cleanup crossed the final scope fence or dropped original error")
			}
		})
	}
}
