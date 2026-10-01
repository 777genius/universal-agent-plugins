//go:build !linux && !darwin

package pluginkitai

import (
	"fmt"
	"os"
	"time"
)

// Synchronous inherited handles are never wrapped in an uninterruptible
// goroutine. Other hosts must provide files already supporting Go deadlines or
// inject CursorObserverIO with its own proven cancellable transport.
func prepareCursorPipe(source *os.File) (*os.File, error) {
	if source == nil {
		return nil, fmt.Errorf("Cursor observer requires owned pipe IO")
	}
	stat, err := source.Stat()
	if err == nil && stat.Mode()&os.ModeNamedPipe == 0 {
		err = fmt.Errorf("Cursor observer requires pipe IO")
	}
	if err == nil {
		err = source.SetDeadline(time.Time{})
	}
	if err != nil {
		_ = source.Close()
		return nil, err
	}
	return source, nil
}
