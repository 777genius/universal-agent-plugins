package main

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func buildPublicationDoctorJSONReport(report pluginmanifest.Inspection, warnings []pluginmanifest.Warning, requestedTarget string, diagnosis publicationDiagnosis, localRoot *app.PluginPublicationVerifyRootResult) publicationDoctorJSONReport {
	return newPublicationDoctorJSONReport(
		publicationDoctorRequestedTarget(requestedTarget),
		publicationDoctorWarnings(warnings),
		publicationDoctorReportPublication(report),
		diagnosis,
		publicationDoctorReportLocalRoot(localRoot),
	)
}

func newPublicationDoctorJSONReport(requestedTarget string, warnings []string, publication publicationmodel.Model, diagnosis publicationDiagnosis, localRoot *app.PluginPublicationVerifyRootResult) publicationDoctorJSONReport {
	report := publicationDoctorJSONReportMetadata(requestedTarget, warnings, diagnosis)
	report.LocalRoot = localRoot
	report.Publication = publication
	return report
}

func publicationDoctorJSONReportMetadata(requestedTarget string, warnings []string, diagnosis publicationDiagnosis) publicationDoctorJSONReport {
	return publicationDoctorJSONReport{
		Format:                "plugin-kit-ai/publication-doctor-report",
		SchemaVersion:         1,
		RequestedTarget:       requestedTarget,
		Ready:                 diagnosis.Ready,
		Status:                diagnosis.Status,
		WarningCount:          len(warnings),
		Warnings:              append([]string(nil), warnings...),
		IssueCount:            len(diagnosis.Issues),
		Issues:                publicationDoctorIssues(diagnosis.Issues),
		NextSteps:             publicationDoctorNextSteps(diagnosis.NextSteps),
		MissingPackageTargets: publicationDoctorMissingTargets(diagnosis.MissingPackageTargets),
	}
}

func buildPublicationJSONReport(report pluginmanifest.Inspection, warnings []pluginmanifest.Warning, requestedTarget string) publicationJSONReport {
	return newPublicationJSONReport(
		publicationDoctorRequestedTarget(requestedTarget),
		publicationDoctorWarnings(warnings),
		publicationDoctorReportPublication(report),
	)
}

func newPublicationJSONReport(requestedTarget string, warnings []string, publication publicationmodel.Model) publicationJSONReport {
	report := publicationJSONReportMetadata(requestedTarget, warnings)
	report.Publication = publication
	return report
}

func publicationJSONReportMetadata(requestedTarget string, warnings []string) publicationJSONReport {
	return publicationJSONReport{
		Format:          "plugin-kit-ai/publication-report",
		SchemaVersion:   1,
		RequestedTarget: requestedTarget,
		WarningCount:    len(warnings),
		Warnings:        append([]string(nil), warnings...),
	}
}

func publicationDoctorRequestedTarget(requestedTarget string) string {
	return strings.TrimSpace(requestedTarget)
}

func publicationDoctorWarnings(warnings []pluginmanifest.Warning) []string {
	return warningMessages(warnings)
}

func publicationDoctorIssues(issues []publicationIssue) []publicationIssue {
	return append([]publicationIssue{}, issues...)
}

func publicationDoctorNextSteps(nextSteps []string) []string {
	return append([]string(nil), nextSteps...)
}

func publicationDoctorMissingTargets(targets []string) []string {
	return append([]string(nil), targets...)
}

func publicationDoctorReportPublication(report pluginmanifest.Inspection) publicationmodel.Model {
	return normalizePublicationModel(report.Publication)
}

func publicationDoctorReportLocalRoot(localRoot *app.PluginPublicationVerifyRootResult) *app.PluginPublicationVerifyRootResult {
	return normalizePublicationLocalRoot(localRoot)
}
