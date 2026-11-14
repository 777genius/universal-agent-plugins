package cursor

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (a Adapter) PlanInstall(_ context.Context, in ports.PlanInstallInput) (ports.AdapterPlan, error) {
	return a.planInstall(in), nil
}

func (a Adapter) ApplyInstall(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	return a.applyInstall(ctx, in)
}

func (a Adapter) PlanUpdate(_ context.Context, in ports.PlanUpdateInput) (ports.AdapterPlan, error) {
	return a.planUpdate(in), nil
}

func (a Adapter) ApplyUpdate(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	return a.applyUpdate(ctx, in)
}

func (a Adapter) PlanRemove(_ context.Context, in ports.PlanRemoveInput) (ports.AdapterPlan, error) {
	return a.planRemove(in), nil
}

func (a Adapter) ApplyRemove(ctx context.Context, in ports.ApplyInput) (ports.ApplyResult, error) {
	return a.applyRemove(ctx, in)
}

func (a Adapter) Repair(ctx context.Context, in ports.RepairInput) (ports.ApplyResult, error) {
	return a.repair(ctx, in)
}
