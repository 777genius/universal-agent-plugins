package process

import "errors"

// ErrOrdinaryContainmentUnsupported identifies a missing observation primitive
// for a positively live task. It is not a generic execution or permission error.
var ErrOrdinaryContainmentUnsupported = errors.New("ordinary containment proc prerequisite is unavailable")

// OrdinaryPreflight checks process-inert launch prerequisites. On Linux this
// requires its proc children interface; success does not prove future shutdown.
// Other platforms retain their existing constructor and cleanup checks.
func (OS) OrdinaryPreflight() error { return ordinaryContainmentPreflight() }
