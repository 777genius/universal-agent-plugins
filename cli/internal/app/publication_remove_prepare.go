package app

import (
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationexec"
)

type publicationRemovePlan struct {
	removedPackage      bool
	catalogRel          string
	removedCatalogEntry bool
	updatedCatalog      []byte
}

func preparePublicationRemove(ctx publicationContext) (publicationRemovePlan, error) {
	removedPackage, err := detectPublicationPackageRemoval(ctx)
	if err != nil {
		return publicationRemovePlan{}, err
	}
	catalogRel, removedCatalogEntry, updatedCatalog, err := preparePublicationCatalogRemoval(ctx)
	if err != nil {
		return publicationRemovePlan{}, err
	}
	return publicationRemovePlan{
		removedPackage:      removedPackage,
		catalogRel:          catalogRel,
		removedCatalogEntry: removedCatalogEntry,
		updatedCatalog:      updatedCatalog,
	}, nil
}

func detectPublicationPackageRemoval(ctx publicationContext) (bool, error) {
	if _, err := os.Stat(ctx.destPackageRoot()); err == nil {
		return true, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	return false, nil
}

func preparePublicationCatalogRemoval(ctx publicationContext) (string, bool, []byte, error) {
	catalogRel, err := ctx.catalogArtifactPath()
	if err != nil {
		return "", false, nil, err
	}
	catalogFull := filepath.Join(ctx.dest, filepath.FromSlash(catalogRel))
	existing, err := os.ReadFile(catalogFull)
	if os.IsNotExist(err) {
		return catalogRel, false, nil, nil
	}
	if err != nil {
		return "", false, nil, err
	}
	updated, removed, err := publicationexec.RemoveCatalogArtifact(ctx.target, existing, ctx.graph.Manifest.Name)
	if err != nil {
		return "", false, nil, err
	}
	return catalogRel, removed, updated, nil
}
