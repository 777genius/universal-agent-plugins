package main

import "github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"

func inactivePublicationDiagnosis(lines []string) publicationDiagnosis {
	next := []string{
		"enable at least one package-capable target: claude, codex-package, or gemini",
	}
	issues := []publicationIssue{{
		Code:    "no_publication_targets",
		Message: "no publication-capable package targets are enabled for the requested scope",
	}}
	lines = appendPublicationDiagnosisIssues(lines, issues)
	lines = append(lines, "Status: inactive (no publication-capable package targets enabled)")
	lines = appendPublicationDiagnosisNextSteps(lines, next)
	return publicationDiagnosis{Ready: false, Status: "inactive", Lines: lines, NextSteps: next, Issues: issues}
}

func readyPublicationDiagnosis(lines []string, model publicationmodel.Model) publicationDiagnosis {
	next := publicationReadyNextSteps(model)
	lines = append(lines, "Status: ready (every publication-capable package target has an authored publication channel)")
	lines = appendPublicationDiagnosisNextSteps(lines, next)
	return publicationDiagnosis{Ready: true, Status: "ready", Lines: lines, NextSteps: next}
}
