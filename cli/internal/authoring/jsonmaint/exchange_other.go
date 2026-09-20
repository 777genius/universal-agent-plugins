//go:build !linux && !darwin

package jsonmaint

import (
	"errors"
	"os"
)

func exchange(_ *os.File, _, _ string) error {
	return errors.New("atomic file exchange is unavailable")
}
