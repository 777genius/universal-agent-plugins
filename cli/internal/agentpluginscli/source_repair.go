package agentpluginscli

import (
	"context"
	"errors"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func (app App) loadRecordedDirect(ctx context.Context, installation domain.Installation, binding domain.ClientBinding) (loadedPackage, error) {
	source := installation.Source
	revision := binding.PackageRevision
	if revision == nil || revision.TreeDigest == "" || revision.ManifestDigest == "" {
		return loadedPackage{}, errors.New("recorded direct package revision is incomplete")
	}
	direct, err := app.directAcquirer()
	if err != nil {
		return loadedPackage{}, err
	}
	var snapshot domain.PackageSnapshot
	switch source.SourceKind {
	case "git":
		snapshot, err = direct.AcquireGit(ctx, sourcespec.Git{URL: source.SourceURL, Path: source.PackageSubpath}, revision.ResolvedRevision, revision.TreeDigest)
	case "archive":
		checksum := strings.TrimPrefix(revision.ResolvedRevision, "sha256:")
		if !sourcespec.ValidSHA256(checksum) || revision.ResolvedRevision != "sha256:"+checksum {
			return loadedPackage{}, errors.New("recorded archive byte revision is invalid")
		}
		snapshot, err = direct.AcquireArchive(ctx, sourcespec.Archive{URL: source.SourceURL, Path: source.PackageSubpath}, checksum, revision.TreeDigest)
	default:
		return loadedPackage{}, errors.New("recorded source is not a typed direct package")
	}
	if err != nil {
		return loadedPackage{}, err
	}
	if snapshot.Source.CanonicalSource != source.CanonicalSource {
		_ = packagedigest.Remove(snapshot)
		return loadedPackage{}, errors.New("recorded direct source identity changed")
	}
	snapshot.Source.RequestedSource = source.RequestedSource
	loaded, err := app.loadStandardSnapshot(ctx, snapshot, domain.OriginModeDirect, nil, nil)
	if err != nil {
		return loadedPackage{}, err
	}
	if loaded.envelope.ManifestDigest != revision.ManifestDigest || loaded.envelope.TreeDigest != revision.TreeDigest {
		_ = loaded.cleanup()
		return loadedPackage{}, errors.New("recorded direct package digests disagree")
	}
	return loaded, nil
}
