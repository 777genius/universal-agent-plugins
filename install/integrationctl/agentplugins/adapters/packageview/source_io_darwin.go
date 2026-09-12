//go:build darwin && arm64

package packageview

import "runtime"

// Each phase owns a fresh goroutine/thread, never the caller's thread. Waiting
// is unconditional: cancellation must not abandon an active phase or cleanup.
func sourceIO(work func() error, abort func()) error {
	return darwinIO(work, abort, materializationGet, materializationSet)
}
func darwinIO(work func() error, abort func(), get func() (int, error), set func(int) error) error {
	type result struct {
		err        error
		panicValue any
	}
	done := make(chan result, 1)
	go func() {
		runtime.LockOSThread()
		reusable := false
		r := result{}
		defer func() {
			if p := recover(); p != nil {
				r.panicValue = p
			}
			if reusable {
				runtime.UnlockOSThread()
			}
			// Exiting locked retires this OS thread after a failed restore.
			done <- r
		}()
		previous, e := get()
		if e != nil {
			r.err = fail("platform_unavailable")
			abort()
			reusable = true
			return
		}
		// Even a failed set can have changed state. Always restore and read back.
		defer func() {
			p := recover()
			if p != nil {
				r.panicValue = p
				abort()
			}
			restoreErr := set(previous)
			current, readErr := get()
			reusable = restoreErr == nil && readErr == nil && current == previous
			if !reusable {
				r.err = fail("platform_unavailable")
				abort()
			}
		}()
		e = set(materializationOff)
		current, readErr := get()
		if e != nil || readErr != nil || current != materializationOff {
			r.err = fail("platform_unavailable")
			abort()
			return
		}
		r.err = work()
		if r.err != nil {
			abort()
		}
	}()
	r := <-done
	if r.panicValue != nil {
		panic(r.panicValue)
	}
	return r.err
}
