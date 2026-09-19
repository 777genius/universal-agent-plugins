package pathpolicy

// Policy is the one implementation of the agentplugins PathPolicy port. It adds
// no logic of its own: every method forwards to the package function that
// already carries the containment rules.
type Policy struct{}

func (Policy) ValidateLeafID(value string) error { return ValidateLeafID(value) }

func (Policy) RequireContainedChild(base, candidate string) error {
	return RequireContainedChild(base, candidate)
}

func (Policy) RequireExactPath(expected, candidate string) error {
	return RequireExactPath(expected, candidate)
}
