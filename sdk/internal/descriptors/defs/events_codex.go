package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func codexEvents() []EventDescriptor {
	return []EventDescriptor{
		{
			Platform: "codex",
			Event:    "Notify",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "notify",
			},
			Carrier: runtime.CarrierArgvJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityStable,
				V1Target: true,
			},
			DecodeFunc: "DecodeNotify",
			EncodeFunc: "EncodeNotify",
			Registrar: RegistrarMeta{
				MethodName:   "OnNotify",
				EventType:    "*NotifyEvent",
				ResponseType: "*Response",
				WrapFunc:     "wrapNotify",
			},
			Docs: DocsMeta{
				SnippetKey: "codex-notify",
				TableGroup: "codex",
				Summary:    "Codex notify hook",
			},
			Capabilities: []runtime.CapabilityID{"notify"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "notify", Platform: "notify"},
			},
		},
		{
			Platform: "codex",
			Event:    "Stop",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "CodexStop",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
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
				SnippetKey: "codex-stop",
				TableGroup: "codex",
				Summary:    "Codex Stop lifecycle hook",
			},
			Capabilities: []runtime.CapabilityID{"codex_stop"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "codex_stop", Platform: "codex_stop"},
			},
		},
		{
			Platform: "codex",
			Event:    "SubagentStop",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "CodexSubagentStop",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
			},
			DecodeFunc: "DecodeSubagentStop",
			EncodeFunc: "EncodeSubagentStop",
			Registrar: RegistrarMeta{
				MethodName:   "OnSubagentStop",
				EventType:    "*SubagentStopEvent",
				ResponseType: "*Response",
				WrapFunc:     "wrapSubagentStop",
			},
			Docs: DocsMeta{
				SnippetKey: "codex-subagentstop",
				TableGroup: "codex",
				Summary:    "Codex SubagentStop lifecycle hook",
			},
			Capabilities: []runtime.CapabilityID{"codex_subagent_stop"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "codex_subagent_stop", Platform: "codex_subagent_stop"},
			},
		},
		{
			Platform: "codex",
			Event:    "PreToolUse",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "CodexPreToolUse",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
			},
			DecodeFunc: "DecodePreToolUse",
			EncodeFunc: "EncodePreToolUse",
			Registrar: RegistrarMeta{
				MethodName:   "OnPreToolUse",
				EventType:    "*PreToolUseEvent",
				ResponseType: "*Response",
				WrapFunc:     "wrapPreToolUse",
			},
			Docs: DocsMeta{
				SnippetKey: "codex-pretooluse",
				TableGroup: "codex",
				Summary:    "Codex PreToolUse hook",
			},
			Capabilities: []runtime.CapabilityID{"codex_pre_tool_use"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "codex_pre_tool_use", Platform: "codex_pre_tool_use"},
			},
		},
		{
			Platform: "codex",
			Event:    "PermissionRequest",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "CodexPermissionRequest",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
			},
			DecodeFunc: "DecodePermissionRequest",
			EncodeFunc: "EncodePermissionRequest",
			Registrar: RegistrarMeta{
				MethodName:   "OnPermissionRequest",
				EventType:    "*PermissionRequestEvent",
				ResponseType: "*Response",
				WrapFunc:     "wrapPermissionRequest",
			},
			Docs: DocsMeta{
				SnippetKey: "codex-permissionrequest",
				TableGroup: "codex",
				Summary:    "Codex PermissionRequest hook",
			},
			Capabilities: []runtime.CapabilityID{"codex_permission_request"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "codex_permission_request", Platform: "codex_permission_request"},
			},
		},
	}
}
