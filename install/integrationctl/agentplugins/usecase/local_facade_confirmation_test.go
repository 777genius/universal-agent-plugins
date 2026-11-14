package usecase_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type facadeLocalAdapter struct {
	testEffectLocalAdapter
	activations, discoveries  int
	chooseMCP                 bool
	unselectSkill, historical bool
	change                    func(*domain.LocalDeliveryFacts)
}

func (a *facadeLocalAdapter) RefinePlan(ctx context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	components := plan.Components
	plan.Components = nil
	if err := a.testLocalAdapter.RefinePlan(ctx, in, plan); err != nil {
		return err
	}
	plan.Components = components
	facts, _ := plan.SelectedDelivery.LocalFacts()
	for i := range plan.Components {
		c := &plan.Components[i]
		if c.Kind == domain.ComponentMCPServer {
			c.Support = domain.SupportUnsupported
			if a.chooseMCP {
				c.Support = domain.SupportNative
				facts.MCPServers = append(facts.MCPServers, c.Name)
			}
		}
		if c.Kind == domain.ComponentSkill && a.unselectSkill {
			c.Support = domain.SupportUnsupported
		}
		if c.Kind == domain.ComponentSkill && c.Support != domain.SupportUnsupported {
			facts.Skills = append(facts.Skills, c.Name)
		}
	}
	if a.change != nil {
		a.change(&facts)
	}
	if a.historical {
		plan.SelectedDelivery = domain.SelectedDelivery{}
		return nil
	}
	return clients.SelectLocalDelivery(plan, facts)
}
func (a *facadeLocalAdapter) DetectSurfaces(clients.Host) clients.Detection {
	a.discoveries++
	return clients.Detection{}
}
func (a *facadeLocalAdapter) Activate(_ context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	a.activations++
	if req.VerifyOnly {
		return domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationPackageValid}, nil
	}
	state, err := (statev2.Store{Path: filepath.Join(a.root, "state", "state-v2.json")}).Load()
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	facts, _ := req.Plan.SelectedDelivery.LocalFacts()
	for _, binding := range state.Installations[0].Clients {
		if binding.ClientID != string(req.Plan.ClientID) || binding.TargetLocator != req.Plan.ActivePath {
			continue
		}
		if binding.PendingNativeIntent == nil || binding.PendingNativeIntent.Direction != domain.NativeIntentRegister || !reflect.DeepEqual(binding.SelectedDelivery, req.Plan.SelectedDelivery) {
			return domain.ActivationOutcome{}, fmt.Errorf("TEST profile write lacks matching durable authority")
		}
		if err := patchLocalEntry(env.NativeConfig, facts, false, req.Plan.SelectedDelivery.OwnsProfileEntry(req.PreviousNativeObjects)); err != nil {
			return domain.ActivationOutcome{}, err
		}
		return domain.ActivationOutcome{Activation: domain.ActivationActive, Authentication: domain.AuthenticationNotRequired, Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled, NativeEffect: domain.NativeEffectCommitted, NativeObjects: []domain.NativeObjectOwnership{facts.Registration.Ownership(facts.SettingsPath)}}, nil
	}
	return domain.ActivationOutcome{}, fmt.Errorf("TEST binding is absent")
}

// Red on the accepted base: public Apply commits the changed shell and writes
// the TEST profile after Prepare. Confirmation must fence that first install.
// These are filesystem/public-contract fixtures, not native VS Code qualification.
func TestLocalFacadeChangedSelection(t *testing.T) {
	root := localProcessRoot(t)
	a := &facadeLocalAdapter{testEffectLocalAdapter: testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root}}
	runner := &localBoundaryRunner{testHome: filepath.Join(root, "TEST-home")}
	eng := localEngineWithRunner(t, root, a, runner)
	h, err := eng.Prepare(t.Context(), installerRequest(root))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()
	stateBefore, err := localRecoveryService(t, root).StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(root, "profile", "settings.json")
	before, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	a.targetShell = "TEST-changed-after-confirmation"
	result, err := eng.Apply(t.Context(), h, confirmedDecision())
	if !errors.Is(err, installer.ErrPlanChanged) || result.Outcome != installer.OutcomeConflict || result.Mutated {
		t.Fatalf("changed selection accepted: %+v %v", result, err)
	}
	stateAfter, err := localRecoveryService(t, root).StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stateBefore, stateAfter) || string(before) != string(after) || a.activations != 0 || a.discoveries != 0 || runner.calls != 0 {
		t.Fatalf("refusal changed state/profile or invoked native/discovery: activation=%d discovery=%d commands=%d", a.activations, a.discoveries, runner.calls)
	}
	for _, path := range []string{"mutation.lock", "managed", "operations", "plugin-data"} {
		if _, err := os.Lstat(filepath.Join(root, "state", path)); !os.IsNotExist(err) {
			t.Fatalf("confirmation reached effect %s: %v", path, err)
		}
	}
}

// Red: persisted projection sealing can hide a new adapter revision. Confirmation
// must compare the complete raw frozen value before that sealing, including the
// two digests that SameSelection intentionally ignores.
func TestLocalFacadeRevisionDrift(t *testing.T) {
	for _, field := range []string{"canonical", "projection", "recorded-projection", "recorded-after-preflight"} {
		t.Run(field, func(t *testing.T) {
			root := localProcessRoot(t)
			a := newFacadeAdapter(root)
			var stageHook func()
			eng := facadeEngine(t, root, a, installer.Config{Progress: func(event installer.ProgressEvent) {
				if event.Phase == installer.ProgressStage && stageHook != nil {
					stageHook()
				}
			}})
			h, err := eng.Prepare(t.Context(), installerRequest(root))
			if err != nil {
				t.Fatal(err)
			}
			installed, err := eng.Apply(t.Context(), h, confirmedDecision())
			if err != nil {
				t.Fatal(err)
			}
			_ = h.Close()
			req := installerRequest(root)
			req.Operation, req.InstallationID = installer.OpRefreshProjection, installed.InstallationID
			h, err = eng.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = h.Close() }()
			statePath := filepath.Join(root, "state", "state-v2.json")
			before, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			profile, err := os.ReadFile(filepath.Join(root, "profile", "settings.json"))
			if err != nil {
				t.Fatal(err)
			}
			if field == "recorded-projection" || field == "recorded-after-preflight" {
				mutate := func() {
					state := mustLoadFacadeState(t, root)
					for id, binding := range state.Installations[0].Clients {
						binding.SelectedDelivery, err = binding.SelectedDelivery.WithProjectionDigest("sha256:" + strings.Repeat("c", 64))
						if err != nil {
							t.Fatal(err)
						}
						state.Installations[0].Clients[id] = binding
					}
					if err := localRecoveryService(t, root).StateStore.Save(state); err != nil {
						t.Fatal(err)
					}
					before, err = os.ReadFile(statePath)
					if err != nil {
						t.Fatal(err)
					}
				}
				if field == "recorded-after-preflight" {
					stageHook = mutate
				} else {
					mutate()
				}
			} else {
				a.change = func(f *domain.LocalDeliveryFacts) {
					if field == "canonical" {
						f.CanonicalDigest = "sha256:" + strings.Repeat("a", 64)
					} else {
						f.ProjectionDigest = "sha256:" + strings.Repeat("b", 64)
					}
				}
			}
			result, err := eng.Apply(t.Context(), h, confirmedDecision())
			if !errors.Is(err, installer.ErrPlanChanged) || result.Mutated {
				t.Fatalf("revision drift admitted: %+v %v", result, err)
			}
			after, _ := os.ReadFile(statePath)
			profileAfter, _ := os.ReadFile(filepath.Join(root, "profile", "settings.json"))
			if string(before) != string(after) || string(profile) != string(profileAfter) || a.activations != 1 {
				t.Fatal("revision refusal changed committed files")
			}
		})
	}
}

// Red: the old DTO/handoff drops selection. Review copies and diagnostic JSON
// must not change the handle, and the successful callback must see the actual
// committed selection even if its caller changes the adapter immediately after.
func TestLocalFacadeMatchingCommittedFacts(t *testing.T) {
	root := localProcessRoot(t)
	facadeComponents(t, root)
	a := newFacadeAdapter(root)
	a.chooseMCP = true
	a.change = func(f *domain.LocalDeliveryFacts) {
		disabled := false
		f.Registration.DesiredValue = &disabled
		f.Registration.PreviousValue = &disabled
	}
	var committed installer.BindingFacts
	eng := facadeEngine(t, root, a, installer.Config{OnCommittedBinding: func(_ context.Context, f installer.BindingFacts) error {
		state, err := localRecoveryService(t, root).StateStore.Load()
		if err != nil {
			return err
		}
		binding := localOnlyBinding(t, state)
		if !reflect.DeepEqual(f.SelectedDelivery, binding.SelectedDelivery) || f.BindingID != binding.ClientBindingID || f.DataRoot == "" {
			return fmt.Errorf("handoff lost committed facts")
		}
		committed = f
		a.targetShell = "TEST-ambient-after-commit"
		return nil
	}})
	req := installerRequest(root)
	digest, err := eng.LocalPackageTreeDigest(t.Context(), req.PackageRoot)
	if err != nil {
		t.Fatal(err)
	}
	req.Assessment = &installer.Assessment{TreeDigest: digest, Outcome: installer.AssessmentAllow}
	req.RequiredComponents = []string{"mcp", "skills"}
	h, err := eng.Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()
	plan := h.Plan()
	frozen := plan.SelectedDelivery
	if frozen.IsZero() || !reflect.DeepEqual(frozen, plan.Delivery.SelectedDelivery) {
		t.Fatal("review omitted typed selection")
	}
	facts, _ := frozen.LocalFacts()
	if *facts.Registration.DesiredValue || *facts.Registration.PreviousValue || len(facts.MCPServers) != 1 || len(facts.Skills) != 1 {
		t.Fatal("review omitted false or components")
	}
	*facts.Registration.DesiredValue = true
	*facts.Registration.PreviousValue = true
	facts.MCPServers[0] = "caller-mcp"
	facts.Skills[0] = "caller-skill"
	plan.SelectedDelivery = domain.SelectedDelivery{}
	plan.Delivery.SelectedDelivery = domain.SelectedDelivery{}
	req.ClientConfigRoot = filepath.Join(root, "TEST-wrong-profile")
	req.RequiredComponents[0] = "caller-required"
	req.Assessment.Outcome = installer.AssessmentBlock
	diagnostic, _ := json.Marshal(h.Plan())
	var decoded installer.Plan
	if err := json.Unmarshal(diagnostic, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.SelectedDelivery.IsZero() || !decoded.Delivery.SelectedDelivery.IsZero() || bytes.Contains(diagnostic, []byte("TEST-process-contract")) {
		t.Fatal("diagnostic JSON granted selection authority")
	}
	if !reflect.DeepEqual(h.Plan().SelectedDelivery, frozen) {
		t.Fatal("review mutation reached handle")
	}
	result, err := eng.Apply(t.Context(), h, confirmedDecision())
	if err != nil || result.Outcome != installer.OutcomeCompleted {
		t.Fatalf("matching path refused: %+v %v", result, err)
	}
	binding := localOnlyBinding(t, mustLoadFacadeState(t, root))
	selected := binding.SelectedDelivery
	expected, err := frozen.WithProjectionDigest(mustFacts(t, binding).ProjectionDigest)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selected, expected) || !reflect.DeepEqual(committed.SelectedDelivery, selected) || !reflect.DeepEqual(result.Binding.SelectedDelivery, selected) || !reflect.DeepEqual(result.Client.SelectedDelivery, selected) || result.Delivery == nil || !reflect.DeepEqual(result.Delivery.SelectedDelivery, selected) || a.discoveries != 0 {
		t.Fatal("committed selection did not survive public handoff")
	}
	if err := localRecoveryService(t, root).Stager.Verify(t.Context(), binding.TargetLocator, mustFacts(t, binding).ProjectionDigest); err != nil {
		t.Fatal(err)
	}
	doc := readLocalDocument(t, filepath.Join(root, "profile", "settings.json"))
	if doc["foreign.setting"] != "original" || doc["chat.pluginLocations"].(map[string]any)[binding.TargetLocator] != false {
		t.Fatal("matching install lost foreign/disabled intent")
	}
}

func newFacadeAdapter(root string) *facadeLocalAdapter {
	return &facadeLocalAdapter{testEffectLocalAdapter: testEffectLocalAdapter{testLocalAdapter: testLocalAdapter{Adapter: vscode.New()}, root: root}}
}
func facadeEngine(t *testing.T, root string, a clients.Adapter, cfg installer.Config, others ...clients.Adapter) *installer.Engine {
	t.Helper()
	registry, err := clients.NewRegistry(append([]clients.Adapter{a}, others...)...)
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cfg.StateRoot, cfg.Registry, cfg.HelperExecutable = filepath.Join(root, "state"), registry, helper
	cfg.TrustedLocalPackages = true
	eng, err := installer.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return eng
}
func facadeComponents(t *testing.T, root string) {
	t.Helper()
	writeFile := func(path, body string) {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(filepath.Join(root, "package", "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"notify":{"type":"streamable-http","url":"https://example.invalid/TEST-never-contacted"}}}`)
	skill := filepath.Join(root, "package", "skills", "test-skill")
	if err := os.MkdirAll(skill, 0700); err != nil {
		t.Fatal(err)
	}
	writeFile(filepath.Join(skill, "SKILL.md"), "---\nname: test-skill\ndescription: TEST fixture\n---\nTEST content\n")
}
func mustLoadFacadeState(t *testing.T, root string) domain.StateFileV2 {
	t.Helper()
	state, err := localRecoveryService(t, root).StateStore.Load()
	if err != nil {
		t.Fatal(err)
	}
	return state
}

// Red: the public group facade excludes an injected Local adapter; once admitted,
// its old first-install confirmation skips every target when state is absent.
// A changed second target must stop the entire operation before staging either.
func TestLocalFacadeGroupConfirmation(t *testing.T) {
	for _, changed := range []bool{true, false} {
		t.Run(fmt.Sprintf("changed=%t", changed), func(t *testing.T) {
			root := localProcessRoot(t)
			a := newFacadeAdapter(root)
			var handed []installer.BindingFacts
			eng := facadeEngine(t, root, a, installer.Config{OnCommittedBinding: func(_ context.Context, f installer.BindingFacts) error {
				state := mustLoadFacadeState(t, root)
				for _, b := range state.Installations[0].Clients {
					if b.ClientBindingID == f.BindingID && reflect.DeepEqual(b.SelectedDelivery, f.SelectedDelivery) {
						handed = append(handed, f)
						return nil
					}
				}
				return fmt.Errorf("group callback lacks committed matching binding")
			}}, codex.New())
			req := installerRequest(root)
			req.Targets = []installer.ClientTarget{
				{ClientID: "codex", ClientConfigRoot: filepath.Join(root, "TEST-codex-profile"), ClientExecutable: req.ClientExecutable},
				{ClientID: "vscode", ClientConfigRoot: req.ClientConfigRoot, ClientExecutable: req.ClientExecutable},
			}
			h, err := eng.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = h.Close() }()
			plan := h.Plan()
			if len(plan.Targets) != 2 || plan.Targets[1].SelectedDelivery.IsZero() || !plan.Targets[0].SelectedDelivery.IsZero() {
				t.Fatal("group review lost selected or historical mode")
			}
			req.Targets[1].ClientConfigRoot = filepath.Join(root, "TEST-caller-changed-profile")
			plan.Targets[1].SelectedDelivery = domain.SelectedDelivery{}
			if changed {
				a.targetShell = "TEST-changed-second-target"
			}
			got, err := eng.Apply(t.Context(), h, confirmedDecision())
			if changed {
				if !errors.Is(err, installer.ErrPlanChanged) || got.Mutated || a.activations != 0 || len(handed) != 0 || len(mustLoadFacadeState(t, root).Installations) != 0 {
					t.Fatalf("stale second target affected group: %+v %v", got, err)
				}
				for _, path := range []string{filepath.Join(root, "TEST-codex-profile"), filepath.Join(root, "state", "managed"), filepath.Join(root, "state", "mutation.lock")} {
					if _, err := os.Lstat(path); !os.IsNotExist(err) {
						t.Fatalf("stale group staged %s: %v", path, err)
					}
				}
				return
			}
			if err != nil || got.Outcome != installer.OutcomeCompleted || len(got.Targets) != 2 || len(handed) != 2 || a.activations != 1 {
				t.Fatalf("matching group failed: %+v %v", got, err)
			}
			for _, target := range got.Targets {
				for _, f := range handed {
					if f.BindingID == target.BindingID && !reflect.DeepEqual(target.SelectedDelivery, f.SelectedDelivery) {
						t.Fatal("group result lost committed selection")
					}
				}
			}
			_ = h.Close()
			req.Targets[1].ClientConfigRoot = filepath.Join(root, "profile")
			req.InstallationID = got.InstallationID
			repeat, err := eng.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = repeat.Close() }()
			again, err := eng.Apply(t.Context(), repeat, confirmedDecision())
			if err != nil || again.Outcome != installer.OutcomeUnchanged || len(again.Targets) != 2 {
				t.Fatalf("matching group retry failed: %+v %v", again, err)
			}
			for i := range got.Targets {
				if !reflect.DeepEqual(got.Targets[i].SelectedDelivery, again.Targets[i].SelectedDelivery) {
					t.Fatal("unchanged group lost committed selection")
				}
			}
		})
	}
}

// Red: the guarded replan refuses stage-callback drift after creating the
// mutation lock. Single-target confirmation must also precede that effect.
func TestLocalFacadeReplanBeforeStage(t *testing.T) {
	root := localProcessRoot(t)
	a := newFacadeAdapter(root)
	eng := facadeEngine(t, root, a, installer.Config{Progress: func(event installer.ProgressEvent) {
		if event.Phase == installer.ProgressStage {
			a.targetShell = "TEST-changed-during-apply"
		}
	}})
	h, err := eng.Prepare(t.Context(), installerRequest(root))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()
	got, err := eng.Apply(t.Context(), h, confirmedDecision())
	if !errors.Is(err, installer.ErrPlanChanged) || got.Mutated || a.activations != 0 || len(mustLoadFacadeState(t, root).Installations) != 0 {
		t.Fatalf("replan drift reached staging: %+v %v", got, err)
	}
	if entries, err := os.ReadDir(filepath.Join(root, "state", "managed")); err != nil || len(entries) != 0 {
		t.Fatalf("replan drift staged artifacts: %v %v", entries, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "state", "mutation.lock")); !os.IsNotExist(err) {
		t.Fatalf("late single confirmation reached mutation lock: %v", err)
	}
}

// Intercept actual native observer dispatch without starting a process.
type facadeCommandObserver struct{ commands []legacyports.Command }

func (r *facadeCommandObserver) Run(_ context.Context, c legacyports.Command) (legacyports.CommandResult, error) {
	r.commands = append(r.commands, c)
	return legacyports.CommandResult{Stdout: []byte(`{"installed":[]}`)}, nil
}

// Red on rejected R1: Codex identity discovery dispatches plugin list and
// writes mutation.lock before the second target's late drift is refused.
func TestLocalFacadeLateGroupConfirmation(t *testing.T) {
	for _, phase := range []installer.ProgressPhase{installer.ProgressPreflight, installer.ProgressStage} {
		t.Run(string(phase), func(t *testing.T) {
			root := localProcessRoot(t)
			a := newFacadeAdapter(root)
			runner := &facadeCommandObserver{}
			eng := facadeEngine(t, root, a, installer.Config{
				EnableNativeObserver: true, Runner: runner,
				Progress: func(event installer.ProgressEvent) {
					if event.Phase == phase {
						a.targetShell = "TEST-late-second-target"
					}
				},
			}, codex.New())
			profile := filepath.Join(root, "TEST-codex-profile")
			if err := os.MkdirAll(profile, 0700); err != nil {
				t.Fatal(err)
			}
			req := installerRequest(root)
			req.Targets = []installer.ClientTarget{
				{ClientID: "codex", ClientConfigRoot: profile, ClientExecutable: req.ClientExecutable},
				{ClientID: "vscode", ClientConfigRoot: req.ClientConfigRoot, ClientExecutable: req.ClientExecutable},
			}
			h, err := eng.Prepare(t.Context(), req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = h.Close() }()
			if len(runner.commands) != 0 {
				t.Fatal("Prepare dispatched a command")
			}
			stateBefore := mustLoadFacadeState(t, root)
			settings := filepath.Join(root, "profile", "settings.json")
			before, err := os.ReadFile(settings)
			if err != nil {
				t.Fatal(err)
			}
			got, err := eng.Apply(t.Context(), h, confirmedDecision())
			if !errors.Is(err, installer.ErrPlanChanged) || got.Outcome != installer.OutcomeConflict || got.Mutated {
				t.Fatalf("late second-target drift admitted: %+v %v", got, err)
			}
			after, readErr := os.ReadFile(settings)
			if readErr != nil || !bytes.Equal(before, after) || !reflect.DeepEqual(stateBefore, mustLoadFacadeState(t, root)) || a.activations != 0 || a.discoveries != 0 {
				t.Fatal("late refusal changed TEST state/profile or invoked activation/discovery")
			}
			_, lockErr := os.Lstat(filepath.Join(root, "state", "mutation.lock"))
			if len(runner.commands) != 0 || !os.IsNotExist(lockErr) {
				t.Fatalf("late group confirmation reached commands=%d mutation.lock=%v", len(runner.commands), lockErr)
			}
		})
	}
}

// Red: narrowed identity comparators omit distinct operational fields. Each
// candidate here remains a valid plan; the stale confirmation alone refuses it.
func TestLocalFacadeCompleteAuthority(t *testing.T) {
	changes := map[string]func(*facadeLocalAdapter){
		"physical-profile": func(a *facadeLocalAdapter) {
			a.change = func(f *domain.LocalDeliveryFacts) { f.ProfileIdentity += "-changed" }
		},
		"physical-settings": func(a *facadeLocalAdapter) {
			a.change = func(f *domain.LocalDeliveryFacts) { f.SettingsIdentity += "-changed" }
		},
		"profile-path": func(a *facadeLocalAdapter) {
			a.change = func(f *domain.LocalDeliveryFacts) {
				f.ProfileRoot += "-changed"
				f.SettingsPath = filepath.Join(f.ProfileRoot, "settings.json")
			}
		},
		"settings-path": func(a *facadeLocalAdapter) {
			a.change = func(f *domain.LocalDeliveryFacts) {
				f.SettingsPath = filepath.Join(f.ProfileRoot, "TEST-other-settings.json")
			}
		},
		"tuple": func(a *facadeLocalAdapter) {
			a.change = func(f *domain.LocalDeliveryFacts) { f.Tuple.CopilotVersion += "-changed" }
		},
		"native-stop": func(a *facadeLocalAdapter) { a.change = func(f *domain.LocalDeliveryFacts) { f.NativeStop = false } },
		"owned-object": func(a *facadeLocalAdapter) {
			a.change = func(f *domain.LocalDeliveryFacts) { f.Registration.ObjectID += "-changed" }
		},
		"disabled": func(a *facadeLocalAdapter) {
			a.change = func(f *domain.LocalDeliveryFacts) { disabled := false; f.Registration.DesiredValue = &disabled }
		},
		"previous-presence": func(a *facadeLocalAdapter) {
			a.change = func(f *domain.LocalDeliveryFacts) { enabled := true; f.Registration.PreviousValue = &enabled }
		},
		"mcp":    func(a *facadeLocalAdapter) { a.chooseMCP = false },
		"skills": func(a *facadeLocalAdapter) { a.unselectSkill = true },
		"mode":   func(a *facadeLocalAdapter) { a.historical = true },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			root := localProcessRoot(t)
			facadeComponents(t, root)
			a := newFacadeAdapter(root)
			a.chooseMCP = true
			eng := facadeEngine(t, root, a, installer.Config{})
			h, err := eng.Prepare(t.Context(), installerRequest(root))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = h.Close() }()
			before, _ := os.ReadFile(filepath.Join(root, "profile", "settings.json"))
			change(a)
			got, err := eng.Apply(t.Context(), h, confirmedDecision())
			if !errors.Is(err, installer.ErrPlanChanged) || got.Mutated || a.activations != 0 || len(mustLoadFacadeState(t, root).Installations) != 0 {
				t.Fatalf("authority drift admitted: %+v %v", got, err)
			}
			after, _ := os.ReadFile(filepath.Join(root, "profile", "settings.json"))
			if string(before) != string(after) {
				t.Fatal("authority refusal changed profile")
			}
		})
	}
}
