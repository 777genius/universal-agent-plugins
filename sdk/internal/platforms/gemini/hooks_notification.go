package gemini

import (
	"encoding/json"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

type NotificationType string

const NotificationTypeToolPermission NotificationType = "ToolPermission"

type NotificationInput struct {
	BaseInput
	NotificationType NotificationType           `json:"notification_type"`
	Message          string                     `json:"message"`
	Details          map[string]json.RawMessage `json:"details"`
}

// NotificationOutcome is the observer subset of native NotificationOutput.
type NotificationOutcome struct{}

func DecodeNotification(env runtime.Envelope) (any, string, error) {
	return decodeJSONInput[NotificationInput](env, "notification", "Notification")
}

func EncodeNotification(v any) runtime.Result {
	if _, ok := v.(NotificationOutcome); !ok {
		return outcomeTypeMismatch("Gemini Notification")
	}
	return runtime.Result{ExitCode: 0, Stdout: []byte("{}")}
}
