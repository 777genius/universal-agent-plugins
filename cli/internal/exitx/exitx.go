package exitx

import "errors"

type wrapped struct {
	code int
	err  error
}

func (w *wrapped) Error() string { return w.err.Error() }
func (w *wrapped) Unwrap() error { return w.err }

// Wrap attaches a process exit code to an error for main().
func Wrap(err error, code int) error {
	if err == nil {
		return nil
	}
	return &wrapped{code: code, err: err}
}

// Code returns the exit code from Wrap, or 1 for other errors.
func Code(err error) int {
	var w *wrapped
	if errors.As(err, &w) {
		if w.code < 0 || w.code > 125 {
			return 1
		}
		return w.code
	}
	return 1
}
