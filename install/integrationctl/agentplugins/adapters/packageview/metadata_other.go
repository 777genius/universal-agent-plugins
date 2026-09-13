//go:build !darwin || !arm64

package packageview

import (
	"os"
	"runtime"
)

const ReadProfile = "packageview-local-" + runtime.GOOS + "-v1"

func sameIdentity(a, b os.FileInfo) bool { return os.SameFile(a, b) }

func scratchIdentityCheck(*source, string) error { return nil }
