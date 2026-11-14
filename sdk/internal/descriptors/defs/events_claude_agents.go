package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func claudeAgentEvents() []EventDescriptor {
	return []EventDescriptor{
		{
			Platform: "claude",
			Event:    "SubagentStart",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "SubagentStart",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodeSubagentStart",
			EncodeFunc: "EncodeSubagentStart",
			Registrar: RegistrarMeta{
				MethodName:   "OnSubagentStart",
				EventType:    "*SubagentStartEvent",
				ResponseType: "*SubagentStartResponse",
				WrapFunc:     "wrapSubagentStart",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-subagentstart",
				TableGroup: "claude",
				Summary:    "Claude SubagentStart beta hook",
			},
			Capabilities: []runtime.CapabilityID{"subagent_start"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "subagent_start", Platform: "subagent_start"},
			},
		},
		{
			Platform: "claude",
			Event:    "SubagentStop",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "SubagentStop",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodeSubagentStop",
			EncodeFunc: "EncodeSubagentStop",
			Registrar: RegistrarMeta{
				MethodName:   "OnSubagentStop",
				EventType:    "*SubagentStopEvent",
				ResponseType: "*SubagentStopResponse",
				WrapFunc:     "wrapSubagentStop",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-subagentstop",
				TableGroup: "claude",
				Summary:    "Claude SubagentStop beta hook",
			},
			Capabilities: []runtime.CapabilityID{"subagent_stop"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "subagent_stop", Platform: "subagent_stop"},
			},
		},
	}
}
