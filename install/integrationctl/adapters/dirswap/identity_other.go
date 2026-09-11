//go:build !darwin && !linux && !windows

package dirswap

import (
	"fmt"
	"os"
)

func directoryIdentity(path string, info os.FileInfo) (string, error) {
	return "", fmt.Errorf("durable directory identity is unsupported on this platform")
}
