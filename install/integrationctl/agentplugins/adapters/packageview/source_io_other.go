//go:build !darwin || !arm64

package packageview

func sourceIO(work func() error, abort func()) error { return work() }
