//go:build !darwin && !linux && !windows

package agentpluginscli

// freeDiskBytes reports that free disk is unknown on this platform.
func freeDiskBytes(string) (uint64, bool) { return 0, false }
