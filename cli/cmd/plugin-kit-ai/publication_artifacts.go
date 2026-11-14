package main

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"slices"
)

func diagnosePublicationArtifacts(root, requestedTarget string, model publicationmodel.Model) []publicationIssue {
	issues := collectPublicationArtifactIssues(root, requestedTarget, model)
	slices.SortFunc(issues, comparePublicationIssue)
	return issues
}

func collectPublicationArtifactIssues(root, requestedTarget string, model publicationmodel.Model) []publicationIssue {
	issues := diagnoseMissingPublicationArtifacts(root, model)
	if shouldDiagnoseGeneratedPublicationArtifacts(root) {
		issues = append(issues, diagnoseGeneratedPublicationArtifacts(root, requestedTarget, model)...)
	}
	return issues
}
