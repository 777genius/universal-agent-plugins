//go:build !darwin

// Package terminalreader owns cancellable reads without owning caller handles.
package terminalreader

import (
	"os"

	"github.com/muesli/cancelreader"
)

func New(f *os.File) (cancelreader.CancelReader, error) { return cancelreader.NewReader(f) }
