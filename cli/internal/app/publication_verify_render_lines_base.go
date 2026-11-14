package app

import (
	"fmt"
	"path/filepath"
)

func basePublicationVerifyRootLines(ctx publicationContext, plan publicationVerifyPlan) []string {
	return []string{
		fmt.Sprintf("Local marketplace root: %s", filepath.Clean(ctx.dest)),
		fmt.Sprintf("Package root: %s", ctx.packageRoot),
		fmt.Sprintf("Catalog artifact: %s", plan.catalogRel),
	}
}
