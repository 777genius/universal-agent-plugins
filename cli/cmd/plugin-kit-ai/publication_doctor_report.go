package main

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/spf13/cobra"
)

type publicationDoctorJSONInput struct {
	report          pluginmanifest.Inspection
	warnings        []pluginmanifest.Warning
	requestedTarget string
	diagnosis       publicationDiagnosis
	localRoot       *app.PluginPublicationVerifyRootResult
}

func renderPublicationDoctorJSON(cmd *cobra.Command, report pluginmanifest.Inspection, warnings []pluginmanifest.Warning, requestedTarget string, diagnosis publicationDiagnosis, localRoot *app.PluginPublicationVerifyRootResult) error {
	return newPublicationDoctorJSONInput(report, warnings, requestedTarget, diagnosis, localRoot).render(cmd)
}

func writePublicationDoctorJSONReport(cmd *cobra.Command, report pluginmanifest.Inspection, warnings []pluginmanifest.Warning, requestedTarget string, diagnosis publicationDiagnosis, localRoot *app.PluginPublicationVerifyRootResult) error {
	return newPublicationDoctorJSONInput(report, warnings, requestedTarget, diagnosis, localRoot).write(cmd)
}

func renderPublicationDoctorJSONEnvelope(cmd *cobra.Command, report publicationDoctorJSONReport, diagnosis publicationDiagnosis) error {
	if err := writePublicationDoctorJSONEnvelope(cmd, report); err != nil {
		return err
	}
	return publicationDoctorJSONIssueErr(diagnosis)
}

func writePublicationDoctorJSONEnvelope(cmd *cobra.Command, report publicationDoctorJSONReport) error {
	return writePublicationDoctorJSON(cmd, report)
}

func publicationDoctorJSONEnvelope(report pluginmanifest.Inspection, warnings []pluginmanifest.Warning, requestedTarget string, diagnosis publicationDiagnosis, localRoot *app.PluginPublicationVerifyRootResult) publicationDoctorJSONReport {
	return newPublicationDoctorJSONInput(report, warnings, requestedTarget, diagnosis, localRoot).envelope()
}

func publicationDoctorJSONIssueErr(diagnosis publicationDiagnosis) error {
	return publicationDoctorIssueErr(diagnosis.Ready)
}

func newPublicationDoctorJSONInput(report pluginmanifest.Inspection, warnings []pluginmanifest.Warning, requestedTarget string, diagnosis publicationDiagnosis, localRoot *app.PluginPublicationVerifyRootResult) publicationDoctorJSONInput {
	return publicationDoctorJSONInput{
		report:          report,
		warnings:        warnings,
		requestedTarget: requestedTarget,
		diagnosis:       diagnosis,
		localRoot:       localRoot,
	}
}

func (input publicationDoctorJSONInput) envelope() publicationDoctorJSONReport {
	return buildPublicationDoctorJSONReport(input.report, input.warnings, input.requestedTarget, input.diagnosis, input.localRoot)
}

func (input publicationDoctorJSONInput) render(cmd *cobra.Command) error {
	return renderPublicationDoctorJSONEnvelope(cmd, input.envelope(), input.diagnosis)
}

func (input publicationDoctorJSONInput) write(cmd *cobra.Command) error {
	return writePublicationDoctorJSONEnvelope(cmd, input.envelope())
}
