//go:build !windows

package pluginkitairepo_test

import "testing"

// POSIX allows renaming a directory still held as a process working directory.
// Windows tests explicitly wait for the owned native MCP child handles to close.
func claudeWaitForOwnedProbeExit(t *testing.T, pid int) {}
