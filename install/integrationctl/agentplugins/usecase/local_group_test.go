package usecase_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type testGroupLocalAdapter struct {
	testEffectLocalAdapter
	changeSecond                           func(*domain.DeliveryPlan, *domain.LocalDeliveryFacts)
	plans, activations, previews, removals int
}

func (a *testGroupLocalAdapter) RefinePlan(ctx context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	selected := testSelectedMCPLocalAdapter{testEffectLocalAdapter: a.testEffectLocalAdapter, selected: true}
	if err := selected.RefinePlan(ctx, in, plan); err != nil {
		return err
	}
	facts, _ := plan.SelectedDelivery.LocalFacts()
	a.plans++
	if a.plans == 2 && a.changeSecond != nil {
		a.changeSecond(plan, &facts)
	}
	return clients.SelectLocalDelivery(plan, facts)
}

func (a *testGroupLocalAdapter) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if !req.VerifyOnly {
		a.activations++
	}
	return a.testEffectLocalAdapter.Activate(ctx, env, req)
}

func (a *testGroupLocalAdapter) Deactivate(ctx context.Context, env clients.Env, req domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if req.Confirmed {
		a.removals++
		return a.testEffectLocalAdapter.Deactivate(ctx, env, req)
	}
	a.previews++
	state, err := (statev2.Store{Path: filepath.Join(a.root, "state", "state-v2.json")}).Load()
	if err != nil {
		return domain.DeactivationOutcome{}, err
	}
	binding := onlyLocalBinding(state)
	if !reflect.DeepEqual(req.SelectedDelivery, binding.SelectedDelivery) || req.RemoveOwnedEntry != binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) {
		return domain.DeactivationOutcome{}, fmt.Errorf("TEST preview lost persisted delivery/ownership")
	}
	if binding.PendingNativeIntent != nil || binding.NativeActivationAttempt != "" {
		return domain.DeactivationOutcome{}, fmt.Errorf("TEST preview created a native attempt")
	}
	return domain.DeactivationOutcome{ArtifactRemovalAllowed: true}, nil
}

// Red: first-install coalescing overwrites a different selected profile, tuple,
// component set, bool authority or digest. Identical inputs must still share
// one physical commit and activation rather than conflict with their own write.
func TestLocalGroupFrozenSelection(t *testing.T) {
	changes := map[string]func(*domain.DeliveryPlan, *domain.LocalDeliveryFacts){
		"identical": nil,
		"profile": func(_ *domain.DeliveryPlan, f *domain.LocalDeliveryFacts) {
			f.ProfileRoot += "-other"
			f.SettingsPath = filepath.Join(f.ProfileRoot, "settings.json")
		},
		"settings-path": func(_ *domain.DeliveryPlan, f *domain.LocalDeliveryFacts) {
			f.SettingsPath = filepath.Join(f.ProfileRoot, "other-settings.json")
		},
		"physical-profile":  func(_ *domain.DeliveryPlan, f *domain.LocalDeliveryFacts) { f.ProfileIdentity += "-other" },
		"physical-settings": func(_ *domain.DeliveryPlan, f *domain.LocalDeliveryFacts) { f.SettingsIdentity += "-other" },
		"shell": func(_ *domain.DeliveryPlan, f *domain.LocalDeliveryFacts) {
			f.Tuple.TargetShell = "TEST-other-qualified-shell"
		},
		"tuple":       func(_ *domain.DeliveryPlan, f *domain.LocalDeliveryFacts) { f.Tuple.CopilotVersion += "-other" },
		"native-stop": func(_ *domain.DeliveryPlan, f *domain.LocalDeliveryFacts) { f.NativeStop = false },
		"desired-false": func(_ *domain.DeliveryPlan, f *domain.LocalDeliveryFacts) {
			disabled := false
			f.Registration.DesiredValue = &disabled
		},
		"previous-true": func(_ *domain.DeliveryPlan, f *domain.LocalDeliveryFacts) {
			enabled := true
			f.Registration.PreviousValue = &enabled
		},
		"object": func(_ *domain.DeliveryPlan, f *domain.LocalDeliveryFacts) { f.Registration.ObjectID += "-other" },
		"projection-digest": func(_ *domain.DeliveryPlan, f *domain.LocalDeliveryFacts) {
			f.ProjectionDigest = "sha256:" + strings.Repeat("a", 64)
		},
		"mcp": func(p *domain.DeliveryPlan, f *domain.LocalDeliveryFacts) {
			f.MCPServers = nil
			for i := range p.Components {
				if p.Components[i].Kind == domain.ComponentMCPServer {
					p.Components[i].Support = domain.SupportUnsupported
				}
			}
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			root := localProcessRoot(t)
			source := []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"notify":{"type":"streamable-http","url":"https://example.invalid/TEST-never-contacted"}}}`)
			if err := os.WriteFile(filepath.Join(root, "package", "mcp.json"), source, 0600); err != nil {
				t.Fatal(err)
			}
			adapter := &testGroupLocalAdapter{testEffectLocalAdapter: testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root}, changeSecond: change}
			service := localGroupService(t, root, adapter)
			first := localGroupInput(t, root)
			profileBefore, err := os.ReadFile(filepath.Join(root, "profile", "settings.json"))
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.AddGroup(t.Context(), usecase.GroupInput{Targets: []usecase.AddInput{first, first}, Confirmed: true, OperationGroupID: "TEST-frozen-group"})
			if name == "identical" {
				if err != nil {
					t.Fatal(err)
				}
				state, err := service.StateStore.Load()
				if err != nil {
					t.Fatal(err)
				}
				binding := localOnlyBinding(t, state)
				if adapter.activations != 1 || len(result.Targets) != 2 || result.Phase != usecase.GroupPhaseCompleted || !reflect.DeepEqual(result.Targets[0].Plan.SelectedDelivery, result.Targets[1].Plan.SelectedDelivery) || !binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) {
					t.Fatal("identical selections did not coalesce into one owned activation")
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "targets select different") || adapter.activations != 0 {
				t.Fatalf("different frozen selections reached effects/coalesced: %v, activations=%d", err, adapter.activations)
			}
			state, loadErr := service.StateStore.Load()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			profileAfter, readErr := os.ReadFile(filepath.Join(root, "profile", "settings.json"))
			if readErr != nil || string(profileAfter) != string(profileBefore) || len(state.Installations) != 0 {
				t.Fatal("selection refusal changed installed/profile state")
			}
		})
	}
}

func localGroupService(t *testing.T, root string, adapter clients.Adapter) usecase.Service {
	t.Helper()
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	service := localRecoveryService(t, root)
	p := planner.Planner{Registry: registry, Paths: pathpolicy.Policy{}, ManagedRoot: filepath.Join(root, "state", "managed")}
	kernel := nativeconfig.New()
	service.Planner, service.Targets = p, p
	service.Activator = providers.Activator{Registry: registry, NativeConfig: &kernel}
	return service
}

func localGroupInput(t *testing.T, root string) usecase.AddInput {
	t.Helper()
	source := domain.SourceIdentity{CanonicalSource: filepath.Join(root, "package"), RequestedSource: filepath.Join(root, "package")}
	snapshot, err := (packagedigest.Builder{TempRoot: t.TempDir()}).Snapshot(t.Context(), source.CanonicalSource, source)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = packagedigest.Remove(snapshot) })
	schemas, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := (loader.Loader{Registry: schemas}).LoadSnapshot(t.Context(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return usecase.AddInput{Envelope: envelope, Client: domain.DetectedClient{ClientID: domain.ClientVSCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(root, "profile")}, Scope: domain.ScopeUser}
}

// Red: the group preview loses persisted selection/owned-entry authority, or
// writes/acquires the profile lock before confirmation. Hold the real writer
// lock while previewing, then remove only the owned selector after confirmation.
func TestLocalGroupRemovalPreviewAndOwnedSelector(t *testing.T) {
	root := localProcessRoot(t)
	adapter := &testGroupLocalAdapter{testEffectLocalAdapter: testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root}}
	engine := localTestEngine(t, filepath.Join(root, "state"), adapter)
	prepared, err := engine.Prepare(t.Context(), installerRequest(root))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = prepared.Close() }()
	installed, err := engine.Apply(t.Context(), prepared, confirmedDecision())
	if err != nil {
		t.Fatal(err)
	}
	service := localGroupService(t, root, adapter)
	state, err := service.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	binding := localOnlyBinding(t, state)
	facts := mustFacts(t, binding)
	doc := readLocalDocument(t, facts.SettingsPath)
	foreign := filepath.Join(root, "TEST-foreign-plugin")
	doc["late.foreign"], doc["chat.pluginLocations"].(map[string]any)[foreign] = "TEST-preserve", false
	writeLocalDocument(t, facts.SettingsPath, doc)
	profileBefore, err := os.ReadFile(facts.SettingsPath)
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(root, "state", "state-v2.json")
	stateBefore, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	input := usecase.RemoveGroupInput{Selector: installed.InstallationID, Targets: []usecase.RemoveInput{{Client: domain.DetectedClient{ClientID: domain.ClientVSCode, ConfigRoot: facts.ProfileRoot}, Scope: domain.ScopeUser, SelectedDelivery: binding.SelectedDelivery}}, OperationGroupID: "TEST-selected-remove"}
	file, err := nativeconfig.New().BeginExactFile(facts.SettingsPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	for _, owned := range []bool{false, true} {
		previewBinding := binding
		if !owned {
			previewBinding.NativeObjects = []domain.NativeObjectOwnership{}
			for _, object := range binding.NativeObjects {
				if object.ObjectID != facts.Registration.ObjectID {
					previewBinding.NativeObjects = append(previewBinding.NativeObjects, object)
				}
			}
		}
		state.Installations[0].Clients[binding.ClientBindingID] = previewBinding
		if err := service.StateStore.Save(state); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatal(err)
		}
		for _, dryRun := range []bool{true, false} {
			input.DryRun = dryRun
			preview, err := service.RemoveGroup(ctx, input)
			if err != nil || preview.Mutated || preview.Phase != usecase.GroupPhasePlanned {
				t.Fatalf("read-only preview failed with held profile lock: %+v %v", preview, err)
			}
		}
		after, err := os.ReadFile(statePath)
		if err != nil || string(before) != string(after) {
			t.Fatal("preview changed persisted ownership/native intent")
		}
	}
	profileAfter, err := os.ReadFile(facts.SettingsPath)
	if err != nil {
		t.Fatal(err)
	}
	stateAfter, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if adapter.previews != 4 || adapter.removals != 0 || string(profileBefore) != string(profileAfter) || string(stateBefore) != string(stateAfter) {
		t.Fatal("preview changed native/state bytes or attempted removal")
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	input.DryRun, input.Confirmed = false, true
	result, err := service.RemoveGroup(t.Context(), input)
	if err != nil || !result.Mutated || result.Phase != usecase.GroupPhaseCompleted || adapter.removals != 1 {
		t.Fatalf("confirmed group removal failed: %+v %v", result, err)
	}
	after := readLocalDocument(t, facts.SettingsPath)
	delete(doc["chat.pluginLocations"].(map[string]any), facts.Registration.Selector)
	if !reflect.DeepEqual(doc, after) {
		t.Fatal("removal changed foreign settings or retained owned selector")
	}
	state, err = service.StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 {
		t.Fatal("removal retained the selected binding/native intent")
	}
	if _, err := os.Stat(binding.TargetLocator); !os.IsNotExist(err) {
		t.Fatalf("removal retained the managed projection: %v", err)
	}
}
