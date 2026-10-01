//go:build !darwin

// Package terminalreader owns cancellable reads without owning caller handles.
package terminalreader

import (
	"github.com/muesli/cancelreader"
	"os"
)

func New(f *os.File) (cancelreader.CancelReader, error) { return cancelreader.NewReader(f) }
