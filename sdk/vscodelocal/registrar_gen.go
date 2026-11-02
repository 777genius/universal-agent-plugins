package vscodelocal

// OnStop registers a handler for the vscode-local Stop.
func (r *Registrar) OnStop(fn func(*StopEvent) *StopResponse) {
	r.backend.Register("vscode-local", "Stop", wrapStop(fn))
}

// OnSubagentStop registers a handler for the vscode-local SubagentStop.
func (r *Registrar) OnSubagentStop(fn func(*SubagentStopEvent) *SubagentStopResponse) {
	r.backend.Register("vscode-local", "SubagentStop", wrapSubagentStop(fn))
}
