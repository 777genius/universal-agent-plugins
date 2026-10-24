package usecase

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/plannertest"
)

type preparedGroupObservationActivator struct {
	resumeObservationActivator
	effectPath string
}

func (a *preparedGroupObservationActivator) Activate(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	outcome, err := a.resumeObservationActivator.Activate(ctx, request)
	if err == nil && outcome.NativeEffect == domain.NativeEffectCommitted {
		err = os.WriteFile(a.effectPath, []byte("real inert native fixture effect"), 0600)
	}
	return outcome, err
}

func (*preparedGroupObservationActivator) AutomaticallyActivates(domain.ActivationRequest) bool {
	return true
}

func TestGroupedPreparedObservationReportsActualMutation(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		effect                           domain.NativeEffectState
		wantMutation                     bool
		wantStateChange                  bool
		policy                           domain.PolicyState
		authComplete, activationAttested bool
	}{
		{name: "fresh unchanged prepared"},
		{name: "same recorded facts with native effect", effect: domain.NativeEffectCommitted, wantMutation: true},
		{name: "new policy decision", policy: domain.PolicyBlocked, wantMutation: true, wantStateChange: true},
		{name: "explicit authentication completion", authComplete: true, wantMutation: true, wantStateChange: true},
		{name: "attested outcome is not read only", activationAttested: true, wantMutation: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, store, _ := serviceFixture(t)
			project := t.TempDir()
			service.ProjectRoot = project
			store.Path, store.ProjectRoot = filepath.Join(project, ".uap", "state-v2.json"), project
			service.StateStore, service.Kernel.StateStore = store, store
			// This core fixture supplies the production path policy; the CLI
			// regression separately exercises the SDK namespace guard.
			service.CheckNamespace = func() error { return service.Paths.RequireContainedChild(project, store.Path) }
			planner := plannertest.NewPlanner(clientplanner.Planner{ProjectRoot: project, ManagedRoot: filepath.Join(project, ".uap", "managed")})
			service.Planner, service.Targets = planner, planner
			client := domain.DetectedClient{ClientID: domain.ClientClaude, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".claude")}
			client.ExecutablePath = filepath.Join(project, "inert-claude")
			if err := os.WriteFile(client.ExecutablePath, []byte("never execute this fixture"), 0700); err != nil {
				t.Fatal(err)
			}
			input := kiroPowerInput(t, client, "1.0.0", "sha256:prepared-tree", "sha256:prepared-manifest")
			input.Scope = domain.ScopeProject
			input.Confirmed = true
			activator := &preparedGroupObservationActivator{resumeObservationActivator: resumeObservationActivator{
				observedActivator: observedActivator{outcome: domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed}},
				available:         true,
			}}
			activator.effectPath = filepath.Join(project, "native-fixture-effect")
			service.Activator = activator
			added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{input}, Confirmed: true, OperationGroupID: "prepared-group-add"})
			if err != nil {
				t.Fatal(err)
			}
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			stateBytes, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			binding := onlyBinding(before.Installations[0])
			treeBefore, err := os.ReadFile(filepath.Join(binding.TargetLocator, ".claude-plugin", "plugin.json"))
			if err != nil {
				t.Fatal(err)
			}
			count := len(activator.requests)
			activator.outcome.NativeEffect = tc.effect
			if tc.policy != "" {
				activator.outcome.Policy = tc.policy
			}
			input.AuthComplete = tc.authComplete
			activator.outcome.ActivationAttested = tc.activationAttested
			updated, err := service.UpdateGroup(context.Background(), GroupInput{Targets: []AddInput{input}, CompatibilityChecks: []AddInput{input}, Confirmed: true, OperationGroupID: "prepared-group-update"})
			if err != nil {
				t.Fatal(err)
			}
			if len(updated.Targets) != 1 || updated.Mutated != tc.wantMutation || updated.Targets[0].Mutated != tc.wantMutation || updated.Targets[0].NoChange == tc.wantMutation {
				t.Fatalf("group/target mutation classification=%+v", updated)
			}
			if updated.Phase != GroupPhaseCompleted || len(updated.Receipts) != 0 {
				t.Fatalf("unchanged managed group=%+v", updated)
			}
			if len(activator.requests) != count+1 {
				t.Fatalf("fresh observation missing: requests=%d/%d", len(activator.requests), count)
			}
			request := activator.requests[len(activator.requests)-1]
			if request.VerifyOnly || !request.PackageUnchanged {
				t.Fatalf("incomplete registration facts changed=%+v", request)
			}
			if got := updated.Targets[0].Activation; got.Activation != domain.ActivationPrepared || got.Verification != domain.VerificationInstalled {
				t.Fatalf("prepared observation promoted=%+v", got)
			}
			after, err := store.Load()
			if err != nil || reflect.DeepEqual(before, after) == tc.wantStateChange {
				t.Fatalf("same recorded facts rewritten: %+v %v", after, err)
			}
			afterBytes, err := os.ReadFile(store.Path)
			if err != nil || (string(afterBytes) == string(stateBytes)) == tc.wantStateChange {
				t.Fatalf("state bytes changed: %v", err)
			}
			treeAfter, err := os.ReadFile(filepath.Join(binding.TargetLocator, ".claude-plugin", "plugin.json"))
			if err != nil || string(treeBefore) != string(treeAfter) {
				t.Fatalf("managed bytes changed: %v", err)
			}
			if tc.effect == domain.NativeEffectCommitted {
				body, err := os.ReadFile(activator.effectPath)
				if err != nil || string(body) != "real inert native fixture effect" {
					t.Fatalf("declared native effect was not performed: %v", err)
				}
			}
			if added.InstallationID != updated.InstallationID {
				t.Fatal("installation identity changed")
			}
		})
	}
}
