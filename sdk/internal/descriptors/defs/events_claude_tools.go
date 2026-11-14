package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func claudeToolEvents() []EventDescriptor {
	return []EventDescriptor{
		{
			Platform: "claude",
			Event:    "PostToolUse",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "PostToolUse",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
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
				SnippetKey: "claude-posttooluse",
				TableGroup: "claude",
				Summary:    "Claude PostToolUse beta hook",
			},
			Capabilities: []runtime.CapabilityID{"post_tool"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "post_tool", Platform: "post_tool"},
			},
		},
		{
			Platform: "claude",
			Event:    "PostToolUseFailure",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "PostToolUseFailure",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodePostToolUseFailure",
			EncodeFunc: "EncodePostToolUseFailure",
			Registrar: RegistrarMeta{
				MethodName:   "OnPostToolUseFailure",
				EventType:    "*PostToolUseFailureEvent",
				ResponseType: "*PostToolUseFailureResponse",
				WrapFunc:     "wrapPostToolUseFailure",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-posttoolusefailure",
				TableGroup: "claude",
				Summary:    "Claude PostToolUseFailure beta hook",
			},
			Capabilities: []runtime.CapabilityID{"post_tool_failure"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "post_tool_failure", Platform: "post_tool_failure"},
			},
		},
		{
			Platform: "claude",
			Event:    "PermissionRequest",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "PermissionRequest",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodePermissionRequest",
			EncodeFunc: "EncodePermissionRequest",
			Registrar: RegistrarMeta{
				MethodName:   "OnPermissionRequest",
				EventType:    "*PermissionRequestEvent",
				ResponseType: "*PermissionRequestResponse",
				WrapFunc:     "wrapPermissionRequest",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-permissionrequest",
				TableGroup: "claude",
				Summary:    "Claude PermissionRequest beta hook",
			},
			Capabilities: []runtime.CapabilityID{"permission_request"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "permission_request", Platform: "permission_request"},
			},
		},
	}
}
