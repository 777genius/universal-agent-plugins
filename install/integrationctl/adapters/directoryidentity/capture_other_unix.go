//go:build (linux && !amd64 && !arm64) || (darwin && !arm64)

package directoryidentity

import "os"

func verifyCanonicalName(_ *os.File, _ string) error { return ErrUnsupported }
