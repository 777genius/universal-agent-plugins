package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func geminiSessionEvents() []EventDescriptor {
	return []EventDescriptor{
		{
			Platform: "gemini",
			Event:    "SessionStart",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "GeminiSessionStart",
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
				SnippetKey: "gemini-sessionstart",
				TableGroup: "gemini",
				Summary:    "Gemini SessionStart hook",
			},
			Capabilities: []runtime.CapabilityID{"gemini_session_start"},
		},
		{
			Platform: "gemini",
			Event:    "SessionEnd",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "GeminiSessionEnd",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
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
				SnippetKey: "gemini-sessionend",
				TableGroup: "gemini",
				Summary:    "Gemini SessionEnd hook",
			},
			Capabilities: []runtime.CapabilityID{"gemini_session_end"},
		},
	}
}
