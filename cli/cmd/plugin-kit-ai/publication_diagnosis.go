package main

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

type publicationDiagnosis struct {
	Ready                 bool
	Status                string
	Lines                 []string
	NextSteps             []string
	MissingPackageTargets []string
	Issues                []publicationIssue
}

type publicationIssue struct {
	Code          string `json:"code"`
	Target        string `json:"target,omitempty"`
	ChannelFamily string `json:"channel_family,omitempty"`
	Path          string `json:"path,omitempty"`
	Message       string `json:"message"`
}

func diagnosePublication(root, requestedTarget string, report pluginmanifest.Inspection) publicationDiagnosis {
	lines, channelTargets := buildPublicationDiagnosisLines(report.Publication)
	if len(report.Publication.Packages) == 0 {
		return inactivePublicationDiagnosis(lines)
	}

	missing := missingPublicationChannelPackages(report.Publication.Packages, channelTargets)
	artifactIssues := diagnosePublicationArtifacts(root, requestedTarget, report.Publication)
	repositoryIssues, repositoryNext := diagnoseGeminiRepositoryIssues(root, report.Publication)
	return finalizePublicationDiagnosis(lines, report.Layout.AuthoredRoot, report.Publication, missing, artifactIssues, repositoryIssues, repositoryNext)
}
