package agentpluginscli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (app App) loadSnapshot(ctx context.Context, snapshot domain.PackageSnapshot, origin domain.OriginMode, directory *domain.DirectoryOrigin, selection *domain.DirectorySelection) (loadedPackage, error) {
	return app.loadSnapshotMode(ctx, snapshot, origin, directory, selection, false)
}

func (app App) loadStandardSnapshot(ctx context.Context, snapshot domain.PackageSnapshot, origin domain.OriginMode, directory *domain.DirectoryOrigin, selection *domain.DirectorySelection) (loadedPackage, error) {
	return app.loadSnapshotMode(ctx, snapshot, origin, directory, selection, true)
}

func (app App) loadStandardEnvelope(ctx context.Context, input domain.LoadInput) (domain.PackageEnvelope, error) {
	if app.PackageLoader == nil {
		return domain.PackageEnvelope{}, fmt.Errorf("standard package loader is unavailable")
	}
	info, err := os.Lstat(filepath.Join(input.SnapshotRoot, "plugin.json"))
	if err != nil {
		return domain.PackageEnvelope{}, fmt.Errorf("dependency requires a root standard plugin.json: %w", err)
	}
	if !info.Mode().IsRegular() {
		return domain.PackageEnvelope{}, fmt.Errorf("dependency plugin.json must be a regular file")
	}
	envelope, err := app.PackageLoader.Load(ctx, input)
	if err != nil {
		return domain.PackageEnvelope{}, err
	}
	if envelope.FormatID != domain.FormatIDAgentPluginsV1 {
		return domain.PackageEnvelope{}, fmt.Errorf("dependency requires the standard Agent Plugins format")
	}
	return envelope, nil
}

func (app App) loadSnapshotMode(ctx context.Context, snapshot domain.PackageSnapshot, origin domain.OriginMode, directory *domain.DirectoryOrigin, selection *domain.DirectorySelection, standardOnly bool) (loadedPackage, error) {
	fail := func(err error) (loadedPackage, error) {
		_ = packagedigest.Remove(snapshot)
		return loadedPackage{}, err
	}
	load := app.loadAcquiredSnapshot
	if standardOnly {
		load = app.loadStandardEnvelope
	}
	envelope, err := load(ctx, domain.LoadInput{SnapshotRoot: snapshot.Root, TreeDigest: snapshot.TreeDigest, ExecutableFiles: snapshot.ExecutableFiles, Source: snapshot.Source})
	if err != nil {
		return fail(err)
	}
	if envelope.FormatID == domain.FormatIDAgentPluginsV1 {
		if _, err := packageDependencyFacts(envelope); err != nil {
			return fail(err)
		}
	}
	if envelope.TreeDigest != snapshot.TreeDigest {
		return fail(fmt.Errorf("loader changed the acquired immutable package digest"))
	}
	var assessment *domain.SecurityAssessment
	if app.SecurityEvaluator != nil && envelope.FormatID == domain.FormatIDAgentPluginsV1 {
		var trusted *domain.SecurityAssessment
		if app.SecurityIndex != nil {
			// The signed index is an optimization, not an availability dependency.
			// Any fetch, signature, expiry, version, or subject mismatch falls back
			// to the pinned local scanner before mutation.
			trusted, _ = app.SecurityIndex.Lookup(ctx, domain.SecuritySubject{
				TreeDigest: envelope.TreeDigest, ManifestDigest: envelope.ManifestDigest,
			})
		}
		result, securityErr := app.SecurityEvaluator.Evaluate(ctx, domain.SecurityEvaluationInput{
			SnapshotRoot: snapshot.Root, TreeDigest: envelope.TreeDigest, ManifestDigest: envelope.ManifestDigest, Trusted: trusted,
			Commands: envelope.Commands, PluginName: envelope.Manifest.Name,
		})
		if securityErr != nil {
			return fail(fmt.Errorf("security assessment failed before installation: %w", securityErr))
		}
		assessment = &result
	}
	return loadedPackage{envelope: envelope, security: assessment, origin: origin, directory: cloneDirectoryOrigin(directory), directorySelection: cloneDirectorySelection(selection), cleanup: func() error { return packagedigest.Remove(snapshot) }}, nil
}

// loadAcquiredSnapshot selects a manifest format from the immutable snapshot.
// A root plugin.json has absolute precedence over compatibility imports.
func (app App) loadAcquiredSnapshot(ctx context.Context, input domain.LoadInput) (domain.PackageEnvelope, error) {
	if app.PackageLoader == nil || app.NativePackageLoader == nil {
		return domain.PackageEnvelope{}, fmt.Errorf("package loaders are unavailable")
	}
	if _, err := os.Lstat(filepath.Join(input.SnapshotRoot, "plugin.json")); err == nil || !os.IsNotExist(err) {
		return app.PackageLoader.Load(ctx, input)
	}
	return app.NativePackageLoader.Load(ctx, input)
}
