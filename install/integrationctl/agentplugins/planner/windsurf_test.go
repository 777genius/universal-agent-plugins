package planner

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestWindsurfPlanActivatesMCPOnlyForOneSelectedLegacyChannel(t *testing.T) {
	t.Parallel()
	planner := Planner{ManagedRoot: t.TempDir()}
	configRoot := filepath.Join(t.TempDir(), ".codeium", "windsurf-next")
	plan, err := planner.Plan(context.Background(), testEnvelope(), detectedClient(domain.ClientWindsurf, configRoot), domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanReady || plan.Activation != domain.ActivationPrepared || plan.NativeRegistryRoot != configRoot {
		t.Fatalf("selected Windsurf plan = %+v", plan)
	}
	if !contains(plan.Warnings, "windsurf_skills_prepared_only") {
		t.Fatalf("Windsurf plan claimed skills activation: %+v", plan)
	}

	manual, err := planner.Plan(context.Background(), testEnvelope(), detectedClient(domain.ClientWindsurf, ""), domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if manual.Status != domain.PlanManualActivationRequired || manual.Activation != domain.ActivationManual {
		t.Fatalf("ambiguous or Devin-only Windsurf plan = %+v", manual)
	}
}
