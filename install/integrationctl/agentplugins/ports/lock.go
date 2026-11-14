package ports

import "context"

type UnlockFunc func() error

// MutationLock serializes every state and managed-directory mutation across
// processes. Read-only planning deliberately does not acquire it.
type MutationLock interface {
	Acquire(context.Context) (UnlockFunc, error)
}
