package agentpluginscli

import (
	"context"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Legacy replay reconstructs an immutable acquisition URL from a recorded
// revision. That URL is never evidence that the original request was pinned.
func (app App) loadLegacyRepairPackage(ctx context.Context, installation domain.Installation, replay string, request packageResolutionRequest) (loadedPackage, error) {
	source := installation.Source
	identity := domain.SourceIdentity{
		SourceKind: source.SourceKind, SourceURL: source.SourceURL,
		RequestedSource: source.RequestedSource, CanonicalSource: source.CanonicalSource,
		Repository: source.Repository, PackageSubpath: source.PackageSubpath,
		ResolvedRevision: source.ResolvedRevision, ArchiveSHA256: source.ArchiveSHA256,
	}
	if source.Repository == "" && filepath.IsAbs(source.CanonicalSource) {
		identity.SourceBindingHint = "direct-local"
	}
	if err := app.checkEnterpriseSource(ctx, identity, installation.OriginMode); err != nil {
		return loadedPackage{}, err
	}
	loaded, err := app.loadPackageFor(ctx, replay, request)
	if err != nil {
		return loadedPackage{}, err
	}
	// Keep the acquired source coordinates and digests. Only restore the
	// historical request so the final admission sees the same pin intent.
	loaded.envelope.Source.RequestedSource = source.RequestedSource
	return loaded, nil
}
