package planner

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestPlanningRequiresAPathPolicy(t *testing.T) {
	t.Parallel()
	bare := Planner{ManagedRoot: t.TempDir()}
	request := domain.PlanRequest{
		Envelope:           testEnvelope(),
		Client:             detectedClient(domain.ClientCursor, filepath.Join(t.TempDir(), ".cursor")),
		Scope:              domain.ScopeUser,
		PhysicalArtifactID: "demo-0123456789ab",
	}
	if _, err := bare.Plan(context.Background(), request); !errors.Is(err, errPathPolicyRequired) {
		t.Fatalf("Plan without a path policy = %v", err)
	}
	if _, err := bare.ResolveTarget(context.Background(), request.Client, request.Scope, request.PhysicalArtifactID); !errors.Is(err, errPathPolicyRequired) {
		t.Fatalf("ResolveTarget without a path policy = %v", err)
	}
}

// TestPlanAppliesTheRequestedInstallIntent covers the step the use case used to
// run after planning: intent now belongs to the request, and a caller cannot
// forget to apply it.
func TestPlanAppliesTheRequestedInstallIntent(t *testing.T) {
	t.Parallel()
	planner := testPlanner(Planner{ManagedRoot: t.TempDir()}).Planner
	client := detectedClient(domain.ClientKiro, filepath.Join(t.TempDir(), ".kiro"))
	request := domain.PlanRequest{
		Envelope:           testEnvelope(),
		Client:             client,
		Scope:              domain.ScopeUser,
		PhysicalArtifactID: "demo-0123456789ab",
		InstallIntent:      domain.InstallIntentPrepare,
	}
	plan, err := planner.Plan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if plan.InstallIntent != domain.InstallIntentPrepare || plan.Activation != domain.ActivationPrepared {
		t.Fatalf("prepared Kiro plan = %+v", plan)
	}
	if len(plan.UserActions) != 1 || plan.UserActions[0] != KiroPrepareAction {
		t.Fatalf("preparation actions = %v", plan.UserActions)
	}

	request.Client = detectedClient(domain.ClientCursor, filepath.Join(t.TempDir(), ".cursor"))
	if _, err := planner.Plan(context.Background(), request); err == nil {
		t.Fatal("preparation was accepted for a client that does not support it")
	}
}

// TestPlanUsesOnlyTheRequestDetectedMap pins Part 11: detection lives on the
// request. A nil map is not a silent fallback to planner session state.
func TestPlanUsesOnlyTheRequestDetectedMap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	copilot := domain.DetectedClient{
		ClientID: domain.ClientCopilot, Status: domain.DetectionDetected,
		ConfigRoot: filepath.Join(root, "copilot"), ExecutablePath: filepath.Join(root, "bin", "copilot"),
	}
	planner := testPlanner(Planner{ManagedRoot: filepath.Join(root, "managed")}).Planner
	request := domain.PlanRequest{
		Envelope:           testEnvelope(),
		Client:             detectedClient(domain.ClientVSCode, filepath.Join(root, "vscode")),
		Scope:              domain.ScopeUser,
		PhysicalArtifactID: "demo-0123456789ab",
	}
	empty, err := planner.Plan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if empty.NativeRegistryExecutable != "" {
		t.Fatalf("a nil request map still bridged through Copilot: %+v", empty)
	}

	request.Detected = map[domain.ClientID]domain.DetectedClient{domain.ClientCopilot: copilot}
	bridged, err := planner.Plan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if bridged.NativeRegistryExecutable != copilot.ExecutablePath {
		t.Fatalf("the request map was ignored: %+v", bridged)
	}
}
