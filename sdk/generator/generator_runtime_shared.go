package generator

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func carrierExpr(c runtime.CarrierKind) string {
	switch c {
	case runtime.CarrierStdinJSON:
		return "runtime.CarrierStdinJSON"
	case runtime.CarrierArgvJSON:
		return "runtime.CarrierArgvJSON"
	default:
		panic("unsupported carrier")
	}
}

func internalAlias(platform runtime.PlatformID) string {
	return "internal_" + strings.ReplaceAll(string(platform), "-", "_")
}

func joinCapabilities(in []runtime.CapabilityID) string {
	out := make([]string, 0, len(in))
	for _, cap := range in {
		out = append(out, string(cap))
	}
	return strings.Join(out, ", ")
}

func joinTransportModes(in []runtime.TransportMode) string {
	out := make([]string, 0, len(in))
	for _, mode := range in {
		out = append(out, string(mode))
	}
	return strings.Join(out, ", ")
}
