//go:build windows

package process

func isStdoutLimitPipeExit(error) bool { return false }
