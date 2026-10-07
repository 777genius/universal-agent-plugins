package gen

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func cursorSupportEntries() []runtime.SupportEntry {
	return []runtime.SupportEntry{
		{
			Platform:       "cursor",
			Event:          "stop",
			Status:         "runtime_supported",
			Maturity:       "beta",
			V1Target:       false,
			InvocationKind: "argv_command_casefold",
			Carrier:        runtime.CarrierStdinJSON,
			TransportModes: []runtime.TransportMode{
				"process",
			},
			ScaffoldSupport: true,
			ValidateSupport: true,
			Capabilities: []runtime.CapabilityID{
				"cursor_stop",
			},
			Summary:         "Cursor stop beta observer (neutral output subset; native qualification pending)",
			LiveTestProfile: "cursor_stop_contract",
		},
	}
}
