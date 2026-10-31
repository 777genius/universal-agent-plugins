package pi

// OnObservation registers a handler for the pi observation.
func (r *Registrar) OnObservation(fn func(*ObservationEvent) *ObservationResponse) {
	r.backend.Register("pi", "observation", wrapObservation(fn))
}
