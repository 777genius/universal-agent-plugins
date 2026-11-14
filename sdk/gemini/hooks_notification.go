package gemini

import (
	internalgemini "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/gemini"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// NotificationType is a native Gemini notification subtype. Unknown future
// values are decoded unchanged; consumers can ignore subtypes they do not use.
// This API is public-beta.
type NotificationType = internalgemini.NotificationType

const NotificationTypeToolPermission NotificationType = internalgemini.NotificationTypeToolPermission

// NotificationEvent is the native Gemini Notification input (public-beta).
// Details retains arbitrary native detail keys and JSON values. Message and
// Details may contain sensitive agent content; filtering belongs to consumers.
type NotificationEvent = internalgemini.NotificationInput

// NotificationResponse is an advisory observer subset of Gemini's native
// NotificationOutput (public-beta). Both nil and an empty response encode as {}.
// Native suppressOutput and systemMessage options are outside this subset.
// It cannot decide tool permission or supply context.
type NotificationResponse struct{}

func wrapNotification(fn func(*NotificationEvent) *NotificationResponse) runtime.TypedHandler {
	return wrapGeminiHandler("Notification", fn, func(*NotificationResponse) any {
		return internalgemini.NotificationOutcome{}
	})
}
