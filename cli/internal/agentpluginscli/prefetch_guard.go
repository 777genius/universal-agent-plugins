package agentpluginscli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// prefetchMinFreeBytes is the free disk below which no further concurrent
// acquisition starts; the sequential consumer then continues alone.
const prefetchMinFreeBytes = 1 << 30

// prefetchTraceEnv names an optional debug file that receives one JSON line per
// prefetch outcome. It is off unless set.
const prefetchTraceEnv = "AGENTPLUGINS_PREFETCH_TRACE"

var prefetchTraceMu sync.Mutex

// prefetchGuard admits concurrent acquisitions while the disk holding the
// dependency data keeps prefetchMinFreeBytes free, and wires the debug trace.
func (app App) prefetchGuard() prefetchGuard {
	root := app.DependenciesRoot
	if root == "" {
		root = os.TempDir()
	}
	guard := prefetchGuard{admit: func() bool {
		free, known := freeDiskBytes(existingAncestor(root))
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
	case errors.Is(err, syscall.ENOSPC):
		return "no_space"
	default:
		return fmt.Sprintf("%T", err)
	}
}
