//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package agentpluginscli

import (
	"os"
	"os/signal"
	"syscall"
)

// No reader goroutine: redraw consumes the notification, finish unregisters it.
// This also detects a shrink/grow cycle between two progress events.
func watchProgressResize() (changed func() bool, stop func()) {
	events := make(chan os.Signal, 1)
	signal.Notify(events, syscall.SIGWINCH)
	return func() bool {
		select {
		case <-events:
			return true
		default:
			return false
		}
	}, func() { signal.Stop(events) }
}
