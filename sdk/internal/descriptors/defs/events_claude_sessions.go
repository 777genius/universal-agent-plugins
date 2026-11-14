package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func claudeSessionEvents() []EventDescriptor {
	return []EventDescriptor{
		{
			Platform: "claude",
			Event:    "SessionStart",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "SessionStart",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodeSessionStart",
			EncodeFunc: "EncodeSessionStart",
			Registrar: RegistrarMeta{
				MethodName:   "OnSessionStart",
				EventType:    "*SessionStartEvent",
				ResponseType: "*SessionStartResponse",
				WrapFunc:     "wrapSessionStart",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-sessionstart",
				TableGroup: "claude",
				Summary:    "Claude SessionStart beta hook",
			},
			Capabilities: []runtime.CapabilityID{"session_start"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "session_start", Platform: "session_start"},
			},
		},
		{
			Platform: "claude",
			Event:    "SessionEnd",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "SessionEnd",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodeSessionEnd",
			EncodeFunc: "EncodeSessionEnd",
			Registrar: RegistrarMeta{
				MethodName:   "OnSessionEnd",
				EventType:    "*SessionEndEvent",
				ResponseType: "*SessionEndResponse",
				WrapFunc:     "wrapSessionEnd",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-sessionend",
				TableGroup: "claude",
				Summary:    "Claude SessionEnd beta hook",
			},
			Capabilities: []runtime.CapabilityID{"session_end"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "session_end", Platform: "session_end"},
			},
		},
		{
			Platform: "claude",
			Event:    "Notification",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "Notification",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodeNotification",
			EncodeFunc: "EncodeNotification",
			Registrar: RegistrarMeta{
				MethodName:   "OnNotification",
				EventType:    "*NotificationEvent",
				ResponseType: "*NotificationResponse",
				WrapFunc:     "wrapNotification",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-notification",
				TableGroup: "claude",
				Summary:    "Claude Notification beta hook",
			},
			Capabilities: []runtime.CapabilityID{"notify"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "notify", Platform: "notify"},
			},
		},
	}
}
