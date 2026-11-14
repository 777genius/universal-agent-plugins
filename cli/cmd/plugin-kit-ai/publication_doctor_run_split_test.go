package main

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func TestPublicationDoctorRootUsesProvidedArgument(t *testing.T) {
	t.Parallel()

	if got := publicationDoctorRoot([]string{"./demo"}); got != "./demo" {
		t.Fatalf("root = %q", got)
	}
}

func TestNewPublicationDoctorRenderInputPreservesCommandFields(t *testing.T) {
	t.Parallel()

	input := publicationDoctorInputData{format: "json", target: "gemini"}
	inspected := publicationDoctorInspectionResult{
		report:    pluginmanifest.Inspection{Manifest: pluginmanifest.Manifest{Name: "demo"}},
		warnings:  []pluginmanifest.Warning{{Message: "warning"}},
		diagnosis: publicationDiagnosis{Status: "ready"},
		localRoot: &app.PluginPublicationVerifyRootResult{Lines: []string{"local-root"}},
	}

	got := newPublicationDoctorRenderInput(input, inspected)
	if got.format != "json" || got.target != "gemini" {
		t.Fatalf("render input = %+v", got)
	}
	if got.report.Manifest.Name != "demo" || got.diagnosis.Status != "ready" || len(got.warnings) != 1 || got.localRoot == nil {
		t.Fatalf("render input = %+v", got)
	}
}
