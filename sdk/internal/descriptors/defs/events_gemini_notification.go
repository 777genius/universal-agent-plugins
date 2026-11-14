package defs

import "github.com/777genius/plugin-kit-ai/sdk/internal/runtime"

func geminiNotificationEvents() []EventDescriptor {
	return []EventDescriptor{
		{
			Platform: "gemini",
			Event:    "Notification",
			Invocation: InvocationBinding{
				Kind: runtime.InvocationArgvCommandCaseFold,
				Name: "GeminiNotification",
			},
			Carrier: runtime.CarrierStdinJSON,
			Contract: ContractMeta{
				Maturity: runtime.MaturityBeta,
				V1Target: false,
			},
			DecodeFunc: "DecodeNotification",
			EncodeFunc: "EncodeNotification",
			Registrar: RegistrarMeta{
				MethodName:   "OnNotification",
				EventType:    "*NotificationEvent",
				ResponseType: "*NotificationResponse",
				WrapFunc:     "wrapNotification",
			},
			Docs: DocsMeta{
				SnippetKey: "gemini-notification",
				TableGroup: "gemini",
				Summary:    "Gemini Notification beta observer (neutral output subset; native qualification pending)",
			},
			Capabilities: []runtime.CapabilityID{"gemini_notification"},
		},
	}
}
