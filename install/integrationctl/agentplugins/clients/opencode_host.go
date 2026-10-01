package clients

import (
	"errors"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var ErrOpenCodeAdapterUnavailable = errors.New("host_adapter_unavailable")

// OpenCodeHostProfileConsumer opts the native adapter into explicit host
// preparation. The generic installer dispatches through this client capability
// rather than branching on an identity or constructing the default registry.
type OpenCodeHostProfileConsumer interface {
	UsesOpenCodeHostProfile() bool
	OwnedOpenCodeNativeRequirements([]domain.NativeObjectOwnership) (skills, config bool)
}

func OpenCodeNativeRequirements(envelope domain.PackageEnvelope) (bool, []string) {
	var transports []string
	for _, server := range envelope.MCP.Servers {
		transports = append(transports, server.Type)
	}
	slices.Sort(transports)
	return len(envelope.Skills) > 0, transports
}
