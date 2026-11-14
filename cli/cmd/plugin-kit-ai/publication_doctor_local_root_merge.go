package main

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
)

func mergePublicationDiagnosisLocalRootStatus(diagnosis publicationDiagnosis, localRoot *app.PluginPublicationVerifyRootResult) publicationDiagnosis {
	if diagnosis.Ready {
		diagnosis.Ready = localRoot.Ready
		if !localRoot.Ready {
			diagnosis.Status = localRoot.Status
		}
	}
	return diagnosis
}

func localRootPublicationIssues(requestedTarget string, localRoot *app.PluginPublicationVerifyRootResult) []publicationIssue {
	var issues []publicationIssue
	for _, issue := range localRoot.Issues {
		issues = append(issues, publicationIssue{
			Code:    issue.Code,
			Target:  strings.TrimSpace(requestedTarget),
			Path:    issue.Path,
			Message: issue.Message,
		})
	}
	return issues
}

func mergePublicationDiagnosisLocalRootNextSteps(nextSteps []string, localRoot *app.PluginPublicationVerifyRootResult) []string {
	if localRoot.Ready {
		return nextSteps
	}
	return appendUniqueStrings(nextSteps, localRoot.NextSteps...)
}
