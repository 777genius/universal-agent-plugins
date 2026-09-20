package main

import "github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"

func missingPublicationChannelPackages(packages []publicationmodel.Package, channelTargets map[string]struct{}) []publicationmodel.Package {
	var missing []publicationmodel.Package
	for _, pkg := range packages {
		if _, ok := channelTargets[pkg.Target]; ok {
			continue
		}
		missing = append(missing, pkg)
	}
	return missing
}

func finalizePublicationDiagnosis(
	lines []string,
	authoredRoot string,
	model publicationmodel.Model,
	missing []publicationmodel.Package,
	artifactIssues []publicationIssue,
	repositoryIssues []publicationIssue,
	repositoryNext []string,
) publicationDiagnosis {
	switch {
	case len(missing) == 0 && len(artifactIssues) == 0 && len(repositoryIssues) == 0:
		return readyPublicationDiagnosis(lines, model)
	case len(missing) > 0:
		return missingChannelPublicationDiagnosis(lines, authoredRoot, missing)
	case len(repositoryIssues) > 0:
		return repositoryPublicationDiagnosis(lines, repositoryIssues, repositoryNext)
	default:
		return artifactPublicationDiagnosis(lines, artifactIssues)
	}
}
