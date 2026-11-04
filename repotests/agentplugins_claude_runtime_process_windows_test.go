//go:build windows

package pluginkitairepo_test

import (
	"errors"
	"fmt"
	"testing"

	"golang.org/x/sys/windows"
)

// Do not manipulate a managed directory while a native MCP process may still
// have it as its working directory. Observe only the PIDs returned by this test
// fixture, not unrelated host processes; never kill a caller-owned process.
func claudeWaitForOwnedProbeExit(t *testing.T, pid int) {
	t.Helper()
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return // Already exited before the observation opened a process handle.
	}
	if err != nil {
		t.Fatalf("observe native MCP process %d: %v", pid, err)
	}
	defer windows.CloseHandle(handle)
	result, err := windows.WaitForSingleObject(handle, 5000)
	if err != nil {
		t.Fatalf("wait for native MCP process %d: %v", pid, err)
	}
	if result != windows.WAIT_OBJECT_0 {
		t.Fatal(fmt.Sprintf("native MCP process %d is still alive after CLI exit (wait=%d)", pid, result))
	}
}
