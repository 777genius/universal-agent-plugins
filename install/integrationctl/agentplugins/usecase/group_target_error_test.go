package usecase

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// clientFailingStager fails staging for one client and stages the others.
type clientFailingStager struct {
	ports.PackageStager
	client domain.ClientID
	err    error
	// failed, when set, records that the failure happened.
	failed *bool
}

func (stager clientFailingStager) Stage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints) (domain.StagedDelivery, error) {
	if plan.ClientID == stager.client {
		if stager.failed != nil {
			*stager.failed = true
		}
		return domain.StagedDelivery{}, stager.err
	}
	return stager.PackageStager.Stage(ctx, envelope, plan, operationID, hints)
}

// clientFailingPlanner cannot plan for one client.
type clientFailingPlanner struct {
	ports.DeliveryPlanner
	client domain.ClientID
	err    error
}

func (planner clientFailingPlanner) Plan(ctx context.Context, request domain.PlanRequest) (domain.DeliveryPlan, error) {
	if request.Client.ClientID == planner.client {
		return domain.DeliveryPlan{}, planner.err
	}
	return planner.DeliveryPlanner.Plan(ctx, request)
}

// clientForeignObserver reports a foreign object at one client's destination.
type clientForeignObserver struct{ client domain.ClientID }

func (observer clientForeignObserver) ObserveNativeIdentity(_ context.Context, client domain.DetectedClient, _ domain.DeliveryPlan, _ *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	if client.ClientID == observer.client {
		return domain.NativeIdentityObservation{State: domain.NativeIdentityUnmanaged}, nil
	}
	return domain.NativeIdentityObservation{State: domain.NativeIdentityAbsent}, nil
}

// A group attributes a failure to a client only when that client's own checks
// or staging failed, before anything changed, and keeps the cause's text.
// Input, scope and lifecycle refusals stay unattributed.
func TestGroupAttributesOnlyTargetLocalFailures(t *testing.T) {
	t.Parallel()
	permission := fmt.Errorf("create client target root: %w", &fs.PathError{Op: "mkdir", Path: "/project/.kiro", Err: syscall.EACCES})
	cases := map[string]struct {
		stageErr   error
		planErr    error
		foreign    bool
		badScope   bool
		globalFail bool
		attributed bool
		phase      GroupPhase
	}{
		"staging permission":      {stageErr: permission, attributed: true, phase: GroupPhasePreparationFailed},
		"preflight foreign":       {foreign: true, attributed: true, phase: GroupPhasePlanned},
		"planning unavailable":    {planErr: errors.New("kiro facts unavailable"), attributed: true, phase: GroupPhasePlanned},
		"input scope refusal":     {badScope: true, phase: GroupPhasePlanned},
		"staging canceled":        {stageErr: fmt.Errorf("stage: %w", context.Canceled), phase: GroupPhasePreparationFailed},
		"staging deadline":        {stageErr: fmt.Errorf("stage: %w", context.DeadlineExceeded), phase: GroupPhasePreparationFailed},
		"unrelated staging cause": {stageErr: errors.New("disk said no"), attributed: true, phase: GroupPhasePreparationFailed},
		"group-wide check fails":  {stageErr: permission, globalFail: true, phase: GroupPhasePreparationFailed},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			service, store, cursor := serviceFixture(t)
			kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}
			failed := false
			if tc.stageErr != nil {
				service.Stager = clientFailingStager{PackageStager: service.Stager, client: domain.ClientKiro, err: tc.stageErr, failed: &failed}
			}
			if tc.globalFail {
				// A check that covers every target starts failing together with
				// kiro's staging, so the failure may not be kiro's own.
				service.CheckNamespace = func() error {
					if failed {
						return errors.New("profile namespace changed")
					}
					return nil
				}
			}
			if tc.planErr != nil {
				service.Planner = clientFailingPlanner{DeliveryPlanner: service.Planner, client: domain.ClientKiro, err: tc.planErr}
			}
			if tc.foreign {
				service.NativeObserver = clientForeignObserver{client: domain.ClientKiro}
			}
			second := addInput(t, kiro, "https://example.com/attribution")
			if tc.badScope {
				second.Scope = domain.ScopeProject
			}
			result, err := service.AddGroup(context.Background(), GroupInput{
				Targets: []AddInput{addInput(t, cursor, "https://example.com/attribution"), second}, OperationGroupID: "attribution", Confirmed: true,
			})
			if err == nil || result.Mutated || result.Phase != tc.phase {
				t.Fatalf("group = phase %s mutated %v err %v", result.Phase, result.Mutated, err)
			}
			var attributed *GroupTargetError
			if errors.As(err, &attributed) != tc.attributed {
				t.Fatalf("attribution = %v, want %v: %v", attributed, tc.attributed, err)
			}
			if tc.attributed && (len(attributed.Clients) != 1 || attributed.Clients[0] != domain.ClientKiro) {
				t.Fatalf("attributed clients = %v", attributed.Clients)
			}
			if tc.stageErr != nil && tc.attributed && (err.Error() != tc.stageErr.Error() || !errors.Is(err, tc.stageErr)) {
				t.Fatalf("attribution changed the cause: %q", err.Error())
			}
			state, loadErr := store.Load()
			if loadErr != nil || len(state.Installations) != 0 {
				t.Fatalf("refused group changed state: %+v %v", state.Installations, loadErr)
			}
		})
	}
}
