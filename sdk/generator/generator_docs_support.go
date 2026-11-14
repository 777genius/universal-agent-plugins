package generator

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/sdk/internal/descriptors/defs"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func renderSupportMatrixRow(m model, e defs.EventDescriptor) string {
	p := m.profiles[e.Platform]
	return fmt.Sprintf("| %s | %s | %s | %s | %s | %t | %s | %s | %s | %t | %t | %s | %s | %s |\n",
		e.Platform,
		e.Event,
		p.Status,
		e.Contract.Maturity,
		contractClass(p, e),
		e.Contract.V1Target,
		e.Invocation.Kind,
		e.Carrier,
		joinTransportModes(p.TransportModes),
		len(p.Scaffold.RequiredFiles) > 0,
		len(p.Validate.RequiredFiles) > 0,
		joinCapabilities(e.Capabilities),
		p.LiveTestProfile,
		e.Docs.Summary,
	)
}

func contractClass(p defs.PlatformProfile, e defs.EventDescriptor) string {
	if p.Status == runtime.StatusRuntimeSupported {
		switch e.Contract.Maturity {
		case runtime.MaturityStable:
			return "production-ready"
		case runtime.MaturityExperimental:
			return "public-experimental"
		default:
			return "runtime-supported but not stable"
		}
	}
	switch e.Contract.Maturity {
	case runtime.MaturityExperimental:
		return "public-experimental"
	default:
		return "public-beta"
	}
}
