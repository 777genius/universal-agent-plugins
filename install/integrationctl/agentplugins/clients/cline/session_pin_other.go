//go:build !linux && !darwin && !windows

package cline

import (
	"fmt"
	"os"
)

func openSessionPinnedPath(_ string, _ bool) (*os.File, error) {
	return nil, fmt.Errorf("cline held source identity unsupported on this platform")
}
