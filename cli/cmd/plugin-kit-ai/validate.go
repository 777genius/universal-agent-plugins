package main

import (
	"errors"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/validate"
	"github.com/spf13/cobra"
)

type validateRunner func(root, platform string) (validate.Report, error)

var validateCmd = newValidateCmd(validate.Validate)

type validateJSONReport struct {
	validate.Report
	Format            string                  `json:"format"`
	SchemaVersion     int                     `json:"schema_version"`
	RequestedPlatform string                  `json:"requested_platform,omitempty"`
	Outcome           string                  `json:"outcome"`
	OK                bool                    `json:"ok"`
	StrictMode        bool                    `json:"strict_mode"`
	StrictFailed      bool                    `json:"strict_failed"`
	WarningCount      int                     `json:"warning_count"`
	FailureCount      int                     `json:"failure_count"`
	Publication       *publicationmodel.Model `json:"publication,omitempty"`
}

func newValidateCmd(run validateRunner) *cobra.Command {
	var platform string
	var strict bool
	var format string

	cmd := &cobra.Command{
		Use:   "validate [path]",
		Short: "Validate a package-standard plugin-kit-ai project",
		Long: `Validate a package-standard plugin-kit-ai project.

Text mode is the human-readable default and prints Warning:/Failure: lines.
Use --format json for CI or automation. That mode emits the versioned
"plugin-kit-ai/validate-report" contract with schema_version=1 and an
explicit outcome of "passed", "failed", or "failed_strict_warnings".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := run(args[0], platform)
			var reportErr *validate.ReportError
			hasReportErr := err != nil && errors.As(err, &reportErr)
			if hasReportErr {
				report = reportErr.Report
			}
			report = normalizeValidateReport(report)
			publication := discoverValidatePublication(args[0], platform)

			switch format {
			case "json":
				return runValidateJSONOutput(cmd, report, platform, strict, err, publication)
			default:
				return runValidateTextOutput(cmd, args[0], report, strict, err, hasReportErr, publication)
			}
		},
	}

	cmd.Flags().StringVar(&platform, "platform", "", `target override ("codex-package", "codex-runtime", "claude", "gemini", "opencode", "cursor", or "cursor-workspace")`)
	cmd.Flags().BoolVar(&strict, "strict", false, "treat validation warnings as errors")
	cmd.Flags().StringVar(&format, "format", "text", `output format ("text" or "json")`)
	return cmd
}
