//go:build !linux && !darwin && !windows

package jsonmaint

import (
	"errors"
	"os"
)

func replaceOwned(_ *os.File, _ string, _, _ string) (string, error) {
	return "", errors.New("atomic file replacement is unavailable")
}

func restoreOwned(_ *os.File, _ string, _, _ string) (string, error) {
	return "", errors.New("atomic file restoration is unavailable")
}
