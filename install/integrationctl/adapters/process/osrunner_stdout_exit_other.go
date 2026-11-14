//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package process

func isStdoutLimitPipeExit(error) bool { return false }
