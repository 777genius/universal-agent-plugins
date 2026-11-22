//go:build windows

package agentpluginscli

import (
	"fmt"
	"syscall"
	"testing"
)

// Windows reports a full disk and a write-protected medium with native error
// numbers that the portable errno constants do not match.
func TestWindowsNativeErrorNumbersAreRecognised(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		err  error
		want failureKind
	}{
		"disk full":        {fmt.Errorf("write: %w", syscall.Errno(112)), failureDiskFull},
		"handle disk full": {fmt.Errorf("write: %w", syscall.Errno(39)), failureDiskFull},
		"write protect":    {fmt.Errorf("write: %w", syscall.Errno(19)), failureReadOnly},
	} {
		if got := failureKindOf(tc.err); got != tc.want {
			t.Errorf("%s classified as %s, want %s", name, got, tc.want)
		}
	}
}
