package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func cursorEvents() []EventDescriptor {
	return []EventDescriptor{{
		Platform: "cursor", Event: "stop",
		Invocation: InvocationBinding{Kind: runtime.InvocationArgvCommandCaseFold, Name: "CursorStop"},
		Carrier:    runtime.CarrierStdinJSON,
		Contract:   ContractMeta{Maturity: runtime.MaturityBeta, V1Target: false},
		DecodeFunc: "DecodeStop", EncodeFunc: "EncodeStop",
		Registrar:    RegistrarMeta{MethodName: "OnStop", EventType: "*StopEvent", ResponseType: "*StopResponse", WrapFunc: "wrapStop"},
		Docs:         DocsMeta{SnippetKey: "cursor-stop", TableGroup: "cursor", Summary: "Cursor stop beta observer (neutral output subset; native qualification pending)"},
		Capabilities: []runtime.CapabilityID{"cursor_stop"},
	}}
}
