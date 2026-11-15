package ports

// ProcessClosureError means a started client process has not been proved fully
// stopped, reaped, and released together with its supervised descendants.
// Callers must not continue effects or report an ordinary closed probe failure.
// A command that failed but was fully cleaned up does not use this error.
type ProcessClosureError struct {
	Cause error
}

func (err *ProcessClosureError) Error() string {
	if err.Cause == nil {
		return "client process closure is unproved"
	}
	return "client process closure is unproved: " + err.Cause.Error()
}

func (err *ProcessClosureError) Unwrap() error { return err.Cause }
