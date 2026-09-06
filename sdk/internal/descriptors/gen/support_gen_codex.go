package gen

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func codexSupportEntries() []runtime.SupportEntry {
	return []runtime.SupportEntry{
		{
			Platform:       "codex",
			Event:          "Notify",
			Status:         "runtime_supported",
			Maturity:       "stable",
			V1Target:       true,
			InvocationKind: "argv_command_casefold",
			Carrier:        runtime.CarrierArgvJSON,
			TransportModes: []runtime.TransportMode{
				"process",
			},
			ScaffoldSupport: true,
			ValidateSupport: true,
			Capabilities: []runtime.CapabilityID{
				"notify",
			},
			Summary:         "Codex notify hook",
			LiveTestProfile: "codex_notify",
		},
		{
			Platform:       "codex",
			Event:          "Stop",
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
				"codex_stop",
			},
			Summary:         "Codex Stop lifecycle hook",
			LiveTestProfile: "codex_notify",
		},
		{
			Platform:       "codex",
			Event:          "SubagentStop",
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
				"codex_subagent_stop",
			},
			Summary:         "Codex SubagentStop lifecycle hook",
			LiveTestProfile: "codex_notify",
		},
		{
			Platform:       "codex",
			Event:          "PermissionRequest",
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
				"codex_permission_request",
			},
			Summary:         "Codex PermissionRequest hook",
			LiveTestProfile: "codex_notify",
		},
	}
}
