package gen

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func vscodeLocalSupportEntries() []runtime.SupportEntry {
	return []runtime.SupportEntry{
		{
			Platform:       "vscode-local",
			Event:          "Stop",
			Status:         "runtime_supported",
			Maturity:       "beta",
			V1Target:       false,
			InvocationKind: "argv_command_casefold",
			Carrier:        runtime.CarrierStdinJSON,
			TransportModes: []runtime.TransportMode{
				"process",
			},
			ScaffoldSupport: false,
			ValidateSupport: false,
			Capabilities: []runtime.CapabilityID{
				"vscode_local_stop",
			},
			Summary:         "Local execution about to stop; beta neutral observer; installed integration unqualified",
			LiveTestProfile: "",
		},
		{
			Platform:       "vscode-local",
			Event:          "SubagentStop",
			Status:         "runtime_supported",
			Maturity:       "beta",
			V1Target:       false,
			InvocationKind: "argv_command_casefold",
			Carrier:        runtime.CarrierStdinJSON,
			TransportModes: []runtime.TransportMode{
				"process",
			},
			ScaffoldSupport: false,
			ValidateSupport: false,
			Capabilities: []runtime.CapabilityID{
				"vscode_local_subagent_stop",
			},
			Summary:         "Local subagent about to stop; beta neutral observer; no root-session inference",
			LiveTestProfile: "",
		},
	}
}
