package locks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type FileLock struct {
	BaseDir string
}

type lockInfo struct {
	Key       string `json:"key"`
	PID       int    `json:"pid"`
	StartedAt string `json:"started_at"`
}

func (l FileLock) Acquire(_ context.Context, key string) (ports.UnlockFunc, error) {
	if key == "" {
		return nil, fmt.Errorf("lock key required")
	}
	if err := os.MkdirAll(l.BaseDir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(l.BaseDir, sanitize(key)+".lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("lock already held: %s", key)
		}
		return nil, err
	}
	info := lockInfo{Key: key, PID: os.Getpid(), StartedAt: time.Now().UTC().Format(time.RFC3339)}
	body, _ := json.MarshalIndent(info, "", "  ")
	body = append(body, '\n')
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return func() error {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}, nil
}

func sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "lock"
	}
	return string(out)
}
