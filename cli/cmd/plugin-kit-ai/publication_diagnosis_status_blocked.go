package main

func repositoryPublicationDiagnosis(lines []string, issues []publicationIssue, next []string) publicationDiagnosis {
	lines = appendPublicationDiagnosisIssues(lines, issues)
	lines = append(lines, "Status: needs_repository (publication metadata is authored, but repository-rooted Gemini distribution prerequisites are missing)")
	lines = appendPublicationDiagnosisNextSteps(lines, next)
	return publicationDiagnosis{
		Ready:     false,
		Status:    "needs_repository",
		Lines:     lines,
		NextSteps: next,
		Issues:    issues,
	}
}

func artifactPublicationDiagnosis(lines []string, issues []publicationIssue) publicationDiagnosis {
	next := publicationNextStepsForArtifactIssues(issues)
	lines = appendPublicationDiagnosisIssues(lines, issues)
	lines = append(lines, "Status: needs_generate (authored publication inputs exist, but generated publication artifacts are missing)")
	lines = appendPublicationDiagnosisNextSteps(lines, next)
	return publicationDiagnosis{
		Ready:     false,
		Status:    "needs_generate",
		Lines:     lines,
		NextSteps: next,
		Issues:    issues,
	}
}
