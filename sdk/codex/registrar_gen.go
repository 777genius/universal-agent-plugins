package codex

// OnNotify registers a handler for the Codex Notify.
func (r *Registrar) OnNotify(fn func(*NotifyEvent) *Response) {
	r.backend.Register("codex", "Notify", wrapNotify(fn))
}

// OnPermissionRequest registers a handler for the Codex PermissionRequest.
func (r *Registrar) OnPermissionRequest(fn func(*PermissionRequestEvent) *Response) {
	r.backend.Register("codex", "PermissionRequest", wrapPermissionRequest(fn))
}

// OnPreToolUse registers a handler for the Codex PreToolUse.
func (r *Registrar) OnPreToolUse(fn func(*PreToolUseEvent) *Response) {
	r.backend.Register("codex", "PreToolUse", wrapPreToolUse(fn))
}

// OnStop registers a handler for the Codex Stop.
func (r *Registrar) OnStop(fn func(*StopEvent) *Response) {
	r.backend.Register("codex", "Stop", wrapStop(fn))
}

// OnSubagentStop registers a handler for the Codex SubagentStop.
func (r *Registrar) OnSubagentStop(fn func(*SubagentStopEvent) *Response) {
	r.backend.Register("codex", "SubagentStop", wrapSubagentStop(fn))
}
