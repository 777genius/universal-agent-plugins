package gemini

// OnAfterAgent registers a handler for the gemini AfterAgent.
func (r *Registrar) OnAfterAgent(fn func(*AfterAgentEvent) *AfterAgentResponse) {
	r.backend.Register("gemini", "AfterAgent", wrapAfterAgent(fn))
}

// OnAfterModel registers a handler for the gemini AfterModel.
func (r *Registrar) OnAfterModel(fn func(*AfterModelEvent) *AfterModelResponse) {
	r.backend.Register("gemini", "AfterModel", wrapAfterModel(fn))
}

// OnAfterTool registers a handler for the gemini AfterTool.
func (r *Registrar) OnAfterTool(fn func(*AfterToolEvent) *AfterToolResponse) {
	r.backend.Register("gemini", "AfterTool", wrapAfterTool(fn))
}

// OnBeforeAgent registers a handler for the gemini BeforeAgent.
func (r *Registrar) OnBeforeAgent(fn func(*BeforeAgentEvent) *BeforeAgentResponse) {
	r.backend.Register("gemini", "BeforeAgent", wrapBeforeAgent(fn))
}

// OnBeforeModel registers a handler for the gemini BeforeModel.
func (r *Registrar) OnBeforeModel(fn func(*BeforeModelEvent) *BeforeModelResponse) {
	r.backend.Register("gemini", "BeforeModel", wrapBeforeModel(fn))
}

// OnBeforeTool registers a handler for the gemini BeforeTool.
func (r *Registrar) OnBeforeTool(fn func(*BeforeToolEvent) *BeforeToolResponse) {
	r.backend.Register("gemini", "BeforeTool", wrapBeforeTool(fn))
}

// OnBeforeToolSelection registers a handler for the gemini BeforeToolSelection.
func (r *Registrar) OnBeforeToolSelection(fn func(*BeforeToolSelectionEvent) *BeforeToolSelectionResponse) {
	r.backend.Register("gemini", "BeforeToolSelection", wrapBeforeToolSelection(fn))
}

// OnNotification registers a handler for the gemini Notification.
func (r *Registrar) OnNotification(fn func(*NotificationEvent) *NotificationResponse) {
	r.backend.Register("gemini", "Notification", wrapNotification(fn))
}

// OnSessionEnd registers a handler for the gemini SessionEnd.
func (r *Registrar) OnSessionEnd(fn func(*SessionEndEvent) *SessionEndResponse) {
	r.backend.Register("gemini", "SessionEnd", wrapSessionEnd(fn))
}

// OnSessionStart registers a handler for the gemini SessionStart.
func (r *Registrar) OnSessionStart(fn func(*SessionStartEvent) *SessionStartResponse) {
	r.backend.Register("gemini", "SessionStart", wrapSessionStart(fn))
}
