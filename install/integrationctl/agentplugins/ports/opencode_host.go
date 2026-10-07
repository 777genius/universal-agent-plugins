package ports

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// OpenCodeHostPreparer is trusted composition authority. Preparation may probe
// only an explicit desired-native target; revalidation fences authoritative
// effects. Dry-run and stored inspect/remove/recovery never call this port.
type OpenCodeHostPreparer interface {
	PrepareOpenCodeHost(context.Context, domain.DetectedClient, domain.PackageEnvelope) (domain.DetectedClient, error)
	RevalidateOpenCodeHost(context.Context, domain.DetectedClient) error
}
