package agentpluginscli

import (
	"context"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

type migrationLockFunc func(context.Context) (ports.UnlockFunc, error)

func (lock migrationLockFunc) Acquire(ctx context.Context) (ports.UnlockFunc, error) {
	return lock(ctx)
}

func TestMigrateExpectedRejectsMissingLockRelease(t *testing.T) {
	t.Parallel()
	t.Run("outer mutation lock", func(t *testing.T) {
		session := migrateStateSession{
			ctx: context.Background(),
			app: App{Lifecycle: usecase.Service{Lock: migrationLockFunc(func(context.Context) (ports.UnlockFunc, error) {
				return nil, nil
			})}},
		}
		if err := session.migrateExpected(); err == nil || err.Error() != "agentplugins mutation lock returned no release function" {
			t.Fatalf("migration error = %v, want a missing mutation unlock error", err)
		}
	})
	t.Run("legacy state lock", func(t *testing.T) {
		releasedOuter := false
		session := migrateStateSession{
			ctx: context.Background(),
			app: App{
				Lifecycle: usecase.Service{Lock: migrationLockFunc(func(context.Context) (ports.UnlockFunc, error) {
					return func() error {
						releasedOuter = true
						return nil
					}, nil
				})},
				LegacyStateLock: migrationLockFunc(func(context.Context) (ports.UnlockFunc, error) {
					return nil, nil
				}),
			},
		}
		if err := session.migrateExpected(); err == nil || err.Error() != "legacy state lock returned no release function" {
			t.Fatalf("migration error = %v, want a missing legacy unlock error", err)
		}
		if !releasedOuter {
			t.Fatal("outer mutation lock remained held after rejecting missing legacy release")
		}
	})
}
