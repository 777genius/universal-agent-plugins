//go:build !windows

package agentpluginscli

import (
	"errors"
	"syscall"
)

func isDiskFull(err error) bool { return errors.Is(err, syscall.ENOSPC) }

func isReadOnly(err error) bool { return errors.Is(err, syscall.EROFS) }
