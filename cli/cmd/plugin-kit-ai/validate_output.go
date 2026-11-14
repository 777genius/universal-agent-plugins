package main

import (
	"encoding/json"
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/validate"
	"github.com/spf13/cobra"
)

func runValidateJSONOutput(cmd *cobra.Command, report validate.Report, platform string, strict bool, runErr error, publication *publicationmodel.Model) error {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	body, marshalErr := json.MarshalIndent(buildValidateJSONReport(report, platform, strict, runErr, publication), "", "  ")
	if marshalErr != nil {
		return marshalErr
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", body)
	if runErr != nil {
		return runErr
	}
	if len(report.Failures) > 0 {
		return &validate.ReportError{Report: report}
	}
	if strict && len(report.Warnings) > 0 {
		return fmt.Errorf("validation warnings treated as errors (%d warning(s))", len(report.Warnings))
	}
	return nil
}

func runValidateTextOutput(cmd *cobra.Command, root string, report validate.Report, strict bool, runErr error, hasReportErr bool, publication *publicationmodel.Model) error {
	for _, warning := range report.Warnings {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", warning.Message)
	}
	for _, hint := range geminiValidateWarningHints(report) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Hint: %s\n", hint)
	}
	if len(report.Failures) > 0 {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		for _, line := range validatePublicationText(publication) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
		}
		for _, failure := range report.Failures {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Failure: %s\n", failure.Message)
		}
		for _, hint := range geminiValidateFailureHints(report) {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Hint: %s\n", hint)
		}
		if hasReportErr {
			return runErr
		}
		return &validate.ReportError{Report: report}
	}
	if runErr != nil {
		return runErr
	}
	if strict && len(report.Warnings) > 0 {
		return fmt.Errorf("validation warnings treated as errors (%d warning(s))", len(report.Warnings))
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Validated %s\n", root)
	for _, line := range validatePublicationText(publication) {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
	}
	for _, hint := range geminiValidateSuccessHints(root, report) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Hint: %s\n", hint)
	}
	return nil
}

func normalizeValidateReport(report validate.Report) validate.Report {
	if report.Checks == nil {
		report.Checks = []string{}
	}
	if report.Warnings == nil {
		report.Warnings = []validate.Warning{}
	}
	if report.Failures == nil {
		report.Failures = []validate.Failure{}
	}
	return report
}

func buildValidateJSONReport(report validate.Report, requestedPlatform string, strict bool, runErr error, publication *publicationmodel.Model) validateJSONReport {
	failureCount := len(report.Failures)
	warningCount := len(report.Warnings)
	strictFailed := strict && failureCount == 0 && warningCount > 0
	ok := runErr == nil && failureCount == 0 && !strictFailed
	outcome := "passed"
	switch {
	case failureCount > 0:
		outcome = "failed"
	case strictFailed:
		outcome = "failed_strict_warnings"
	}
	return validateJSONReport{
		Report:            report,
		Format:            "plugin-kit-ai/validate-report",
		SchemaVersion:     1,
		RequestedPlatform: requestedPlatform,
		Outcome:           outcome,
		OK:                ok,
		StrictMode:        strict,
		StrictFailed:      strictFailed,
		WarningCount:      warningCount,
		FailureCount:      failureCount,
		Publication:       publication,
	}
}
