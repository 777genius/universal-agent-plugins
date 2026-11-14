package qoder

// OnPostToolUse registers a handler for the qoder post_tool_use.
func (r *Registrar) OnPostToolUse(fn func(*PostToolUseEvent) *PostToolUseResponse) {
	r.backend.Register("qoder", "post_tool_use", wrapPostToolUse(fn))
}

// OnSessionStart registers a handler for the qoder session_start.
func (r *Registrar) OnSessionStart(fn func(*SessionStartEvent) *SessionStartResponse) {
	r.backend.Register("qoder", "session_start", wrapSessionStart(fn))
}

// OnStop registers a handler for the qoder stop.
func (r *Registrar) OnStop(fn func(*StopEvent) *StopResponse) {
	r.backend.Register("qoder", "stop", wrapStop(fn))
}

// OnUserPromptSubmit registers a handler for the qoder user_prompt_submit.
func (r *Registrar) OnUserPromptSubmit(fn func(*UserPromptSubmitEvent) *UserPromptSubmitResponse) {
	r.backend.Register("qoder", "user_prompt_submit", wrapUserPromptSubmit(fn))
}
