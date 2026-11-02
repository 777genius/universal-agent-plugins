package gen

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func qoderSupportEntries() []runtime.SupportEntry {
	return []runtime.SupportEntry{
		{
			Platform:       "qoder",
			Event:          "session_start",
			Status:         "runtime_supported",
			Maturity:       "stable",
			V1Target:       true,
			InvocationKind: "argv_command",
			Carrier:        runtime.CarrierStdinJSON,
			TransportModes: []runtime.TransportMode{
				"process",
			},
			ScaffoldSupport: true,
			ValidateSupport: true,
			Capabilities: []runtime.CapabilityID{
				"qoder_session_start",
			},
			Summary:         "Qoder SessionStart plugin hook",
			LiveTestProfile: "qoder_plugin",
		},
		{
			Platform:       "qoder",
			Event:          "user_prompt_submit",
			Status:         "runtime_supported",
			Maturity:       "stable",
			V1Target:       true,
			InvocationKind: "argv_command",
			Carrier:        runtime.CarrierStdinJSON,
			TransportModes: []runtime.TransportMode{
				"process",
			},
			ScaffoldSupport: true,
			ValidateSupport: true,
			Capabilities: []runtime.CapabilityID{
				"qoder_user_prompt_submit",
			},
			Summary:         "Qoder UserPromptSubmit plugin hook",
			LiveTestProfile: "qoder_plugin",
		},
		{
			Platform:       "qoder",
			Event:          "post_tool_use",
			Status:         "runtime_supported",
			Maturity:       "stable",
			V1Target:       true,
			InvocationKind: "argv_command",
			Carrier:        runtime.CarrierStdinJSON,
			TransportModes: []runtime.TransportMode{
				"process",
			},
			ScaffoldSupport: true,
			ValidateSupport: true,
			Capabilities: []runtime.CapabilityID{
				"qoder_post_tool_use",
			},
			Summary:         "Qoder PostToolUse plugin hook",
			LiveTestProfile: "qoder_plugin",
		},
		{
			Platform:       "qoder",
			Event:          "stop",
			Status:         "runtime_supported",
			Maturity:       "stable",
			V1Target:       true,
			InvocationKind: "argv_command",
			Carrier:        runtime.CarrierStdinJSON,
			TransportModes: []runtime.TransportMode{
				"process",
			},
			ScaffoldSupport: true,
			ValidateSupport: true,
			Capabilities: []runtime.CapabilityID{
				"qoder_stop",
			},
			Summary:         "Qoder Stop plugin hook",
			LiveTestProfile: "qoder_plugin",
		},
	}
}
