package copilot

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.Projector = (*Adapter)(nil)

// Project writes the managed GitHub Copilot marketplace that VS Code shares.
func (*Adapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := shared.ProjectCopilotMarketplace(in.StagingPath, in.Envelope, in.Plan); err != nil {
		return nil, err
	}
	return nil, nil
}
