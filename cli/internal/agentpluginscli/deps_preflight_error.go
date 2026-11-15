package agentpluginscli

import "fmt"

// Retain binding identity without exposing arbitrary native-provider diagnostics
// through the terminal and browser review error contract.
type dependencyPreflightError struct {
	Alias, ClientID string
	Cause           error
}

func (e *dependencyPreflightError) Error() string {
	return fmt.Sprintf("preflight %s/%s: %v", e.Alias, e.ClientID, e.Cause)
}

func (e *dependencyPreflightError) Unwrap() error { return e.Cause }
