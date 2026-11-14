package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestValidateSingleExistingToggleTargetRejectsMultipleTargets(t *testing.T) {
	t.Parallel()

	_, err := validateSingleExistingToggleTarget([]plannedExistingTarget{{}, {}})
	if err == nil || !strings.Contains(err.Error(), "supports one target at a time") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsureExistingTogglePlanAllowedRejectsBlockingPlans(t *testing.T) {
	t.Parallel()

	err := ensureExistingTogglePlanAllowed(plannedExistingTarget{
		TargetID: domain.TargetGemini,
		Plan:     ports.AdapterPlan{Blocking: true},
	})
	if err == nil || !strings.Contains(err.Error(), "planned mutation is blocked for target gemini") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildExistingToggleApplyInputPreservesPolicyAndInspect(t *testing.T) {
	t.Parallel()

	record := domain.InstallationRecord{Policy: domain.InstallPolicy{Scope: "project"}}
	target := plannedExistingTarget{
		Plan:    ports.AdapterPlan{ActionClass: "enable_target"},
		Inspect: ports.InspectResult{TargetID: domain.TargetGemini},
	}
	got := buildExistingToggleApplyInput(record, target)
	if got.Plan.ActionClass != "enable_target" || got.Policy.Scope != "project" || got.Inspect.TargetID != domain.TargetGemini || got.Record == nil {
		t.Fatalf("apply input = %+v", got)
	}
}

func TestExistingToggleMutationApplyRejectsUnsupportedAction(t *testing.T) {
	t.Parallel()

	_, err := existingToggleMutationApply(stubTargetAdapter{id: domain.TargetGemini}, "unknown_action")
	if err == nil || !strings.Contains(err.Error(), "unsupported existing lifecycle action unknown_action") {
		t.Fatalf("error = %v", err)
	}
}

func TestExistingToggleMutationApplySelectsEnableOperation(t *testing.T) {
	t.Parallel()

	called := false
	apply, err := existingToggleMutationApply(toggleOnlyAdapter{
		applyEnable: func(context.Context, ports.ApplyInput) (ports.ApplyResult, error) {
			called = true
			return ports.ApplyResult{TargetID: domain.TargetGemini}, nil
		},
	}, "enable_target")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := apply(context.Background(), ports.ApplyInput{}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected enable apply function to be selected")
	}
}

type toggleOnlyAdapter struct {
	applyEnable  func(context.Context, ports.ApplyInput) (ports.ApplyResult, error)
	applyDisable func(context.Context, ports.ApplyInput) (ports.ApplyResult, error)
}

func (a toggleOnlyAdapter) PlanEnable(context.Context, ports.PlanToggleInput) (ports.AdapterPlan, error) {
	return ports.AdapterPlan{}, nil
}

func (a toggleOnlyAdapter) ApplyEnable(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if a.applyEnable != nil {
		return a.applyEnable(ctx, in)
	}
	return ports.ApplyResult{}, nil
}

func (a toggleOnlyAdapter) PlanDisable(context.Context, ports.PlanToggleInput) (ports.AdapterPlan, error) {
	return ports.AdapterPlan{}, nil
}

func (a toggleOnlyAdapter) ApplyDisable(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	if a.applyDisable != nil {
		return a.applyDisable(ctx, in)
	}
	return ports.ApplyResult{}, nil
}
