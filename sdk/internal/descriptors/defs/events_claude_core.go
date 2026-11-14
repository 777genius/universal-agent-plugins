package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func claudeCoreEvents() []EventDescriptor {
	return []EventDescriptor{
		{
			Platform: "claude",
			Event:    "Stop",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "Stop",
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
				ResponseType: "*Response",
				WrapFunc:     "wrapStop",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-stop",
				TableGroup: "claude",
				Summary:    "Claude Stop command hook",
			},
			Capabilities: []runtime.CapabilityID{"stop_gate"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "stop_gate", Platform: "stop_gate"},
			},
		},
		{
			Platform: "claude",
			Event:    "PreToolUse",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "PreToolUse",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
			},
			DecodeFunc: "DecodePreToolUse",
			EncodeFunc: "EncodePreToolUse",
			Registrar: RegistrarMeta{
				MethodName:   "OnPreToolUse",
				EventType:    "*PreToolUseEvent",
				ResponseType: "*PreToolResponse",
				WrapFunc:     "wrapPreToolUse",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-pretooluse",
				TableGroup: "claude",
				Summary:    "Claude PreToolUse command hook",
			},
			Capabilities: []runtime.CapabilityID{"tool_gate"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "tool_gate", Platform: "tool_gate"},
			},
		},
		{
			Platform: "claude",
			Event:    "UserPromptSubmit",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "UserPromptSubmit",
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
				EventType:    "*UserPromptEvent",
				ResponseType: "*UserPromptResponse",
				WrapFunc:     "wrapUserPromptSubmit",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-userpromptsubmit",
				TableGroup: "claude",
				Summary:    "Claude UserPromptSubmit command hook",
			},
			Capabilities: []runtime.CapabilityID{"prompt_submit_gate"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "prompt_submit_gate", Platform: "prompt_submit_gate"},
			},
		},
	}
}
