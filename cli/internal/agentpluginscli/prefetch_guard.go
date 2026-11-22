package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// prefetchMinFreeBytes is the free disk below which no further concurrent
// acquisition starts; the sequential consumer then continues alone.
const prefetchMinFreeBytes = 1 << 30

// prefetchFreeDisk is replaced in tests.
var prefetchFreeDisk = freeDiskBytes

// prefetchTraceEnv names an optional debug file that receives one JSON line per
// prefetch outcome. It is off unless set.
const prefetchTraceEnv = "AGENTPLUGINS_PREFETCH_TRACE"

var prefetchTraceMu sync.Mutex

// prefetchGuard admits concurrent acquisitions while the volume the source
// acquirer stages files on keeps prefetchMinFreeBytes free, and wires the
// debug trace. When free space cannot be measured the guard admits (fail-open):
// a failing acquisition is then reported exactly as without prefetch.
func (app App) prefetchGuard() prefetchGuard {
	root := os.TempDir()
	if staged, ok := app.SourceAcquirer.(interface{ AcquisitionTempRoot() string }); ok && staged.AcquisitionTempRoot() != "" {
		root = staged.AcquisitionTempRoot()
	}
	guard := prefetchGuard{admit: func() bool {
		free, known := prefetchFreeDisk(existingAncestor(root))
		return !known || free >= prefetchMinFreeBytes
	}}
	if path := os.Getenv(prefetchTraceEnv); path != "" {
		guard.trace = func(event prefetchEvent) { appendPrefetchTrace(path, event) }
	}
	return guard
}

func appendPrefetchTrace(path string, event prefetchEvent) {
	line, err := json.Marshal(event)
	if err != nil {
		return
	}
	prefetchTraceMu.Lock()
	defer prefetchTraceMu.Unlock()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = file.Write(append(line, '\n'))
	_ = file.Close()
}

func existingAncestor(path string) string {
	for {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return path
		}
		path = parent
	}
}

// errorClass names an error without its message, for traces.
func errorClass(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline"
	default:
		return fmt.Sprintf("%T", err)
	}
}
