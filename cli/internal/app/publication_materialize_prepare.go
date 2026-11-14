package app

import (
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

type publicationMaterializePlan struct {
	packageFiles       []pluginmanifest.Artifact
	generated          pluginmanifest.RenderResult
	catalogArtifact    pluginmanifest.Artifact
	mergedCatalog      []byte
	packageRootAction  string
	catalogArtifactAct string
}

func preparePublicationMaterialize(ctx publicationContext) (publicationMaterializePlan, error) {
	packageFiles, generated, err := preparePublicationMaterializePackageArtifacts(ctx)
	if err != nil {
		return publicationMaterializePlan{}, err
	}
	catalogArtifact, mergedCatalog, err := preparePublicationMaterializeCatalog(ctx)
	if err != nil {
		return publicationMaterializePlan{}, err
	}
	packageRootAction, catalogAction, err := detectPublicationMaterializeActions(ctx, catalogArtifact)
	if err != nil {
		return publicationMaterializePlan{}, err
	}
	return buildPublicationMaterializePlan(packageFiles, generated, catalogArtifact, mergedCatalog, packageRootAction, catalogAction), nil
}

func detectMaterializePackageRootAction(ctx publicationContext) (string, error) {
	if info, err := os.Stat(ctx.destPackageRoot()); err == nil && info.IsDir() {
		return "replace", nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return "create", nil
}

func detectMaterializeCatalogAction(ctx publicationContext, artifact pluginmanifest.Artifact) (string, error) {
	full := filepath.Join(ctx.dest, filepath.FromSlash(artifact.RelPath))
	if _, err := os.Stat(full); err == nil {
		return "merge", nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return "create", nil
}
