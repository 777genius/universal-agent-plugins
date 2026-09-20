package main

func appendPublicationDiagnosisIssues(lines []string, issues []publicationIssue) []string {
	for _, issue := range issues {
		lines = append(lines, formatPublicationIssueLine(issue))
	}
	return lines
}

func appendPublicationDiagnosisNextSteps(lines []string, next []string) []string {
	lines = append(lines, "Next:")
	for _, step := range next {
		lines = append(lines, "  "+step)
	}
	return lines
}

func formatPublicationIssueLine(issue publicationIssue) string {
	return "Issue[" + issue.Code + "]: " + issue.Message
}
