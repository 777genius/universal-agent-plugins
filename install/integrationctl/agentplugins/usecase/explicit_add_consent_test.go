package usecase

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/plannertest"
)

// consentActor records the explicit-add consent of every acting request and
// admits prepared setup completion only with it, like a trust-granting client.
type consentActor struct {
	*existingSetupActor
	consents []bool
}

func (a *consentActor) Activate(ctx context.Context, r domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if !r.VerifyOnly {
		a.consents = append(a.consents, r.ProjectTrustConsent)
	}
	return a.existingSetupActor.Activate(ctx, r)
}

func (a *consentActor) CanCompletePreparedSetup(r domain.ActivationRequest, observed domain.ActivationOutcome) bool {
	return r.ProjectTrustConsent && a.existingSetupActor.CanCompletePreparedSetup(r, observed)
}

// Native project trust must follow only an explicit add, including finishing
// prepared setup on a repeated add, and never an update of the same package.
func TestExplicitAddConsentReachesOnlyAddActivations(t *testing.T) {
	service, store, _ := serviceFixture(t)
	project := t.TempDir()
	service.ProjectRoot = project
	store.Path, store.ProjectRoot = filepath.Join(project, ".uap", "state-v2.json"), project
	service.StateStore, service.Kernel.StateStore = store, store
	service.CheckNamespace = func() error { return service.Paths.RequireContainedChild(project, store.Path) }
	planner := plannertest.NewPlanner(clientplanner.Planner{ProjectRoot: project, ManagedRoot: filepath.Join(project, ".uap", "managed")})
	service.Planner, service.Targets = planner, planner
	client := domain.DetectedClient{ClientID: domain.ClientClaude, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "profile"), ExecutablePath: "/TEST/claude"}
	actor := &consentActor{existingSetupActor: &existingSetupActor{allow: true, marker: filepath.Join(t.TempDir(), "setup-effect"),
		observed: domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, NativeEffect: domain.NativeEffectUnchanged}}}
	service.Activator = actor
	input := addInput(t, client, "https://example.invalid/explicit-add-consent")
	input.Scope, input.BackendExecutable, input.Confirmed, input.ProjectTrustConsent = domain.ScopeProject, client.ExecutablePath, true, true
	if _, err := service.Add(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	actor.needsSetup = true
	if repeated, err := service.Add(t.Context(), input); err != nil || !repeated.Mutated {
		t.Fatalf("repeated explicit add did not finish prepared setup: %+v %v", repeated, err)
	}
	if !slices.Equal(actor.consents, []bool{true, true}) {
		t.Fatalf("explicit add consent did not reach both acting requests: %v", actor.consents)
	}
	actor.needsSetup = true
	update := input
	update.ProjectTrustConsent = false
	if _, err := service.Update(t.Context(), update); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(actor.consents, []bool{true, true}) || actor.acts != 2 {
		t.Fatalf("update acted or carried add consent: %v acts=%d", actor.consents, actor.acts)
	}
}
