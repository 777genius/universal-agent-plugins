package qoder

import (
	internalqoder "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/qoder"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// SessionStartEvent is the Qoder SessionStart hook input (decoded from stdin
// JSON; wire uses snake_case via the platform adapter).
type SessionStartEvent = internalqoder.SessionStartInput

// SessionStartResponse is the hook outcome for SessionStart.
type SessionStartResponse = ContextResponse

func wrapSessionStart(fn func(*SessionStartEvent) *SessionStartResponse) runtime.TypedHandler {
	return wrapQoderHandler("SessionStart", fn, func(r *SessionStartResponse) any {
		return contextOutcomeFromResponse(r)
	})
}
