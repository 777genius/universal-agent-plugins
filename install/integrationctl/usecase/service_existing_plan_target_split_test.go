package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestLoadExistingTargetBaseRejectsMissingTarget(t *testing.T) {
	t.Parallel()

	svc := Service{}
	_, err := svc.loadExistingTargetBase(context.Background(), domain.InstallationRecord{
		IntegrationID: "demo",
		Targets:       map[domain.TargetID]domain.TargetInstallation{},
	}, domain.TargetClaude)
	if err == nil || !strings.Contains(err.Error(), "target missing from installation record: claude") {
		t.Fatalf("error = %v", err)
	}
}

func TestDispatchExistingTargetPlanRejectsUnsupportedAction(t *testing.T) {
	t.Parallel()

	svc := Service{}
	_, err := svc.dispatchExistingTargetPlan(context.Background(), domain.InstallationRecord{
		IntegrationID: "demo",
	}, "unknown_action", plannedExistingTarget{
		TargetID: domain.TargetClaude,
		Adapter:  stubTargetAdapter{id: domain.TargetClaude},
		Inspect:  ports.InspectResult{TargetID: domain.TargetClaude},
	}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported existing lifecycle action unknown_action") {
		t.Fatalf("error = %v", err)
	}
}

func TestExistingTargetDeliveryProjectsStoredNativeRef(t *testing.T) {
	t.Parallel()

	got := existingTargetDelivery(domain.TargetGemini, domain.TargetInstallation{
		DeliveryKind: domain.DeliveryGeminiExtension,
		NativeRef:    "native-ref",
	})
	if got.TargetID != domain.TargetGemini || got.DeliveryKind != domain.DeliveryGeminiExtension || got.NativeRefHint != "native-ref" {
		t.Fatalf("delivery = %+v", got)
	}
}

func TestExistingToggleAdapterRejectsUnsupportedEnable(t *testing.T) {
	t.Parallel()

	_, err := existingToggleAdapter(plannedExistingTarget{
		TargetID: domain.TargetGemini,
		Adapter:  existingPlanTargetNonToggleAdapter{id: domain.TargetGemini},
	}, true)
	if err == nil || !strings.Contains(err.Error(), "target gemini does not support enable") {
		t.Fatalf("error = %v", err)
	}
}

func TestRequireExistingMutationDeliveryRejectsMissingTarget(t *testing.T) {
	t.Parallel()

	_, err := requireExistingMutationDelivery(domain.IntegrationManifest{
		Deliveries: []domain.Delivery{{TargetID: domain.TargetClaude}},
	}, domain.TargetGemini)
	if err == nil || !strings.Contains(err.Error(), "updated manifest no longer exposes target gemini") {
		t.Fatalf("error = %v", err)
	}
}

type existingPlanTargetNonToggleAdapter struct {
	id domain.TargetID
}

func (a existingPlanTargetNonToggleAdapter) ID() domain.TargetID { return a.id }

func (a existingPlanTargetNonToggleAdapter) Capabilities(context.Context) (ports.Capabilities, error) {
	return ports.Capabilities{}, nil
}

func (a existingPlanTargetNonToggleAdapter) Inspect(context.Context, ports.InspectInput) (ports.InspectResult, error) {
	return ports.InspectResult{TargetID: a.id}, nil
}

func (a existingPlanTargetNonToggleAdapter) PlanInstall(context.Context, ports.PlanInstallInput) (ports.AdapterPlan, error) {
	return ports.AdapterPlan{}, nil
}

func (a existingPlanTargetNonToggleAdapter) ApplyInstall(context.Context, ports.ApplyInput) (ports.ApplyResult, error) {
	return ports.ApplyResult{}, nil
}

func (a existingPlanTargetNonToggleAdapter) PlanUpdate(context.Context, ports.PlanUpdateInput) (ports.AdapterPlan, error) {
	return ports.AdapterPlan{}, nil
}

func (a existingPlanTargetNonToggleAdapter) ApplyUpdate(context.Context, ports.ApplyInput) (ports.ApplyResult, error) {
	return ports.ApplyResult{}, nil
}

func (a existingPlanTargetNonToggleAdapter) PlanRemove(context.Context, ports.PlanRemoveInput) (ports.AdapterPlan, error) {
	return ports.AdapterPlan{}, nil
}

func (a existingPlanTargetNonToggleAdapter) ApplyRemove(context.Context, ports.ApplyInput) (ports.ApplyResult, error) {
	return ports.ApplyResult{}, nil
}

func (a existingPlanTargetNonToggleAdapter) Repair(context.Context, ports.RepairInput) (ports.ApplyResult, error) {
	return ports.ApplyResult{}, nil
}
