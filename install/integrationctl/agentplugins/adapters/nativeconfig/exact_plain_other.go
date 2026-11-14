//go:build !darwin || !arm64

package nativeconfig

import "fmt"

func capturePlainOS(string) (plainExactGuard, FileSnapshot, error) {
	return nil, FileSnapshot{}, fmt.Errorf("plain exact mutation is unavailable on this platform")
}
