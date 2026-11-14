package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func geminiModelEvents() []EventDescriptor {
	return []EventDescriptor{
		{
			Platform: "gemini",
			Event:    "BeforeModel",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "GeminiBeforeModel",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
			},
			DecodeFunc: "DecodeBeforeModel",
			EncodeFunc: "EncodeBeforeModel",
			Registrar: RegistrarMeta{
				MethodName:   "OnBeforeModel",
				EventType:    "*BeforeModelEvent",
				ResponseType: "*BeforeModelResponse",
				WrapFunc:     "wrapBeforeModel",
			},
			Docs: DocsMeta{
				SnippetKey: "gemini-beforemodel",
				TableGroup: "gemini",
				Summary:    "Gemini BeforeModel hook",
			},
			Capabilities: []runtime.CapabilityID{"gemini_before_model"},
		},
		{
			Platform: "gemini",
			Event:    "AfterModel",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "GeminiAfterModel",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
			},
			DecodeFunc: "DecodeAfterModel",
			EncodeFunc: "EncodeAfterModel",
			Registrar: RegistrarMeta{
				MethodName:   "OnAfterModel",
				EventType:    "*AfterModelEvent",
				ResponseType: "*AfterModelResponse",
				WrapFunc:     "wrapAfterModel",
			},
			Docs: DocsMeta{
				SnippetKey: "gemini-aftermodel",
				TableGroup: "gemini",
				Summary:    "Gemini AfterModel hook",
			},
			Capabilities: []runtime.CapabilityID{"gemini_after_model"},
		},
		{
			Platform: "gemini",
			Event:    "BeforeToolSelection",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "GeminiBeforeToolSelection",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
			},
			DecodeFunc: "DecodeBeforeToolSelection",
			EncodeFunc: "EncodeBeforeToolSelection",
			Registrar: RegistrarMeta{
				MethodName:   "OnBeforeToolSelection",
				EventType:    "*BeforeToolSelectionEvent",
				ResponseType: "*BeforeToolSelectionResponse",
				WrapFunc:     "wrapBeforeToolSelection",
			},
			Docs: DocsMeta{
				SnippetKey: "gemini-beforetoolselection",
				TableGroup: "gemini",
				Summary:    "Gemini BeforeToolSelection hook",
			},
			Capabilities: []runtime.CapabilityID{"gemini_before_tool_selection"},
		},
	}
}
