//go:build !darwin && !linux && !windows

package directoryidentity

import (
	"fmt"
	"os"
)

func LegacyIdentity(_ string, _ os.FileInfo) (string, error) {
	return "", fmt.Errorf("durable directory identity is unsupported on this platform")
}
func canonicalize(_ string) (string, error) { return "", ErrUnsupported }

func verifyCanonicalName(_ *os.File, _ string) error { return ErrUnsupported }
