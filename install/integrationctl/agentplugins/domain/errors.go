package domain

import (
	"errors"
	"fmt"
)

// ErrRetainedRevisionChange gives an existing revision refusal the observed
// empty-Directory context. It grants no replacement or migration authority.
var ErrRetainedRevisionChange = errors.New("this Directory installation has no installed targets; replacing its recorded release through add is not supported, and update requires an installed target")

type LoadError struct {
	Diagnostic Diagnostic
	Cause      error
}

func (err *LoadError) Error() string {
	if err == nil {
		return ""
	}
	if err.Cause != nil {
		return fmt.Sprintf("%s: %v", err.Diagnostic.Message, err.Cause)
	}
	return err.Diagnostic.Message
}

func (err *LoadError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

func FatalLoad(code, path, message string, cause error) error {
	return &LoadError{
		Diagnostic: Diagnostic{
			Severity: SeverityError,
			Boundary: BoundaryPlugin,
			Code:     code,
			Path:     path,
			Message:  message,
		},
		Cause: cause,
	}
}
