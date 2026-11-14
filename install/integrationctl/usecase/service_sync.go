package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) updateAll(ctx context.Context, dryRun bool) (domain.Report, error) {
	return s.runUpdateAll(ctx, dryRun)
}

func (s Service) sync(ctx context.Context, dryRun bool) (domain.Report, error) {
	return s.runSync(ctx, dryRun)
}
