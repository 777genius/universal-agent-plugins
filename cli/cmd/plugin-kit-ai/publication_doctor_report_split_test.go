package main

import (
	"bytes"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/spf13/cobra"
)

func TestNormalizePublicationModelInitializesNilSlices(t *testing.T) {
	t.Parallel()
	model := normalizePublicationModel(publicationmodel.Model{
		Packages: []publicationmodel.Package{{}},
		Channels: []publicationmodel.Channel{{}},
	})
	if model.Packages[0].ChannelFamilies == nil || model.Packages[0].AuthoredInputs == nil || model.Packages[0].ManagedArtifacts == nil {
		t.Fatalf("package slices = %+v", model.Packages[0])
	}
	if model.Channels[0].PackageTargets == nil {
		t.Fatalf("channel slices = %+v", model.Channels[0])
	}
}

func TestWarningMessagesProjectsWarningBodies(t *testing.T) {
	t.Parallel()
	got := warningMessages([]pluginmanifest.Warning{{Message: "first"}, {Message: "second"}})
	if len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("warnings = %+v", got)
	}
}

func TestPublicationDoctorIssueErrWrapsExitCodeOne(t *testing.T) {
	t.Parallel()

	err := publicationDoctorIssueErr(false)
	if err == nil {
		t.Fatal("expected error")
	}
	if code := exitx.Code(err); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
}

func TestPublicationDoctorIssueErrSkipsReadyReports(t *testing.T) {
	t.Parallel()

	if err := publicationDoctorIssueErr(true); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestPublicationDoctorRequestedTargetTrimsWhitespace(t *testing.T) {
	t.Parallel()

	if got := publicationDoctorRequestedTarget(" gemini "); got != "gemini" {
		t.Fatalf("requested target = %q", got)
	}
}

func TestNewPublicationDoctorJSONReportCopiesSlices(t *testing.T) {
	t.Parallel()

	issues := []publicationIssue{{Code: "demo"}}
	nextSteps := []string{"step"}
	missing := []string{"gemini"}
	report := newPublicationDoctorJSONReport("gemini", []string{"warn"}, publicationmodel.Model{}, publicationDiagnosis{
		Ready:                 true,
		Status:                "ready",
		Issues:                issues,
		NextSteps:             nextSteps,
		MissingPackageTargets: missing,
	}, &app.PluginPublicationVerifyRootResult{Ready: true})
	issues[0].Code = "changed"
	nextSteps[0] = "changed"
	missing[0] = "changed"
	if report.Issues[0].Code != "demo" || report.NextSteps[0] != "step" || report.MissingPackageTargets[0] != "gemini" {
		t.Fatalf("report = %+v", report)
	}
}

func TestPublicationDoctorJSONIssueErrUsesDiagnosisReadyFlag(t *testing.T) {
	t.Parallel()

	if err := publicationDoctorJSONIssueErr(publicationDiagnosis{Ready: true}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	err := publicationDoctorJSONIssueErr(publicationDiagnosis{Ready: false})
	if code := exitx.Code(err); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
}

func TestPublicationDoctorJSONEnvelopeBuildsRequestedTarget(t *testing.T) {
	t.Parallel()

	report := publicationDoctorJSONEnvelope(pluginmanifest.Inspection{}, []pluginmanifest.Warning{{Message: "warn"}}, " gemini ", publicationDiagnosis{
		Ready:  true,
		Status: "ready",
	}, &app.PluginPublicationVerifyRootResult{Ready: true})
	if report.RequestedTarget != "gemini" || report.WarningCount != 1 || report.Status != "ready" {
		t.Fatalf("report = %+v", report)
	}
}

func TestWritePublicationDoctorJSONReportEmitsJSONEnvelope(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := writePublicationDoctorJSONReport(cmd, pluginmanifest.Inspection{}, nil, "gemini", publicationDiagnosis{Ready: true, Status: "ready"}, &app.PluginPublicationVerifyRootResult{Ready: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got == "" || got[0] != '{' {
		t.Fatalf("output = %q", got)
	}
}

func TestWritePublicationDoctorJSONEnvelopeEmitsJSONEnvelope(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := writePublicationDoctorJSONEnvelope(cmd, publicationDoctorJSONReport{
		Format:        "plugin-kit-ai/publication-doctor-report",
		SchemaVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got == "" || got[0] != '{' {
		t.Fatalf("output = %q", got)
	}
}

func TestRenderPublicationDoctorJSONEnvelopeReturnsIssueExitAfterWrite(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := renderPublicationDoctorJSONEnvelope(cmd, publicationDoctorJSONReport{
		Format:        "plugin-kit-ai/publication-doctor-report",
		SchemaVersion: 1,
	}, publicationDiagnosis{Ready: false})
	if code := exitx.Code(err); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	if got := buf.String(); got == "" || got[0] != '{' {
		t.Fatalf("output = %q", got)
	}
}

func TestNewPublicationDoctorJSONInputEnvelopeBuildsRequestedTarget(t *testing.T) {
	t.Parallel()

	report := newPublicationDoctorJSONInput(pluginmanifest.Inspection{}, []pluginmanifest.Warning{{Message: "warn"}}, " gemini ", publicationDiagnosis{
		Ready:  true,
		Status: "ready",
	}, &app.PluginPublicationVerifyRootResult{Ready: true}).envelope()
	if report.RequestedTarget != "gemini" || report.WarningCount != 1 || report.Status != "ready" {
		t.Fatalf("report = %+v", report)
	}
}

func TestPublicationDoctorJSONInputWriteEmitsJSONEnvelope(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := newPublicationDoctorJSONInput(pluginmanifest.Inspection{}, nil, "", publicationDiagnosis{Ready: true}, nil).write(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got == "" || got[0] != '{' {
		t.Fatalf("output = %q", got)
	}
}

func TestNewPublicationJSONReportCopiesWarnings(t *testing.T) {
	t.Parallel()

	warnings := []string{"warn"}
	report := newPublicationJSONReport("gemini", warnings, publicationmodel.Model{})
	warnings[0] = "changed"
	if report.RequestedTarget != "gemini" || report.WarningCount != 1 || report.Warnings[0] != "warn" {
		t.Fatalf("report = %+v", report)
	}
}

func TestBuildPublicationJSONReportBuildsTrimmedTarget(t *testing.T) {
	t.Parallel()

	report := buildPublicationJSONReport(pluginmanifest.Inspection{}, []pluginmanifest.Warning{{Message: "warn"}}, " gemini ")
	if report.RequestedTarget != "gemini" || report.WarningCount != 1 || report.Warnings[0] != "warn" {
		t.Fatalf("report = %+v", report)
	}
}

func TestWarningMessageProjectsSingleWarningBody(t *testing.T) {
	t.Parallel()

	if got := warningMessage(pluginmanifest.Warning{Message: "warn"}); got != "warn" {
		t.Fatalf("warning message = %q", got)
	}
}

func TestNormalizePublicationPackageInitializesNilSlices(t *testing.T) {
	t.Parallel()

	pkg := normalizePublicationPackage(publicationmodel.Package{})
	if pkg.ChannelFamilies == nil || pkg.AuthoredInputs == nil || pkg.ManagedArtifacts == nil {
		t.Fatalf("package = %+v", pkg)
	}
}

func TestNormalizePublicationChannelInitializesPackageTargets(t *testing.T) {
	t.Parallel()

	channel := normalizePublicationChannel(publicationmodel.Channel{})
	if channel.PackageTargets == nil {
		t.Fatalf("channel = %+v", channel)
	}
}

func TestPublicationDoctorJSONReportMetadataCopiesCollections(t *testing.T) {
	t.Parallel()

	warnings := []string{"warn"}
	diagnosis := publicationDiagnosis{
		Ready:                 true,
		Status:                "ready",
		Issues:                []publicationIssue{{Code: "demo"}},
		NextSteps:             []string{"step"},
		MissingPackageTargets: []string{"gemini"},
	}
	report := publicationDoctorJSONReportMetadata("gemini", warnings, diagnosis)
	warnings[0] = "changed"
	diagnosis.Issues[0].Code = "changed"
	diagnosis.NextSteps[0] = "changed"
	diagnosis.MissingPackageTargets[0] = "changed"
	if report.WarningCount != 1 || report.Warnings[0] != "warn" || report.Issues[0].Code != "demo" || report.NextSteps[0] != "step" || report.MissingPackageTargets[0] != "gemini" {
		t.Fatalf("report = %+v", report)
	}
}

func TestPublicationJSONReportMetadataCopiesWarnings(t *testing.T) {
	t.Parallel()

	warnings := []string{"warn"}
	report := publicationJSONReportMetadata("gemini", warnings)
	warnings[0] = "changed"
	if report.RequestedTarget != "gemini" || report.WarningCount != 1 || report.Warnings[0] != "warn" {
		t.Fatalf("report = %+v", report)
	}
}

func TestMarshalPublicationDoctorJSONProducesIndentedEnvelope(t *testing.T) {
	t.Parallel()

	body, err := marshalPublicationDoctorJSON(publicationDoctorJSONReport{
		Format:        "plugin-kit-ai/publication-doctor-report",
		SchemaVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 || body[0] != '{' {
		t.Fatalf("body = %q", body)
	}
}

func TestWritePublicationDoctorJSONBodyAddsTrailingNewline(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := writePublicationDoctorJSONBody(cmd, []byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "{\"ok\":true}\n" {
		t.Fatalf("output = %q", got)
	}
}
