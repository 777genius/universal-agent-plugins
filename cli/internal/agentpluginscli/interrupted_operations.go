package agentpluginscli

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// finishInterruptedOperations performs the transactional recovery that doctor
// promises for a rerun of the intended add or remove. A run interrupted after
// its state commit leaves an open directory journal and a hidden backup. Planning
// against that leftover cannot reach the mutation path's own recovery: removal
// reports the committed binding as not installed, and add reads the backup as a
// foreign native identity or is refused as pending recovery. Dry runs never
// recover; the installer recovers the complete recorded scope or refuses it.
func finishInterruptedOperations(ctx context.Context, app App, opts *options) error {
	if app.Installer == nil || opts.dryRun {
		return nil
	}
	view, pending := interruptedOperations(ctx, app)
	if !pending {
		return nil
	}
	writeProgress(app, opts.format, "Finishing an operation interrupted in an earlier run...")
	if _, err := app.Installer.Recover(ctx, view); err != nil {
		return fmt.Errorf("finish interrupted operation: %w", err)
	}
	return nil
}

// Only open directory journals, as counted by doctor, trigger recovery. An
// inspection failure stays with the command's own fail-closed checks.
func interruptedOperations(ctx context.Context, app App) (installer.Inspection, bool) {
	open, err := app.Lifecycle.Kernel.Directory.ListOpen()
	if err != nil || len(open) == 0 {
		return installer.Inspection{}, false
	}
	view, err := app.Installer.Inspect(ctx)
	return view, err == nil && view.Recovery.Required
}
