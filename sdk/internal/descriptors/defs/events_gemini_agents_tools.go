package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func geminiAgentToolEvents() []EventDescriptor {
	return []EventDescriptor{
		{
			Platform: "gemini",
			Event:    "BeforeAgent",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "GeminiBeforeAgent",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
			},
			DecodeFunc: "DecodeBeforeAgent",
			EncodeFunc: "EncodeBeforeAgent",
			Registrar: RegistrarMeta{
				MethodName:   "OnBeforeAgent",
				EventType:    "*BeforeAgentEvent",
				ResponseType: "*BeforeAgentResponse",
				WrapFunc:     "wrapBeforeAgent",
			},
			Docs: DocsMeta{
				SnippetKey: "gemini-beforeagent",
				TableGroup: "gemini",
				Summary:    "Gemini BeforeAgent hook",
			},
			Capabilities: []runtime.CapabilityID{"gemini_before_agent"},
		},
		{
			Platform: "gemini",
			Event:    "AfterAgent",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "GeminiAfterAgent",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
			},
			DecodeFunc: "DecodeAfterAgent",
			EncodeFunc: "EncodeAfterAgent",
			Registrar: RegistrarMeta{
				MethodName:   "OnAfterAgent",
				EventType:    "*AfterAgentEvent",
				ResponseType: "*AfterAgentResponse",
				WrapFunc:     "wrapAfterAgent",
			},
			Docs: DocsMeta{
				SnippetKey: "gemini-afteragent",
				TableGroup: "gemini",
				Summary:    "Gemini AfterAgent hook",
			},
			Capabilities: []runtime.CapabilityID{"gemini_after_agent"},
		},
		{
			Platform: "gemini",
			Event:    "BeforeTool",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "GeminiBeforeTool",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
			},
			DecodeFunc: "DecodeBeforeTool",
			EncodeFunc: "EncodeBeforeTool",
			Registrar: RegistrarMeta{
				MethodName:   "OnBeforeTool",
				EventType:    "*BeforeToolEvent",
				ResponseType: "*BeforeToolResponse",
				WrapFunc:     "wrapBeforeTool",
			},
			Docs: DocsMeta{
				SnippetKey: "gemini-beforetool",
				TableGroup: "gemini",
				Summary:    "Gemini BeforeTool hook",
			},
			Capabilities: []runtime.CapabilityID{"gemini_before_tool"},
		},
		{
			Platform: "gemini",
			Event:    "AfterTool",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "GeminiAfterTool",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
			},
			DecodeFunc: "DecodeAfterTool",
			EncodeFunc: "EncodeAfterTool",
			Registrar: RegistrarMeta{
				MethodName:   "OnAfterTool",
				EventType:    "*AfterToolEvent",
				ResponseType: "*AfterToolResponse",
				WrapFunc:     "wrapAfterTool",
			},
			Docs: DocsMeta{
				SnippetKey: "gemini-aftertool",
				TableGroup: "gemini",
				Summary:    "Gemini AfterTool hook",
			},
			Capabilities: []runtime.CapabilityID{"gemini_after_tool"},
		},
	}
}
