//go:build windows

package promptio

import (
	"runtime"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var qualificationGetProcessHandleCount = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")

// This census belongs to the same long-lived process executing ReadLine and
// next-owner reuse. The ConPTY controller is a separate, still-unmeasured owner.
func qualificationConsoleResources(t *testing.T, ceiling *[2]uint32) [2]uint32 {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last [2]uint32
	stable := 0
	for {
		var handles uint32
		ok, _, err := qualificationGetProcessHandleCount.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&handles)))
		if ok == 0 {
			t.Fatalf("GetProcessHandleCount: %v", err)
		}
		got := [2]uint32{handles, uint32(runtime.NumGoroutine())}
		if got == last && (ceiling == nil || (got[0] <= ceiling[0] && got[1] <= ceiling[1])) {
			stable++
		} else {
			stable = 0
		}
		if stable == 3 {
			return got
		}
		if time.Now().After(deadline) {
			stacks := make([]byte, 1<<20)
			n := runtime.Stack(stacks, true)
			t.Fatalf("console resources did not settle: baseline=%v actual=%v\n%s", ceiling, got, stacks[:n])
		}
		last = got
		time.Sleep(20 * time.Millisecond)
	}
}
