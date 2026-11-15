package agentpluginscli

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func seedIncompleteSkillsBinding(t *testing.T, journey savedJourney, activation domain.ActivationState, authentication domain.AuthenticationState) {
	t.Helper()
	state := journey.state(t)
	for key, binding := range state.Installations[0].Clients {
		binding.Activation, binding.Authentication = activation, authentication
		state.Installations[0].Clients[key] = binding
	}
	if err := journey.cli.store.Save(state); err != nil {
		t.Fatal(err)
	}
}

// Model older saved lifecycle facts while retaining a real exact source pin,
// receipt, owned directory and the existing native protocol fixture.
func TestSkillsResumeReviewsAndRepairsIncompleteNativeSetup(t *testing.T) {
	for _, activation := range []domain.ActivationState{domain.ActivationPrepared, domain.ActivationActive} {
		t.Run(string(activation), func(t *testing.T) {
			journey, request, runner := unchangedSkillsFixture(t)
			seedIncompleteSkillsBinding(t, journey, activation, domain.AuthenticationNotChecked)
			binding := savedJourneyBinding(t, journey.state(t), "codex")
			before := savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)
			nativeBefore := savedJourneyTree(t, binding.TargetLocator)
			requirements, locked := journey.intent(t)
			receipts := journey.state(t).EnvironmentReceipts
			plan, err := journey.cli.app.PrepareSkills(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			if actions := plan.Actions(); len(actions) != 1 || actions[0].Action != "repair" || actions[0].TargetPath != binding.TargetLocator {
				t.Fatalf("incomplete setup was not explicitly reviewed: %+v", actions)
			}
			if err := plan.Check(t.Context()); err != nil || runner.mutations != 0 || !reflect.DeepEqual(before, savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)) {
				t.Fatalf("review changed saved setup or failed recheck: %v", err)
			}
			runner.readOnly = false // Only the accepted repair may invoke native setup.
			result, err := plan.Apply(t.Context())
			if err != nil || !skillsNativeReady(result.Items) || result.Items[0].Action != "repair" {
				t.Fatalf("confirmed repair did not observe ready setup: %+v; %v", result, err)
			}
			after := savedJourneyBinding(t, journey.state(t), "codex")
			if after.Activation != domain.ActivationActive || after.Authentication != domain.AuthenticationNotRequired ||
				!reflect.DeepEqual(nativeBefore, savedJourneyTree(t, binding.TargetLocator)) {
				t.Fatalf("lifecycle correction changed installed bytes or lacked native proof: %+v; runner=%+v", after, runner)
			}
			afterRequirements, afterLock := journey.intent(t)
			if !reflect.DeepEqual(requirements, afterRequirements) || !reflect.DeepEqual(locked, afterLock) ||
				!reflect.DeepEqual(receipts, journey.state(t).EnvironmentReceipts) || !reflect.DeepEqual(binding.PackageRevision, after.PackageRevision) {
				t.Fatal("resume changed saved source, receipt or pinned revision")
			}
		})
	}
}

func TestSkillsResumePreservesUnprovedAuthentication(t *testing.T) {
	for _, authentication := range []domain.AuthenticationState{domain.AuthenticationPending, domain.AuthenticationFailed} {
		t.Run(string(authentication), func(t *testing.T) {
			journey, request, _ := unchangedSkillsFixture(t)
			seedIncompleteSkillsBinding(t, journey, domain.ActivationActive, authentication)
			before := savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)
			plan, err := journey.cli.app.PrepareSkills(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer plan.Close()
			if actions := plan.Actions(); len(actions) != 1 || actions[0].Action != string(environment.ActionUnchanged) {
				t.Fatalf("unrequested setup change was added: %+v", actions)
			}
			result, err := plan.Apply(t.Context())
			if err != nil || skillsNativeReady(result.Items) || !reflect.DeepEqual(before, savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)) {
				t.Fatalf("unproved authentication changed: %+v; %v", result, err)
			}
		})
	}
}

func TestSkillsResumeRefusesDriftForeignOwnershipAndStaleReview(t *testing.T) {
	for _, scenario := range []string{"drift", "foreign", "stale"} {
		t.Run(scenario, func(t *testing.T) {
			journey, request, runner := unchangedSkillsFixture(t)
			seedIncompleteSkillsBinding(t, journey, domain.ActivationPrepared, domain.AuthenticationNotChecked)
			var accepted *PreparedSkillsBatch
			if scenario == "stale" {
				var err error
				accepted, err = journey.cli.app.PrepareSkills(t.Context(), request)
				if err != nil {
					t.Fatal(err)
				}
				defer accepted.Close()
			}
			if scenario == "foreign" {
				state := journey.state(t)
				state.EnvironmentReceipts[0].EnvironmentID = "user:other-environment"
				if err := journey.cli.store.Save(state); err != nil {
					t.Fatal(err)
				}
			} else {
				binding := savedJourneyBinding(t, journey.state(t), "codex")
				dependencyWrite(t, filepath.Join(binding.TargetLocator, "user-edit.txt"), "preserve this edit")
			}
			before := savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)
			var err error
			if accepted != nil {
				_, err = accepted.Apply(t.Context())
				if !errors.Is(err, ErrSkillsPlanStale) {
					t.Fatalf("changed review was not stale: %v", err)
				}
			} else {
				accepted, err = journey.cli.app.PrepareSkills(t.Context(), request)
				if accepted != nil {
					accepted.Close()
				}
			}
			if err == nil || !reflect.DeepEqual(before, savedSkillsDurableTree(t, journey.cli.app.DependenciesRoot)) || runner.mutations != 0 {
				t.Fatalf("refused resume changed owned state: %v; runner=%+v", err, runner)
			}
		})
	}
}

func TestSkillsProjectionRefreshRequiresSingleTypedRefusal(t *testing.T) {
	for _, err := range []error{domain.ErrProjectionRefreshRequired, fmt.Errorf("project: %w", domain.ErrProjectionPinChangeRequired)} {
		if !skillsProjectionRefreshRequired(err) {
			t.Fatalf("typed refusal was not recognized: %v", err)
		}
	}
	for _, err := range []error{nil, errors.New("foreign object"), errors.New(domain.ErrProjectionRefreshRequired.Error()),
		errors.Join(domain.ErrProjectionRefreshRequired, errors.New("foreign object")), fmt.Errorf("wrapped: %w", errors.Join(domain.ErrProjectionPinChangeRequired, errors.New("cleanup failed")))} {
		if skillsProjectionRefreshRequired(err) {
			t.Fatalf("unrelated failure selected a new operation: %v", err)
		}
	}
}
