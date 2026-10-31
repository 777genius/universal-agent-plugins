package gen

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func piSupportEntries() []runtime.SupportEntry {
	return []runtime.SupportEntry{
		{
			Platform:       "pi",
			Event:          "observation",
			Status:         "runtime_supported",
			Maturity:       "stable",
			V1Target:       false,
			InvocationKind: "argv_command",
			Carrier:        runtime.CarrierStdinJSON,
			TransportModes: []runtime.TransportMode{
				"process",
			},
			ScaffoldSupport: false,
			ValidateSupport: false,
			Capabilities: []runtime.CapabilityID{
				"pi_observation",
			},
			Summary:         "Pi settled and UI prompt notification observations (neutral output)",
			LiveTestProfile: "",
		},
	}
}
