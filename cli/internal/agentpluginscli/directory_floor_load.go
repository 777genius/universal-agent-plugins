package agentpluginscli

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
)

// loadAuthorizedDirectory respects the already-installed high-water mark for
// the selected signed protocol. A lost cache must never allow lock restore or
// candidate selection from a rolled-back publication.
func (app App) loadAuthorizedDirectory(ctx context.Context) (directoryv1.VerifiedBundle, error) {
	if app.DirectoryClient == nil {
		return directoryv1.VerifiedBundle{}, fmt.Errorf("signed Directory client is unavailable")
	}
	floor := uint64(0)
	if app.StateStore != nil {
		state, err := app.StateStore.Load()
		if err != nil {
			return directoryv1.VerifiedBundle{}, fmt.Errorf("read scoped directory receipts: %w", err)
		}
		floor = directoryFloorForClient(state, app.DirectoryClient)
	}
	return app.DirectoryClient.Load(ctx, floor)
}
