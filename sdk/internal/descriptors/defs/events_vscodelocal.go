package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func vscodeLocalEvents() []EventDescriptor {
	return []EventDescriptor{
		{
			Platform: "vscode-local", Event: "Stop",
			Invocation: InvocationBinding{Kind: runtime.InvocationArgvCommandCaseFold, Name: "VSCodeLocalStop"},
			Carrier:    runtime.CarrierStdinJSON,
			Contract:   ContractMeta{Maturity: runtime.MaturityBeta, V1Target: false},
			DecodeFunc: "DecodeStop", EncodeFunc: "EncodeObserver",
			Registrar:    RegistrarMeta{MethodName: "OnStop", EventType: "*StopEvent", ResponseType: "*StopResponse", WrapFunc: "wrapStop"},
			Docs:         DocsMeta{SnippetKey: "vscode-local-stop", TableGroup: "vscode-local", Summary: "Local execution about to stop; beta neutral observer; installed integration unqualified"},
			Capabilities: []runtime.CapabilityID{"vscode_local_stop"},
		},
		{
			Platform: "vscode-local", Event: "SubagentStop",
			Invocation: InvocationBinding{Kind: runtime.InvocationArgvCommandCaseFold, Name: "VSCodeLocalSubagentStop"},
			Carrier:    runtime.CarrierStdinJSON,
			Contract:   ContractMeta{Maturity: runtime.MaturityBeta, V1Target: false},
			DecodeFunc: "DecodeSubagentStop", EncodeFunc: "EncodeObserver",
			Registrar:    RegistrarMeta{MethodName: "OnSubagentStop", EventType: "*SubagentStopEvent", ResponseType: "*SubagentStopResponse", WrapFunc: "wrapSubagentStop"},
			Docs:         DocsMeta{SnippetKey: "vscode-local-subagent-stop", TableGroup: "vscode-local", Summary: "Local subagent about to stop; beta neutral observer; no root-session inference"},
			Capabilities: []runtime.CapabilityID{"vscode_local_subagent_stop"},
		},
	}
}
