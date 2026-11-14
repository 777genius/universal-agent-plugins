package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) resolveCurrentSourceManifest(ctx context.Context, record domain.InstallationRecord) (ports.ResolvedSource, domain.IntegrationManifest, error) {
	resolved, err := s.SourceResolver.Resolve(ctx, domain.IntegrationRef{Raw: record.RequestedSourceRef.Value})
	if err != nil {
		return ports.ResolvedSource{}, domain.IntegrationManifest{}, err
	}
	manifest, err := s.ManifestLoader.Load(ctx, resolved)
	if err != nil {
		return ports.ResolvedSource{}, domain.IntegrationManifest{}, err
	}
	if manifest.IntegrationID != record.IntegrationID {
		return ports.ResolvedSource{}, domain.IntegrationManifest{}, domain.NewError(domain.ErrStateConflict, "resolved source does not match installation identity "+record.IntegrationID, nil)
	}
	return resolved, manifest, nil
}

func (s Service) resolveDesiredSourceManifest(ctx context.Context, source string) (ports.ResolvedSource, domain.IntegrationManifest, error) {
	resolved, err := s.SourceResolver.Resolve(ctx, domain.IntegrationRef{Raw: source})
	if err != nil {
		return ports.ResolvedSource{}, domain.IntegrationManifest{}, err
	}
	manifest, err := s.ManifestLoader.Load(ctx, resolved)
	if err != nil {
		cleanupResolvedSource(resolved)
		return ports.ResolvedSource{}, domain.IntegrationManifest{}, err
	}
	return resolved, manifest, nil
}
