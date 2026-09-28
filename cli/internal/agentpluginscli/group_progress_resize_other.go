//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package agentpluginscli

// Platforms without SIGWINCH still query dimensions at every redraw.
func watchProgressResize() (func() bool, func()) { return nil, nil }
