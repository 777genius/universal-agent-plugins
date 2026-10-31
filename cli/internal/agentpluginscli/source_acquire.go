package agentpluginscli

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (app App) acquireDiscovery(ctx context.Context, selector string, request packageResolutionRequest) (loadedPackage, error) {
	if app.DiscoveryClient == nil || app.SourceAcquirer == nil {
		return loadedPackage{}, fmt.Errorf("signed Discovery Index dependencies are unavailable; use owner/repository@FULL_SHA[//path]")
	}
	bundle, err := app.DiscoveryClient.Load(ctx, 0)
	if err != nil {
		return loadedPackage{}, fmt.Errorf("load signed Discovery Index: %w", err)
	}
	record, err := resolveDiscoveryRecord(bundle, selector)
	if err != nil {
		return loadedPackage{}, err
	}
	for _, target := range request.Targets {
		compatible := false
		for _, client := range record.CompatibleClients {
			if string(target) == client {
				compatible = true
				break
			}
		}
		if !compatible {
			return loadedPackage{}, fmt.Errorf("discovery package %q does not declare portable components for %s", selector, target)
		}
	}
	if request.DirectBinding != nil && (request.DirectBinding.Repository != record.Repository || request.DirectBinding.PackageSubpath != record.PackagePath) {
		return loadedPackage{}, fmt.Errorf("discovery selector %q changed repository or package path; use agentplugins switch explicitly", selector)
	}
	loaded, err := app.acquireGitHub(ctx, selector, record.Repository, record.Revision, record.PackagePath, record.TreeDigest)
	if err != nil {
		return loadedPackage{}, err
	}
	if loaded.envelope.ManifestDigest != record.ManifestDigest || loaded.envelope.Manifest.Name != record.Name {
		_ = loaded.cleanup()
		return loadedPackage{}, fmt.Errorf("signed Discovery metadata does not match acquired package manifest")
	}
	// Discovery is the stable update channel; the exact commit and digests remain
	// separately bound in ResolvedRevision, TreeDigest, and ManifestDigest. If the
	// SHA URL became the canonical source, every legitimate index advance would
	// look like an unauthorized publisher/source switch.
	loaded.envelope.Source.CanonicalSource = record.Slug
	loaded.envelope.Source.SourceBindingHint = "discovery-v1"
	_, _ = fmt.Fprintf(app.errorOutput(), "WARNING [conformant_unreviewed]: %s passed static Agent Plugins 1.0 checks but has no runtime review\n", selector)
	return loaded, nil
}

func (app App) acquireLocal(ctx context.Context, requested string) (loadedPackage, error) {
	if app.SourceAcquirer == nil {
		return loadedPackage{}, fmt.Errorf("package source acquirer is unavailable")
	}
	snapshot, err := app.SourceAcquirer.AcquireLocal(ctx, requested)
	if err != nil {
		return loadedPackage{}, err
	}
	if _, err := directoryv1.ResolveDirectExact(snapshot.Source); err != nil {
		_ = packagedigest.Remove(snapshot)
		return loadedPackage{}, err
	}
	loaded, err := app.loadSnapshot(ctx, snapshot, domain.OriginModeDirect, nil, nil)
	if err == nil {
		app.warnDirectRevocation(snapshot.TreeDigest)
	}
	return loaded, err
}

func (app App) acquireGitHub(ctx context.Context, requested, repository, revision, subpath, expectedDigest string) (loadedPackage, error) {
	if app.SourceAcquirer == nil {
		return loadedPackage{}, fmt.Errorf("package source acquirer is unavailable")
	}
	if subpath == "" && expectedDigest == "" {
		loaded, err := app.acquireAutodiscoveredGitHub(ctx, requested, repository, revision)
		if err == nil {
			app.warnDirectRevocation(loaded.envelope.TreeDigest)
		}
		return loaded, err
	}
	loaded, err := app.acquireGitHubPath(ctx, requested, repository, revision, subpath, expectedDigest)
	if err == nil {
		app.warnDirectRevocation(loaded.envelope.TreeDigest)
	}
	return loaded, err
}

func (app App) acquireGitHubPath(ctx context.Context, requested, repository, revision, subpath, expectedDigest string) (loadedPackage, error) {
	var snapshot domain.PackageSnapshot
	var err error
	if expectedDigest == "" {
		snapshot, err = app.SourceAcquirer.AcquireGitHub(ctx, repository, revision, subpath)
	} else {
		snapshot, err = app.SourceAcquirer.AcquireGitHubVerified(ctx, repository, revision, subpath, expectedDigest)
	}
	if err != nil {
		return loadedPackage{}, err
	}
	snapshot.Source.RequestedSource = requested
	if expectedDigest == "" {
		if _, err := directoryv1.ResolveDirectExact(snapshot.Source); err != nil {
			_ = packagedigest.Remove(snapshot)
			return loadedPackage{}, err
		}
	}
	loaded, err := app.loadSnapshot(ctx, snapshot, domain.OriginModeDirect, nil, nil)
	return loaded, err
}

func (app App) warnDirectRevocation(treeDigest string) {
	local, ok := app.DirectoryClient.(localDirectoryReader)
	if !ok {
		return
	}
	floor := uint64(0)
	if app.StateStore != nil {
		if state, err := app.StateStore.Load(); err == nil {
			floor = installedDirectoryFloor(state)
		}
	}
	bundle, err := local.LoadLocal(floor)
	if err != nil {
		return
	}
	for _, warning := range exactDigestWarnings(bundle.Snapshot, treeDigest, "the direct source") {
		_, _ = fmt.Fprintf(app.errorOutput(), "WARNING [%s]: %s\n", warning.Code, warning.Message)
	}
}
