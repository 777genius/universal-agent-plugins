// This independent stdin/main fixture imports only the public SDK. Its stderr
// observations are for synthetic contract tests, not a delivery/privacy policy.
package main

import (
	"encoding/json"
	"os"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/gemini"
)

func main() {
	app := pluginkitai.New(pluginkitai.Config{Name: "gemini-observer-fixture"})
	record := func(kind string, accepted bool, event any) {
		_ = json.NewEncoder(os.Stderr).Encode(struct {
			Kind     string `json:"kind"`
			Accepted bool   `json:"accepted"`
			Event    any    `json:"event"`
		}{kind, accepted, event})
	}
	app.Gemini().OnAfterAgent(func(event *gemini.AfterAgentEvent) *gemini.AfterAgentResponse {
		record("AfterAgent", true, event)
		return gemini.AfterAgentContinue()
	})
	app.Gemini().OnNotification(func(event *gemini.NotificationEvent) *gemini.NotificationResponse {
		// A consumer can ignore future subtypes without rejecting native input.
		record("Notification", event.NotificationType == gemini.NotificationTypeToolPermission, event)
		if len(os.Args) > 2 && os.Args[2] == "nil" {
			return nil
		}
		return &gemini.NotificationResponse{}
	})
	os.Exit(app.Run())
}
