package app

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl"
)

type IntegrationController interface {
	Add(context.Context, integrationctl.AddParams) (integrationctl.Result, error)
	Update(context.Context, integrationctl.UpdateParams) (integrationctl.Result, error)
	Remove(context.Context, integrationctl.RemoveParams) (integrationctl.Result, error)
	Repair(context.Context, integrationctl.RepairParams) (integrationctl.Result, error)
	Enable(context.Context, integrationctl.ToggleParams) (integrationctl.Result, error)
	Disable(context.Context, integrationctl.ToggleParams) (integrationctl.Result, error)
	Sync(context.Context, integrationctl.SyncParams) (integrationctl.Result, error)
	List(context.Context) (integrationctl.Report, error)
	Doctor(context.Context) (integrationctl.Report, error)
}

type integrationctlFacade struct{}

func (integrationctlFacade) Add(ctx context.Context, p integrationctl.AddParams) (integrationctl.Result, error) {
	return integrationctl.Add(ctx, p)
}

func (integrationctlFacade) Update(ctx context.Context, p integrationctl.UpdateParams) (integrationctl.Result, error) {
	return integrationctl.Update(ctx, p)
}

func (integrationctlFacade) Remove(ctx context.Context, p integrationctl.RemoveParams) (integrationctl.Result, error) {
	return integrationctl.Remove(ctx, p)
}

func (integrationctlFacade) Repair(ctx context.Context, p integrationctl.RepairParams) (integrationctl.Result, error) {
	return integrationctl.Repair(ctx, p)
}

func (integrationctlFacade) Enable(ctx context.Context, p integrationctl.ToggleParams) (integrationctl.Result, error) {
	return integrationctl.Enable(ctx, p)
}

func (integrationctlFacade) Disable(ctx context.Context, p integrationctl.ToggleParams) (integrationctl.Result, error) {
	return integrationctl.Disable(ctx, p)
}

func (integrationctlFacade) Sync(ctx context.Context, p integrationctl.SyncParams) (integrationctl.Result, error) {
	return integrationctl.Sync(ctx, p)
}

func (integrationctlFacade) List(ctx context.Context) (integrationctl.Report, error) {
	return integrationctl.List(ctx)
}

func (integrationctlFacade) Doctor(ctx context.Context) (integrationctl.Report, error) {
	return integrationctl.Doctor(ctx)
}

type IntegrationsRunner struct {
	Controller IntegrationController
}

func NewIntegrationsRunner(controller IntegrationController) *IntegrationsRunner {
	if controller == nil {
		controller = integrationctlFacade{}
	}
	return &IntegrationsRunner{Controller: controller}
}
