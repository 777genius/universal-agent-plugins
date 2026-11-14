package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func claudeWorkspaceEvents() []EventDescriptor {
	return []EventDescriptor{
		{
			Platform: "claude",
			Event:    "PreCompact",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "PreCompact",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodePreCompact",
			EncodeFunc: "EncodePreCompact",
			Registrar: RegistrarMeta{
				MethodName:   "OnPreCompact",
				EventType:    "*PreCompactEvent",
				ResponseType: "*PreCompactResponse",
				WrapFunc:     "wrapPreCompact",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-precompact",
				TableGroup: "claude",
				Summary:    "Claude PreCompact beta hook",
			},
			Capabilities: []runtime.CapabilityID{"pre_compact"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "pre_compact", Platform: "pre_compact"},
			},
		},
		{
			Platform: "claude",
			Event:    "Setup",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "Setup",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodeSetup",
			EncodeFunc: "EncodeSetup",
			Registrar: RegistrarMeta{
				MethodName:   "OnSetup",
				EventType:    "*SetupEvent",
				ResponseType: "*SetupResponse",
				WrapFunc:     "wrapSetup",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-setup",
				TableGroup: "claude",
				Summary:    "Claude Setup beta hook",
			},
			Capabilities: []runtime.CapabilityID{"setup"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "setup", Platform: "setup"},
			},
		},
		{
			Platform: "claude",
			Event:    "TeammateIdle",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "TeammateIdle",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodeTeammateIdle",
			EncodeFunc: "EncodeTeammateIdle",
			Registrar: RegistrarMeta{
				MethodName:   "OnTeammateIdle",
				EventType:    "*TeammateIdleEvent",
				ResponseType: "*TeammateIdleResponse",
				WrapFunc:     "wrapTeammateIdle",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-teammateidle",
				TableGroup: "claude",
				Summary:    "Claude TeammateIdle beta hook",
			},
			Capabilities: []runtime.CapabilityID{"teammate_idle"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "teammate_idle", Platform: "teammate_idle"},
			},
		},
		{
			Platform: "claude",
			Event:    "TaskCompleted",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "TaskCompleted",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodeTaskCompleted",
			EncodeFunc: "EncodeTaskCompleted",
			Registrar: RegistrarMeta{
				MethodName:   "OnTaskCompleted",
				EventType:    "*TaskCompletedEvent",
				ResponseType: "*TaskCompletedResponse",
				WrapFunc:     "wrapTaskCompleted",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-taskcompleted",
				TableGroup: "claude",
				Summary:    "Claude TaskCompleted beta hook",
			},
			Capabilities: []runtime.CapabilityID{"task_completed"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "task_completed", Platform: "task_completed"},
			},
		},
		{
			Platform: "claude",
			Event:    "ConfigChange",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "ConfigChange",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodeConfigChange",
			EncodeFunc: "EncodeConfigChange",
			Registrar: RegistrarMeta{
				MethodName:   "OnConfigChange",
				EventType:    "*ConfigChangeEvent",
				ResponseType: "*ConfigChangeResponse",
				WrapFunc:     "wrapConfigChange",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-configchange",
				TableGroup: "claude",
				Summary:    "Claude ConfigChange beta hook",
			},
			Capabilities: []runtime.CapabilityID{"config_change"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "config_change", Platform: "config_change"},
			},
		},
		{
			Platform: "claude",
			Event:    "WorktreeCreate",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "WorktreeCreate",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodeWorktreeCreate",
			EncodeFunc: "EncodeWorktreeCreate",
			Registrar: RegistrarMeta{
				MethodName:   "OnWorktreeCreate",
				EventType:    "*WorktreeCreateEvent",
				ResponseType: "*WorktreeCreateResponse",
				WrapFunc:     "wrapWorktreeCreate",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-worktreecreate",
				TableGroup: "claude",
				Summary:    "Claude WorktreeCreate beta hook",
			},
			Capabilities: []runtime.CapabilityID{"worktree_create"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "worktree_create", Platform: "worktree_create"},
			},
		},
		{
			Platform: "claude",
			Event:    "WorktreeRemove",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "WorktreeRemove",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodeWorktreeRemove",
			EncodeFunc: "EncodeWorktreeRemove",
			Registrar: RegistrarMeta{
				MethodName:   "OnWorktreeRemove",
				EventType:    "*WorktreeRemoveEvent",
				ResponseType: "*WorktreeRemoveResponse",
				WrapFunc:     "wrapWorktreeRemove",
			},
			Docs: DocsMeta{
				SnippetKey: "claude-worktreeremove",
				TableGroup: "claude",
				Summary:    "Claude WorktreeRemove beta hook",
			},
			Capabilities: []runtime.CapabilityID{"worktree_remove"},
			CapabilityMappings: []CapabilityMapping{
				{Unified: "worktree_remove", Platform: "worktree_remove"},
			},
		},
	}
}
