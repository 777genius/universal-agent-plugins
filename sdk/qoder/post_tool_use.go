package qoder

import (
	"encoding/json"

	internalqoder "github.com/777genius/plugin-kit-ai/sdk/internal/platforms/qoder"
	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

// PostToolUseEvent is the Qoder PostToolUse hook input.
type PostToolUseEvent = internalqoder.PostToolUseInput

// PostToolUseResponse extends CommonResponse with tool output overrides.
type PostToolUseResponse struct {
	CommonResponse
	// AdditionalContext appends hook-specific context.
	AdditionalContext string
	// UpdatedMCPToolOutput replaces the MCP tool output payload on the wire.
	UpdatedMCPToolOutput json.RawMessage
}

func postToolUseOutcomeFromResponse(r *PostToolUseResponse) internalqoder.PostToolUseOutcome {
	if r == nil {
		return internalqoder.PostToolUseOutcome{}
	}
	return internalqoder.PostToolUseOutcome{
		CommonOutcome:        commonOutcomeFromResponse(&r.CommonResponse),
		AdditionalContext:    r.AdditionalContext,
		UpdatedMCPToolOutput: r.UpdatedMCPToolOutput,
	}
}

func wrapPostToolUse(fn func(*PostToolUseEvent) *PostToolUseResponse) runtime.TypedHandler {
	return wrapQoderHandler("PostToolUse", fn, func(r *PostToolUseResponse) any {
		return postToolUseOutcomeFromResponse(r)
	})
}
