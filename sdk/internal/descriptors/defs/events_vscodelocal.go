package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func vscodeLocalEvents() []EventDescriptor {
	return []EventDescriptor{
		vscodeLocalEvent(EventDescriptor{
			Event:      "Stop",
			Invocation: InvocationBinding{Kind: runtime.InvocationArgvCommandCaseFold, Name: "VSCodeLocalStop"},
			DecodeFunc: "DecodeStop", EncodeFunc: "EncodeObserver",
			Registrar:    RegistrarMeta{MethodName: "OnStop", EventType: "*StopEvent", ResponseType: "*StopResponse", WrapFunc: "wrapStop"},
			Docs:         DocsMeta{SnippetKey: "vscode-local-stop", Summary: "Local execution about to stop; beta neutral observer; installed integration unqualified"},
			Capabilities: []runtime.CapabilityID{"vscode_local_stop"},
		}),
		vscodeLocalEvent(EventDescriptor{
			Event:      "SubagentStop",
			Invocation: InvocationBinding{Kind: runtime.InvocationArgvCommandCaseFold, Name: "VSCodeLocalSubagentStop"},
			DecodeFunc: "DecodeSubagentStop", EncodeFunc: "EncodeObserver",
			Registrar:    RegistrarMeta{MethodName: "OnSubagentStop", EventType: "*SubagentStopEvent", ResponseType: "*SubagentStopResponse", WrapFunc: "wrapSubagentStop"},
			Docs:         DocsMeta{SnippetKey: "vscode-local-subagent-stop", Summary: "Local subagent about to stop; beta neutral observer; no root-session inference"},
			Capabilities: []runtime.CapabilityID{"vscode_local_subagent_stop"},
		}),
	}
}

func vscodeLocalEvent(e EventDescriptor) EventDescriptor {
	e.Platform = "vscode-local"
	e.Carrier = runtime.CarrierStdinJSON
	e.Contract = ContractMeta{Maturity: runtime.MaturityBeta, V1Target: false}
	e.Docs.TableGroup = "vscode-local"
	return e
}
