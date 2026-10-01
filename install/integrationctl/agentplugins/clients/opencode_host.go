package clients

import (
	"errors"
	"fmt"
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
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

// PlannedOpenCodeNativeRequirements validates only selected effects. Historical
// unsupported components remain available to the selection/removal policy, but
// must not become desired native capability requirements or rendered objects.
func PlannedOpenCodeNativeRequirements(envelope domain.PackageEnvelope, plan domain.DeliveryPlan) (bool, []string, error) {
	var skills bool
	var transports []string
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		switch component.Kind {
		case domain.ComponentSkill:
			skills = true
		case domain.ComponentMCPServer:
			server, ok := envelope.MCP.Servers[component.Name]
			if !ok {
				return false, nil, fmt.Errorf("planned OpenCode MCP server %q is missing", component.Name)
			}
			transports = append(transports, server.Type)
		}
	}
	slices.Sort(transports)
	return skills, transports, nil
}

// DesiredOpenCodeCodec reads only the immutable prepared profile. Missing or
// unknown authority never selects a default dialect for desired effects.
func DesiredOpenCodeCodec(host domain.OpenCodeHostAuthority) (nativeconfig.Codec, error) {
	if host == nil {
		return "", fmt.Errorf("OpenCode prepared profile is required")
	}
	snapshot, ok := host.(interface{ ConfigDialect() string })
	if !ok {
		return "", fmt.Errorf("OpenCode prepared profile is unavailable")
	}
	if err := host.ValidateNative(false, nil); err != nil {
		return "", err
	}
	return nativeconfig.OpenCodeCodecForDialect(snapshot.ConfigDialect())
}
