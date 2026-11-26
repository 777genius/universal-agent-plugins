package skillspicker

// CloseUnused atomically ends an unused browser session before terminal takeover.
// A running review check or any accepted operation retains browser ownership,
// even after the operation finishes. A scan or unaccepted review can be closed.
func (s *Server) CloseUnused() bool {
	s.mu.Lock()
	if s.preparing || s.lastOperation != "" {
		s.mu.Unlock()
		return false
	}
	// Every prepare/apply admission checks this flag under the same mutex. Set it
	// before releasing that mutex, then wait for cleanup outside the critical path.
	s.expired = true
	s.mu.Unlock()
	s.Close()
	return true
}
