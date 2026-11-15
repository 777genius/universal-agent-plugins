package agentpluginscli

import (
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The projection observation also reports incomplete CLI registration as
// false,nil. That is a reason to review the existing repair, never readiness
// evidence and never permission to perform setup before acceptance.
func TestSkillsResumeIncompleteRegistrationRequiresAcceptedNativeResult(t *testing.T) {
	journey, request, runner := unchangedSkillsFixture(t)
	seedIncompleteSkillsBinding(t, journey, domain.ActivationPrepared, domain.AuthenticationNotChecked)
	before := savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)
	plan, err := journey.cli.app.PrepareSkills(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if actions := plan.Actions(); len(actions) != 1 || actions[0].Action != "repair" {
		t.Fatalf("incomplete registration did not review repair: %+v", actions)
	}
	adapter, err := managementNativeAdapter(t.Context(), journey.cli.app)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := adapter.Prepare(t.Context(), plan.request.prepared[0])
	if err != nil {
		t.Fatal(err)
	}
	complete, err := journey.cli.app.Installer.VerifyPreparedProjection(t.Context(), prepared)
	if closeErr := prepared.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil || complete {
		t.Fatalf("fixture must exercise incomplete registration, got %v,%v", complete, err)
	}
	if runner.mutations != 0 || !reflect.DeepEqual(before, savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)) {
		t.Fatal("incomplete observation performed setup before acceptance")
	}
	runner.readOnly = false
	result, err := plan.Apply(t.Context())
	if err != nil || !skillsNativeReady(result.Items) {
		t.Fatalf("accepted repair did not obtain native readiness: %+v,%v", result, err)
	}
	if runner.mutations == 0 {
		t.Fatal("prepared registration reused readiness without native setup")
	}
}

func TestSkillsResumePreparedSetupPreservesExplicitAuthentication(t *testing.T) {
	for _, authentication := range []domain.AuthenticationState{domain.AuthenticationPending, domain.AuthenticationFailed} {
		t.Run(string(authentication), func(t *testing.T) {
			journey, request, runner := unchangedSkillsFixture(t)
			seedIncompleteSkillsBinding(t, journey, domain.ActivationPrepared, authentication)
			plan, err := journey.cli.app.PrepareSkills(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			if actions := plan.Actions(); len(actions) != 1 || actions[0].Action != "repair" {
				t.Fatalf("prepared setup repair was not reviewed: %+v", actions)
			}
			runner.readOnly = false
			result, err := plan.Apply(t.Context())
			if err != nil || len(result.Items) != 1 || result.Items[0].Client.Authentication != string(authentication) || skillsNativeReady(result.Items) {
				t.Fatalf("setup observation changed explicit authentication: %+v,%v", result, err)
			}
			if binding := savedJourneyBinding(t, journey.state(t), "codex"); binding.Authentication != authentication {
				t.Fatalf("setup overwrote saved authentication: %+v", binding)
			}
		})
	}
}
