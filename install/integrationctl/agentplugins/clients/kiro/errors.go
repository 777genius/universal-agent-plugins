package kiro

import (
	"errors"
	"fmt"
)

// Product-name error text starts with "Kiro" and is pinned by lifecycle goldens.
// Constructing those strings through these helpers keeps ST1005 off the call
// site without rewriting the user-visible wording.

func kiroError(msg string) error {
	return errors.New(msg)
}

func kiroErrorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}
