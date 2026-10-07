package cursor

// OnStop registers a handler for the cursor stop.
func (r *Registrar) OnStop(fn func(*StopEvent) *StopResponse) {
	r.backend.Register("cursor", "stop", wrapStop(fn))
}
