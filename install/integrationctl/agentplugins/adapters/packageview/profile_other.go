//go:build !darwin || !arm64

package packageview

import "runtime"

const ReadProfile = "packageview-local-" + runtime.GOOS + "-v1"
