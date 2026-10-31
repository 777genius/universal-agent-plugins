package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func qoderEvents() []EventDescriptor {
	return []EventDescriptor{
		{
			Platform: "qoder",
			Event:    "session_start",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommand,
				Name: "session-start",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
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
				SnippetKey: "qoder-session-start",
				TableGroup: "qoder",
				Summary:    "Qoder SessionStart plugin hook",
			},
			Capabilities: []runtime.CapabilityID{"qoder_session_start"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "qoder_session_start", Platform: "qoder_session_start"},
			},
		},
		{
			Platform: "qoder",
			Event:    "user_prompt_submit",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommand,
				Name: "user-prompt-submit",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
			},
			DecodeFunc: "DecodeUserPromptSubmit",
			EncodeFunc: "EncodeUserPromptSubmit",
			Registrar: RegistrarMeta{
				MethodName:   "OnUserPromptSubmit",
				EventType:    "*UserPromptSubmitEvent",
				ResponseType: "*UserPromptSubmitResponse",
				WrapFunc:     "wrapUserPromptSubmit",
			},
			Docs: DocsMeta{
				SnippetKey: "qoder-user-prompt-submit",
				TableGroup: "qoder",
				Summary:    "Qoder UserPromptSubmit plugin hook",
			},
			Capabilities: []runtime.CapabilityID{"qoder_user_prompt_submit"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "qoder_user_prompt_submit", Platform: "qoder_user_prompt_submit"},
			},
		},
		{
			Platform: "qoder",
			Event:    "post_tool_use",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommand,
				Name: "post-tool-use",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
			},
			DecodeFunc: "DecodePostToolUse",
			EncodeFunc: "EncodePostToolUse",
			Registrar: RegistrarMeta{
				MethodName:   "OnPostToolUse",
				EventType:    "*PostToolUseEvent",
				ResponseType: "*PostToolUseResponse",
				WrapFunc:     "wrapPostToolUse",
			},
			Docs: DocsMeta{
				SnippetKey: "qoder-post-tool-use",
				TableGroup: "qoder",
				Summary:    "Qoder PostToolUse plugin hook",
			},
			Capabilities: []runtime.CapabilityID{"qoder_post_tool_use"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "qoder_post_tool_use", Platform: "qoder_post_tool_use"},
			},
		},
		{
			Platform: "qoder",
			Event:    "stop",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommand,
				Name: "stop",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
			},
			DecodeFunc: "DecodeStop",
			EncodeFunc: "EncodeStop",
			Registrar: RegistrarMeta{
				MethodName:   "OnStop",
				EventType:    "*StopEvent",
				ResponseType: "*StopResponse",
				WrapFunc:     "wrapStop",
			},
			Docs: DocsMeta{
				SnippetKey: "qoder-stop",
				TableGroup: "qoder",
				Summary:    "Qoder Stop plugin hook",
			},
			Capabilities: []runtime.CapabilityID{"qoder_stop"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "qoder_stop", Platform: "qoder_stop"},
			},
		},
	}
}
