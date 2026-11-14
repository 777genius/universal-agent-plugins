//go:build linux

package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/grok"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Stop the existing recovery callback after the real core has persisted the
// removal decision. The native record and finite footer remain exactly those
// written by the production adapter; no fixture journal is manufactured.
type grokRemovalDecisionBoundary struct{ armed, stopped bool }
type grokDecisionBoundaryAdapter struct {
	*grok.Adapter
	boundary *grokRemovalDecisionBoundary
}
type grokDecisionBoundaryRecorder struct {
	*grok.EnablementRecovery
	boundary *grokRemovalDecisionBoundary
}

func (adapter grokDecisionBoundaryAdapter) NewNativeTransitions(authority clients.NativeTransitionContext) ports.ObservedNativeTransitionRecovery {
	inner := adapter.Adapter.NewNativeTransitions(authority).(*grok.EnablementRecovery)
	return &grokDecisionBoundaryRecorder{EnablementRecovery: inner, boundary: adapter.boundary}
}

func (adapter grokDecisionBoundaryAdapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if recorder, ok := env.NativeTransitions.(*grokDecisionBoundaryRecorder); ok {
		env.NativeTransitions = recorder.EnablementRecovery
	}
	return adapter.Adapter.Deactivate(ctx, env, request)
}

func (recorder *grokDecisionBoundaryRecorder) PrepareNativeRemovalDecision(ctx context.Context, installationID, bindingID string, before, desired domain.StateFileV2) (bool, error) {
	handled, err := recorder.EnablementRecovery.PrepareNativeRemovalDecision(ctx, installationID, bindingID, before, desired)
	if handled && err == nil {
		recorder.boundary.armed = true
	}
	return handled, err
}

func (recorder *grokDecisionBoundaryRecorder) Recover(ctx context.Context) error {
	if recorder.boundary.armed && !recorder.boundary.stopped {
		recorder.boundary.stopped = true
		return errors.New("inert interruption after durable native removal decision")
	}
	return recorder.EnablementRecovery.Recover(ctx)
}

func TestProjectGrokSDKColdSavedRemovalDecisionRetainsOwnerAfterMarkerCleared(t *testing.T) {
	f := enableGrokSDKFixture(t)
	request := f.selected(t, OpRemove, false)
	binding := f.binding(t)
	boundary := &grokRemovalDecisionBoundary{}
	registry, err := clients.NewRegistry(grokDecisionBoundaryAdapter{Adapter: grok.New(), boundary: boundary})
	if err != nil {
		t.Fatal(err)
	}
	cfg := f.engine.cfg
	cfg.Registry = registry
	f.engine, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := f.execute(t, request, Decision{Confirmed: true}); err == nil {
		t.Fatalf("removal decision interruption accepted: %+v", result)
	}
	if !boundary.stopped {
		t.Fatal("fixture missed the actual post-state-commit recovery boundary")
	}
	current := f.binding(t)
	if current.NativeActivationAttempt != "" || len(current.NativeObjects) != 1 || current.Activation != domain.ActivationNotRequired {
		t.Fatalf("core did not publish its exact removal decision: %+v", current)
	}
	if _, err := os.Stat(binding.TargetLocator); err != nil {
		t.Fatal("package removed before native record cleanup", err)
	}
	f.engine = f.reopen(t, f.home)
	view, err := f.engine.Inspect(t.Context())
	if err != nil || !view.Recovery.Required || len(view.Recovery.Journals) != 1 || view.Recovery.Journals[0].EnvironmentID != "project-default" || view.Recovery.Journals[0].Phase != "native_state_decision" {
		t.Fatalf("cleared marker lost saved recovery ownership: %+v %v", view, err)
	}
	before, err := f.engine.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	config := deathRead(t, f.config)
	if _, err := f.engine.RecoverEnvironment(t.Context(), view, "foreign-environment"); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatal("foreign environment adopted completed native decision", err)
	}
	after, err := f.engine.store.Load()
	if err != nil || !reflect.DeepEqual(before, after) || deathRead(t, f.config) != config {
		t.Fatal("foreign recovery changed completed native decision", err)
	}
	result, err := f.engine.RecoverEnvironment(t.Context(), view, "project-default")
	if err != nil || result.Outcome != OutcomeCompleted {
		t.Fatalf("cold completed-decision recovery: %+v %v", result, err)
	}
	if deathRead(t, f.config) != config || f.native.duplex != 4 {
		t.Fatal("completed decision recovery reran native mutation")
	}
	result, err = f.execute(t, f.selected(t, OpRemove, false), Decision{Confirmed: true})
	if err != nil || result.Outcome != OutcomeCompleted {
		t.Fatalf("remove already-disabled package after recovery: %+v %v", result, err)
	}
	if _, err := os.Stat(binding.TargetLocator); !os.IsNotExist(err) {
		t.Fatal("owned package retained after settled removal", err)
	}
}

func TestProjectGrokSDKInspectionDoesNotCleanPartialNativePublication(t *testing.T) {
	f := newGrokEnableSDKFixture(t, true)
	f.install(t)
	stage := filepath.Join(f.engine.cfg.OperationsDir, ".grok-enable-stage-foreign")
	deathWrite(t, stage, "partial private publication")
	before, err := os.Stat(stage)
	if err != nil {
		t.Fatal(err)
	}
	state := deathRead(t, filepath.Join(f.project, ".uap", "state-v2.json"))
	view, err := f.engine.Inspect(t.Context())
	if err == nil || !view.Recovery.Required {
		t.Fatalf("partial publication was silently adopted: %+v %v", view, err)
	}
	after, err := os.Stat(stage)
	if err != nil || !os.SameFile(before, after) || deathRead(t, stage) != "partial private publication" || deathRead(t, filepath.Join(f.project, ".uap", "state-v2.json")) != state || f.native.duplex != 0 {
		t.Fatal("read-only inspection cleaned or changed native authority", err)
	}
}
