package app

import (
	"errors"
	"strings"
)

func formatValidationError(prefix string, failures []ValidationFailure) error {
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString(":\n")
	for _, failure := range failures {
		b.WriteString("- ")
		b.WriteString(failure.Path)
		b.WriteString(": ")
		b.WriteString(failure.Message)
		b.WriteString("\n")
	}
	return errors.New(strings.TrimRight(b.String(), "\n"))
}
