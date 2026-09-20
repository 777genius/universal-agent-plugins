package app

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
	"github.com/777genius/plugin-kit-ai/cli/internal/validate"
)

func TestValidateExportServiceReportRejectsWarnings(t *testing.T) {
	t.Parallel()

	err := validateExportServiceReport(validate.Report{
		Warnings: []validate.Warning{{Kind: validate.WarningManifestUnknownField}},
	}, "claude")
	if err == nil || !strings.Contains(err.Error(), "warning(s) present") {
		t.Fatalf("error = %v", err)
	}
}

func TestRequireReadyExportProjectRejectsBlockedDiagnosis(t *testing.T) {
	t.Parallel()

	_, err := requireReadyExportProject(runtimecheck.Project{
		Runtime:    "shell",
		Entrypoint: "./bin/demo",
	})
	if err == nil || !strings.Contains(err.Error(), "runtime readiness") {
		t.Fatalf("error = %v", err)
	}
}
