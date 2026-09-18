//go:build windows

package kiro

import (
	"fmt"
	"io"
)

func queuedACPRealPipeEvidence(io.Reader) (queued, eof bool, err error) {
	// Automatic Kiro ACP activation is rejected by the Windows containment
	// capability preflight. Keep this lower-level boundary fail-closed as well:
	// Windows pipe handles need PeekNamedPipe/overlapped status semantics before
	// an expired deadline can ever be accepted as a quiet settlement window.
	return false, false, fmt.Errorf("safe ACP pipe settlement inspection is unavailable on Windows")
}
