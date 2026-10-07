package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Prepare at the service boundary, before any planner or process-inert reader.
// Detected is the operation-local map retained across online preview/apply.
func (service *Service) prepareHostInputs(ctx context.Context, inputs []AddInput, dryRun bool) ([]AddInput, error) {
	out := append([]AddInput(nil), inputs...)
	if service.Detected == nil {
		service.Detected = map[domain.ClientID]domain.DetectedClient{}
	}
	for i := range out {
		if retained, ok := service.Detected[out[i].Client.ClientID]; ok && out[i].Client.OpenCodeHost == nil {
			out[i].Client.OpenCodeHost = retained.OpenCodeHost
		}
		if ((!dryRun && !out[i].DryRun) || service.PrepareHostsForPreview) && service.ClientPreparation == nil && service.OpenCodeHosts != nil {
			client, err := service.OpenCodeHosts.PrepareOpenCodeHost(ctx, out[i].Client, out[i].Envelope)
			if err != nil {
				return nil, err
			}
			out[i].Client = client
		}
		service.Detected[out[i].Client.ClientID] = out[i].Client
	}
	return out, nil
}

func (service Service) revalidateHost(ctx context.Context, client domain.DetectedClient) error {
	if service.ClientPreparation != nil || service.OpenCodeHosts == nil || client.OpenCodeHost == nil {
		return nil
	}
	return service.OpenCodeHosts.RevalidateOpenCodeHost(ctx, client)
}

func (session *groupSession) revalidateHosts() error {
	for _, target := range session.planned {
		if err := session.service.revalidateHost(session.ctx, target.input.Client); err != nil {
			return err
		}
	}
	return nil
}
