package app

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func requirePublicationTargetModel(ctx publicationContext) (publicationmodel.Model, error) {
	publication := ctx.inspection.Publication
	if _, ok := publicationPackageForTarget(publication, ctx.target); !ok {
		return publicationmodel.Model{}, fmt.Errorf("target %s is not publication-capable", ctx.target)
	}
	return publication, nil
}
