package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// A retained Directory installation can exclude every new target before a
// payload is acquired. Resolve its product from the verified feed, never from
// a manifest name, so an existing unmanaged installation is still explained.
func inspectSavedDirectoryOwnership(ctx context.Context, app App, source string) error {
	if !isDirectorySelector(source) || app.Installer == nil || app.StateStore == nil || app.DirectoryClient == nil {
		return nil
	}
	state, err := app.StateStore.Load()
	if err != nil || len(state.Installations) == 0 {
		return err
	}
	bundle, err := app.DirectoryClient.Load(ctx, directoryFloorForClient(state, app.DirectoryClient))
	if err != nil {
		return err
	}
	product, err := directorySelectorProductID(bundle.Snapshot, source)
	if errors.Is(err, domain.ErrDirectoryNotFound) {
		return nil // Keep the ordinary catalog refusal for an unresolved selector.
	}
	if err != nil {
		return err
	}
	installation, found, err := retainedDirectoryInstallation(state, source, product)
	if err != nil || !found {
		return err
	}
	view, err := app.Installer.Inspect(ctx)
	if err != nil {
		return err
	}
	observed, err := environment.ObserveForEnvironment(view, app.environmentID())
	if err != nil {
		return err
	}
	for _, current := range view.Installations {
		if current.InstallationID == installation.InstallationID {
			return environment.CheckSourceOwnership(environment.LockedPackage{Alias: dependencyManifestAlias(installation.DeclaredName), SourceKey: current.SourceKey}, observed)
		}
	}
	return nil
}
