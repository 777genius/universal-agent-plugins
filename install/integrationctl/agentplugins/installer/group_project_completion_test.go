package installer

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type projectGroupReportFixture struct {
	prepared *PreparedOperation
	got      usecase.GroupResult
	state    domain.StateFileV2
}

func newProjectGroupReportFixture(t *testing.T) projectGroupReportFixture {
	t.Helper()
	project := filepath.Join(t.TempDir(), "project")
	id := "project-group-report"
	prepared := &PreparedOperation{req: Request{Operation: OpRepair, Scope: domain.ScopeProject, ProjectRoot: project, InstallationID: id},
		plan: Plan{InstallationID: id}, detected: map[domain.ClientID]domain.DetectedClient{}}
	got := usecase.GroupResult{InstallationID: id, Phase: usecase.GroupPhaseCompleted}
	installation := domain.Installation{InstallationID: id, Clients: map[string]domain.ClientBinding{}}
	for _, clientID := range []domain.ClientID{domain.ClientCline, domain.ClientClaude} {
		client := domain.DetectedClient{ClientID: clientID, ConfigRoot: filepath.Join(filepath.Dir(project), string(clientID))}
		active := filepath.Join(project, "managed", string(clientID))
		target := domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: client.ConfigRoot}
		plan := domain.DeliveryPlan{ClientID: clientID, Scope: domain.ScopeProject, TargetContext: target, ActivePath: active,
			PhysicalArtifactID: "artifact-" + string(clientID), NativeRegistryRoot: project}
		bindingID := domain.ComputeClientBindingID(id, string(clientID), "project", active)
		prepared.req.Targets = append(prepared.req.Targets, ClientTarget{ClientID: string(clientID), ClientConfigRoot: client.ConfigRoot})
		prepared.clients = append(prepared.clients, client)
		prepared.detected[clientID] = client
		prepared.plan.Targets = append(prepared.plan.Targets, PlanTarget{ClientID: string(clientID), ConfigRoot: client.ConfigRoot, TargetPath: active, BindingID: bindingID})
		prepared.deliveries = append(prepared.deliveries, preparedDeliveryPlan{plan: plan})
		effect := domain.NativeEffectUnchanged
		if clientID == domain.ClientClaude {
			effect = ""
		}
		outcome := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Authentication: domain.AuthenticationNotChecked,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled, NativeEffect: effect,
			UserActions: []string{`{"command":"inspect ` + string(clientID) + `"}`, "load the prepared project"}}
		got.Targets = append(got.Targets, usecase.AddResult{InstallationID: id, Plan: plan, Activation: outcome,
			GroupPhase: usecase.GroupTargetExternalCompleted, NoChange: true})
		installation.Clients[bindingID] = domain.ClientBinding{ClientBindingID: bindingID, ClientID: string(clientID), Scope: "project",
			TargetLocator: active, PhysicalArtifact: plan.PhysicalArtifactID, NativeProfileRoot: project,
			Materialization: domain.MaterializationMaterialized, Activation: outcome.Activation, Authentication: outcome.Authentication,
			Policy: outcome.Policy, Verification: outcome.Verification}
	}
	return projectGroupReportFixture{prepared: prepared, got: got, state: domain.StateFileV2{Installations: []domain.Installation{installation}}}
}

func (f *projectGroupReportFixture) changeBinding(change func(*domain.ClientBinding)) {
	key := f.prepared.plan.Targets[0].BindingID
	binding := f.state.Installations[0].Clients[key]
	change(&binding)
	f.state.Installations[0].Clients[key] = binding
}

func TestFinishProjectGroupRetainsPreparedActionsAndActualMutation(t *testing.T) {
	for _, mutation := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "actual effect"}[mutation], func(t *testing.T) {
			f := newProjectGroupReportFixture(t)
			f.prepared.plan.NoChange = true // A stale preview cannot override the actual operation.
			f.got.Mutated = mutation
			engine := Engine{store: groupReportingStore{state: f.state}}
			result, err := engine.finishGroup(f.prepared, Result{}, true, f.got, nil)
			want := OutcomeUnchanged
			if mutation {
				want = OutcomeCompleted
			}
			if err != nil || result.Outcome != want || result.Mutated != mutation || result.NoChange == mutation {
				t.Fatalf("project completion=%+v error=%v", result, err)
			}
			actions := append(append([]string{}, f.got.Targets[0].Activation.UserActions...), f.got.Targets[1].Activation.UserActions...)
			if !reflect.DeepEqual(actions, result.ManualActions) {
				t.Fatalf("fresh actions changed: %q", result.ManualActions)
			}
			for _, target := range result.Targets {
				if target.Activation != "prepared" || target.Verification != "installation_verified" || target.Authentication != "not_checked" {
					t.Fatalf("presentation promoted lifecycle %+v", target)
				}
			}
		})
	}
}

func TestFinishProjectGroupRejectsInconsistentCompletion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*projectGroupReportFixture)
	}{
		{"group phase", func(f *projectGroupReportFixture) { f.got.Phase = usecase.GroupPhaseManagedCommitted }},
		{"group installation", func(f *projectGroupReportFixture) { f.got.InstallationID = "another" }},
		{"target installation", func(f *projectGroupReportFixture) { f.got.Targets[0].InstallationID = "another" }},
		{"target failure", func(f *projectGroupReportFixture) {
			f.got.Targets[0].Failure = &usecase.GroupTargetFailure{Stage: "persist"}
		}},
		{"target phase", func(f *projectGroupReportFixture) {
			f.got.Targets[0].GroupPhase = usecase.GroupTargetExternalNotAttempted
		}},
		{"missing target", func(f *projectGroupReportFixture) { f.got.Targets = f.got.Targets[:1] }},
		{"duplicate target", func(f *projectGroupReportFixture) { f.got.Targets[1] = f.got.Targets[0] }},
		{"wrong client", func(f *projectGroupReportFixture) { f.got.Targets[0].Plan.ClientID = domain.ClientCodex }},
		{"mixed scope", func(f *projectGroupReportFixture) { f.got.Targets[0].Plan.Scope = domain.ScopeUser }},
		{"project context", func(f *projectGroupReportFixture) { f.got.Targets[0].Plan.TargetContext.ProjectRoot += "-other" }},
		{"original user context", func(f *projectGroupReportFixture) { f.got.Targets[0].Plan.TargetContext.UserConfigRoot += "-other" }},
		{"physical artifact", func(f *projectGroupReportFixture) { f.got.Targets[0].Plan.PhysicalArtifactID = "another" }},
		{"active locator", func(f *projectGroupReportFixture) { f.got.Targets[0].Plan.ActivePath += "-other" }},
		{"native root", func(f *projectGroupReportFixture) { f.got.Targets[0].Plan.NativeRegistryRoot += "-other" }},
		{"namespace", func(f *projectGroupReportFixture) {
			f.got.Targets[0].Plan = f.got.Targets[0].Plan.WithProfileAuthority(nil, "another")
		}},
		{"no frozen delivery", func(f *projectGroupReportFixture) { f.prepared.deliveries = f.prepared.deliveries[1:] }},
		{"missing installation", func(f *projectGroupReportFixture) { f.state.Installations = nil }},
		{"duplicate installation", func(f *projectGroupReportFixture) {
			f.state.Installations = append(f.state.Installations, f.state.Installations[0])
		}},
		{"missing binding", func(f *projectGroupReportFixture) {
			delete(f.state.Installations[0].Clients, f.prepared.plan.Targets[0].BindingID)
		}},
		{"another binding of same client", func(f *projectGroupReportFixture) {
			key := f.prepared.plan.Targets[0].BindingID
			binding := f.state.Installations[0].Clients[key]
			binding.ClientBindingID = "other-binding"
			f.state.Installations[0].Clients[binding.ClientBindingID] = binding
		}},
		{"wrong binding id", func(f *projectGroupReportFixture) {
			f.changeBinding(func(b *domain.ClientBinding) { b.ClientBindingID = "other" })
		}},
		{"wrong saved artifact", func(f *projectGroupReportFixture) {
			f.changeBinding(func(b *domain.ClientBinding) { b.PhysicalArtifact = "other" })
		}},
		{"wrong saved profile", func(f *projectGroupReportFixture) {
			f.changeBinding(func(b *domain.ClientBinding) { b.NativeProfileRoot += "-other" })
		}},
		{"wrong saved namespace", func(f *projectGroupReportFixture) {
			f.changeBinding(func(b *domain.ClientBinding) { b.ProfileNamespace = "other" })
		}},
		{"pending attempt", func(f *projectGroupReportFixture) {
			f.changeBinding(func(b *domain.ClientBinding) { b.NativeActivationAttempt = "unacknowledged" })
		}},
		{"pending intent", func(f *projectGroupReportFixture) {
			f.changeBinding(func(b *domain.ClientBinding) { b.PendingNativeIntent = &domain.PendingNativeIntent{} })
		}},
		{"unmaterialized", func(f *projectGroupReportFixture) {
			f.changeBinding(func(b *domain.ClientBinding) { b.Materialization = domain.MaterializationStaged })
		}},
		{"changed live lifecycle", func(f *projectGroupReportFixture) {
			f.changeBinding(func(b *domain.ClientBinding) { b.Verification = domain.VerificationRuntime })
		}},
		{"prepared attestation", func(f *projectGroupReportFixture) { f.got.Targets[0].Activation.ActivationAttested = true }},
		{"missing fresh actions", func(f *projectGroupReportFixture) { f.got.Targets[0].Activation.UserActions = nil }},
		{"tracked empty effect", func(f *projectGroupReportFixture) { f.got.Targets[0].Activation.NativeEffect = "" }},
		{"legacy unknown effect", func(f *projectGroupReportFixture) { f.got.Targets[1].Activation.NativeEffect = "unknown" }},
		{"uncertain effect", func(f *projectGroupReportFixture) {
			f.got.Targets[0].Activation.NativeEffect = domain.NativeEffectUncertain
		}},
		{"manual activation", func(f *projectGroupReportFixture) { f.got.Targets[0].Activation.Activation = domain.ActivationManual }},
		{"failed activation", func(f *projectGroupReportFixture) { f.got.Targets[0].Activation.Activation = domain.ActivationFailed }},
		{"unknown activation", func(f *projectGroupReportFixture) { f.got.Targets[0].Activation.Activation = "future" }},
		{"failed authentication", func(f *projectGroupReportFixture) {
			f.got.Targets[0].Activation.Authentication = domain.AuthenticationFailed
		}},
		{"unknown authentication", func(f *projectGroupReportFixture) { f.got.Targets[0].Activation.Authentication = "future" }},
		{"blocked policy", func(f *projectGroupReportFixture) { f.got.Targets[0].Activation.Policy = domain.PolicyBlocked }},
		{"unknown policy", func(f *projectGroupReportFixture) { f.got.Targets[0].Activation.Policy = "future" }},
		{"unverified files", func(f *projectGroupReportFixture) {
			f.got.Targets[0].Activation.Verification = domain.VerificationPackageValid
		}},
		{"unknown verification", func(f *projectGroupReportFixture) { f.got.Targets[0].Activation.Verification = "future" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newProjectGroupReportFixture(t)
			tc.change(&f)
			f.got.Mutated = true
			engine := Engine{store: groupReportingStore{state: f.state}}
			result, err := engine.finishGroup(f.prepared, Result{}, true, f.got, nil)
			if err == nil || result.Outcome != OutcomeIncomplete || !result.Mutated || result.NoChange {
				t.Fatalf("invalid completion %+v %v", result, err)
			}
		})
	}
}

func TestFinishProjectGroupRejectsChangedDeliveryOrAuthority(t *testing.T) {
	for _, saved := range []bool{false, true} {
		f := newProjectGroupReportFixture(t)
		var selected domain.SelectedDelivery
		if err := json.Unmarshal([]byte(`{"mode":"unknown-project-mode"}`), &selected); err != nil {
			t.Fatal(err)
		}
		if saved {
			f.changeBinding(func(b *domain.ClientBinding) { b.SelectedDelivery = selected })
		} else {
			f.got.Targets[0].Plan.SelectedDelivery = selected
		}
		if err := completedProjectGroup(f.prepared, f.got, f.state); err == nil {
			t.Fatal("changed selected delivery accepted")
		}
	}
	f := newProjectGroupReportFixture(t)
	// A zero token is still structurally distinct from the absent authority.
	f.changeBinding(func(b *domain.ClientBinding) { b.ProfileAuthority = &domain.ProfileAuthority{} })
	if err := completedProjectGroup(f.prepared, f.got, f.state); err == nil {
		t.Fatal("changed profile authority accepted")
	}
}

func TestFinishProjectGroupStateFailureCannotHideObservedEffects(t *testing.T) {
	loadErr := errors.New("fixture reload failure")
	coreErr := errors.New("fixture core failure")
	for _, mutation := range []bool{false, true} {
		for _, failure := range []error{nil, coreErr} {
			f := newProjectGroupReportFixture(t)
			f.got.Mutated = mutation
			engine := Engine{store: groupReportingStore{err: loadErr}}
			result, err := engine.finishGroup(f.prepared, Result{}, true, f.got, failure)
			expected := loadErr
			if failure != nil {
				expected = failure
			}
			if !errors.Is(err, expected) || result.Outcome != OutcomeIncomplete || result.Mutated != mutation || result.NoChange {
				t.Fatalf("state failure %+v %v", result, err)
			}
			if len(result.ManualActions) != 4 {
				t.Fatalf("fresh actions lost on inspection failure %+v", result)
			}
		}
	}
}

func TestFinishGroupFailurePrecedenceRetainsObservedFacts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cause   error
		reason  string
		outcome Outcome
	}{
		{"ordinary", errors.New("activation failed"), "activation failed", OutcomeIncomplete},
		{"plan", ErrPlanChanged, "plan_changed", OutcomeConflict},
		{"refresh", errors.Join(ErrPlanChanged, usecase.ErrProjectionRefreshRequired), "update_required", OutcomeConflict},
		{"text", errors.Join(ErrPlanChanged, errors.New("run update separately")), "update_required", OutcomeConflict},
		{"pin", errors.Join(ErrPlanChanged, usecase.ErrProjectionPinChangeRequired), "pin_change_required", OutcomeConflict},
		{"all", errors.Join(ErrPlanChanged, usecase.ErrProjectionRefreshRequired, usecase.ErrProjectionPinChangeRequired), "pin_change_required", OutcomeConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newProjectGroupReportFixture(t)
			f.got.Mutated = true
			f.got.Targets[0].Failure = &usecase.GroupTargetFailure{Stage: "activation", Message: "observed failure"}
			engine := Engine{store: groupReportingStore{state: f.state}}
			initial := Result{Binding: BindingFacts{BindingID: "host-fact"}, ManualActions: []string{"host action"}}
			got, err := engine.finishGroup(f.prepared, initial, true, f.got, tc.cause)
			if !errors.Is(err, tc.cause) || err.Error() != wrapLifecycleError(tc.cause).Error() || got.Outcome != tc.outcome || got.Reason != tc.reason || !got.Mutated || got.NoChange {
				t.Fatalf("failure classification %+v %v", got, err)
			}
			wantActions := []string{"host action"}
			for _, target := range f.got.Targets {
				wantActions = append(wantActions, target.Activation.UserActions...)
			}
			if got.Binding.BindingID != f.prepared.plan.Targets[0].BindingID || !reflect.DeepEqual(got.ManualActions, wantActions) || len(got.Targets) != 2 || got.Targets[0].Failure == nil || got.Targets[0].Failure.Message != "observed failure" {
				t.Fatalf("observed facts lost: binding=%q actions=%v targets=%+v", got.Binding.BindingID, got.ManualActions, got.Targets)
			}
		})
	}
}
