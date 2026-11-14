package codex

import (
	"context"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (a Adapter) materializeAuthoredCodexSource(ctx context.Context, manifest domain.IntegrationManifest, sourceRoot, destRoot string) error {
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return domain.NewError(domain.ErrMutationApply, "prepare Codex materialized package root", err)
	}
	doc, err := a.buildAuthoredCodexManifest(ctx, manifest, sourceRoot, destRoot)
	if err != nil {
		return err
	}
	return writeCodexPluginManifest(destRoot, doc)
}
