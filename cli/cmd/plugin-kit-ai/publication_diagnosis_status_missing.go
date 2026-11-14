package main

import (
	"fmt"
	"slices"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func missingChannelPublicationDiagnosis(lines []string, authoredRoot string, missing []publicationmodel.Package) publicationDiagnosis {
	issues, missingTargets := missingChannelPublicationIssues(authoredRoot, missing)
	next := publicationNextStepsForMissing(authoredRoot, missing)
	lines = appendPublicationDiagnosisIssues(lines, issues)
	lines = append(lines, "Status: needs_channels (one or more publication-capable package targets have no authored publish/... channel)")
	lines = appendPublicationDiagnosisNextSteps(lines, next)
	return publicationDiagnosis{
		Ready:                 false,
		Status:                "needs_channels",
		Lines:                 lines,
		NextSteps:             next,
		MissingPackageTargets: missingTargets,
		Issues:                issues,
	}
}

func missingChannelPublicationIssues(authoredRoot string, missing []publicationmodel.Package) ([]publicationIssue, []string) {
	missingTargets := make([]string, 0, len(missing))
	issues := make([]publicationIssue, 0, len(missing))
	for _, pkg := range missing {
		missingTargets = append(missingTargets, pkg.Target)
		channelFamily, channelPath := expectedPublicationChannel(authoredRoot, pkg.Target)
		issues = append(issues, publicationIssue{
			Code:          "missing_channel",
			Target:        pkg.Target,
			ChannelFamily: channelFamily,
			Path:          channelPath,
			Message:       fmt.Sprintf("target %s requires authored %s at %s", pkg.Target, channelFamily, channelPath),
		})
	}
	slices.Sort(missingTargets)
	return issues, missingTargets
}
