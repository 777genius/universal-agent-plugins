package main

import "github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"

func publicationNextStepsForMissing(authoredRoot string, missing []publicationmodel.Package) []string {
	return buildMissingPublicationNextSteps(authoredRoot, missing)
}

func publicationNextStepsForArtifactIssues(issues []publicationIssue) []string {
	if len(issues) == 0 {
		return []string{}
	}
	return []string{
		"run plugin-kit-ai generate . to regenerate package and publication artifacts",
		"run plugin-kit-ai validate . --strict to confirm generated publication outputs are in sync",
	}
}

func publicationReadyNextSteps(model publicationmodel.Model) []string {
	return buildReadyPublicationNextSteps(model)
}

func expectedGeminiPublicationChannel(model publicationmodel.Model) (publicationmodel.Channel, bool) {
	return findPublicationChannelByFamily(model.Channels, "gemini-gallery")
}

func expectedPublicationChannel(authoredRoot, target string) (family string, path string) {
	return publicationChannelForTarget(authoredRoot, target)
}
