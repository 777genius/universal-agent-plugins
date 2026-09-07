//go:build darwin && arm64 && !cgo

package packageview

// No source resolution is allowed when the real libc binding is absent.
const materializationOff = 0

func materializationGet() (int, error) { return 0, fail("platform_unavailable") }
func materializationSet(int) error     { return fail("platform_unavailable") }

const darwinCGO = false
