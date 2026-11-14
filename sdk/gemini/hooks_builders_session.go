package gemini

// SessionStartContinue returns an explicit no-op SessionStart response.
func SessionStartContinue() *SessionStartResponse {
	return &SessionStartResponse{}
}

// SessionStartAddContext appends additional context during SessionStart.
func SessionStartAddContext(context string) *SessionStartResponse {
	return &SessionStartResponse{AdditionalContext: context}
}

// SessionStartMessage emits a systemMessage during SessionStart.
func SessionStartMessage(message string) *SessionStartResponse {
	return &SessionStartResponse{CommonResponse: CommonResponse{SystemMessage: message}}
}

// SessionEndContinue returns an explicit no-op SessionEnd response.
func SessionEndContinue() *SessionEndResponse {
	return &SessionEndResponse{}
}

// SessionEndMessage emits a systemMessage during SessionEnd.
func SessionEndMessage(message string) *SessionEndResponse {
	return &SessionEndResponse{SystemMessage: message}
}
