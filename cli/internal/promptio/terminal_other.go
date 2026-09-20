//go:build !darwin

package promptio

import "golang.org/x/term"

// SnapshotTerminal returns a restore callback for the current terminal state.
// Call it after this owner's input readers and cancellation callbacks have joined.
func SnapshotTerminal(fd int) (func() error, error) {
	state, err := term.GetState(fd)
	if err != nil {
		return nil, err
	}
	return func() error { return term.Restore(fd, state) }, nil
}
