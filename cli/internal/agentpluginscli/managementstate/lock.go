package managementstate

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
)

// Lock adapts the existing nonblocking process lock without changing its path,
// contention semantics, metadata, permissions or release ownership.
type Lock struct {
	lock processlock.Lock
}

var _ management.MutationLock = Lock{}

func NewLock(stateRoot string) (Lock, error) {
	if !filepath.IsAbs(stateRoot) {
		return Lock{}, errors.New("management requires an absolute state root")
	}
	return Lock{lock: processlock.Lock{Path: filepath.Join(stateRoot, "management.lock")}}, nil
}

func (l Lock) Acquire(ctx context.Context) (func() error, error) {
	return l.lock.Acquire(ctx)
}

func (Lock) IsActive(err error) bool {
	return errors.Is(err, processlock.ErrActive)
}
