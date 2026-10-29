package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func piProfile() PlatformProfile {
	return PlatformProfile{Platform: "pi", Status: runtime.StatusRuntimeSupported, PublicPackage: "pi", InternalPackage: "pi", InternalImport: "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/pi", TransportModes: []runtime.TransportMode{runtime.ProcessMode}, RuntimeOnly: true}
}
func piEvents() []EventDescriptor {
	return []EventDescriptor{{Platform: "pi", Event: "observation", Invocation: InvocationBinding{Kind: runtime.InvocationArgvCommand, Name: "pi-event"}, Carrier: runtime.CarrierStdinJSON,
		Contract: ContractMeta{Maturity: runtime.MaturityStable}, DecodeFunc: "DecodeObservation", EncodeFunc: "EncodeObservation",
		Registrar: RegistrarMeta{MethodName: "OnObservation", EventType: "*ObservationEvent", ResponseType: "*ObservationResponse", WrapFunc: "wrapObservation"},
		Docs:      DocsMeta{SnippetKey: "pi-observation", TableGroup: "pi", Summary: "Pi settled and UI prompt notification observations (neutral output)"}, Capabilities: []runtime.CapabilityID{"pi_observation"},
	}}
}
