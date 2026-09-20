package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func buildOpenCodeImportResult(state opencodeImportedState, seed ImportSeed) (ImportResult, error) {
	artifacts, err := buildOpenCodeImportArtifacts(state)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{
		Manifest:  seed.Manifest,
		Launcher:  nil,
		Artifacts: compactArtifacts(artifacts),
		Warnings:  state.warnings,
	}, nil
}

func buildOpenCodeImportArtifacts(state opencodeImportedState) ([]pluginmodel.Artifact, error) {
	artifacts := buildOpenCodeBaseImportArtifacts(state)
	var err error
	artifacts, err = appendOpenCodeOptionalImportArtifacts(artifacts, state)
	if err != nil {
		return nil, err
	}
	artifacts = appendOpenCodeImportedArtifacts(artifacts, state)
	return artifacts, nil
}
