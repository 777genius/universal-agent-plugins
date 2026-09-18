package gemini

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.Projector = (*Adapter)(nil)

// Project records the native objects Gemini will own after activation.
func (*Adapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	return BuildGeminiNativeObjects(in.StagingPath, in.Envelope, in.Plan, in.PluginDataPath)
}
