package agentpluginscli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/spf13/cobra"
)

func TestBlockingSecurityAssessmentRequiresExplicitNonInteractiveOverride(t *testing.T) {
	loaded := loadedPackage{security: testSecurityAssessment(1, 2)}
	command := securityTestCommand(&bytes.Buffer{}, strings.NewReader(""))
	err := authorizeSecurityAssessment(command, App{Terminal: false}, &options{format: "human"}, &loaded)
	if err == nil || !strings.Contains(err.Error(), "--accept-security-risk") || loaded.securityAuthorized {
		t.Fatalf("blocking assessment error=%v authorized=%t", err, loaded.securityAuthorized)
	}
	if err := authorizeSecurityAssessment(command, App{Terminal: false}, &options{format: "human", acceptSecurityRisk: true}, &loaded); err != nil || !loaded.securityAuthorized {
		t.Fatalf("explicit override error=%v authorized=%t", err, loaded.securityAuthorized)
	}
}

func TestBlockingSecurityAssessmentPromptsInteractiveUser(t *testing.T) {
	output := &bytes.Buffer{}
	loaded := loadedPackage{security: testSecurityAssessment(1, 0)}
	command := securityTestCommand(output, strings.NewReader("yes\n"))
	if err := authorizeSecurityAssessment(command, App{Terminal: true}, &options{format: "human"}, &loaded); err != nil {
		t.Fatal(err)
	}
	if !loaded.securityAuthorized || !strings.Contains(output.String(), "not a guarantee of safety") || !strings.Contains(output.String(), "Continue anyway") {
		t.Fatalf("unexpected security prompt output: %s", output.String())
	}
}

func TestWarningSecurityAssessmentDoesNotPrompt(t *testing.T) {
	output := &bytes.Buffer{}
	loaded := loadedPackage{security: testSecurityAssessment(0, 3)}
	command := securityTestCommand(output, strings.NewReader(""))
	if err := authorizeSecurityAssessment(command, App{Terminal: true}, &options{format: "human"}, &loaded); err != nil {
		t.Fatal(err)
	}
	if !loaded.securityAuthorized || strings.Contains(output.String(), "Continue anyway") {
		t.Fatalf("warning assessment prompted unexpectedly: %s", output.String())
	}
	if !strings.Contains(output.String(), "no blocking findings; 3 notes found") || !strings.Contains(output.String(), "3 install notes") {
		t.Fatalf("warning assessment was not presented calmly: %s", output.String())
	}
}

func TestWarningSecurityAssessmentCollapsesRepositoryMaintenanceNoise(t *testing.T) {
	output := &bytes.Buffer{}
	assessment := testSecurityAssessment(0, 4)
	assessment.Findings = []domain.SecurityFinding{
		{Code: "SEC329", Disposition: "warning", Path: "mcp.json", Message: "mutable package launcher"},
		{Code: "SEC324", Disposition: "warning", Path: ".github/workflows/ci.yml", Message: "action uses a mutable tag"},
		{Code: "SEC328", Disposition: "warning", Path: ".github/workflows/release.yml", Message: "write token reaches a third-party action"},
		{Code: "SEC325", Disposition: "warning", Path: ".github/workflows/docs.yml", Message: "workflow hardening note"},
	}
	loaded := loadedPackage{security: assessment}
	command := securityTestCommand(output, strings.NewReader(""))
	if err := authorizeSecurityAssessment(command, App{Terminal: true}, &options{format: "human"}, &loaded); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "1 install note, 3 repository-maintenance notes") || !strings.Contains(text, "SEC329") || strings.Contains(text, "SEC324") {
		t.Fatalf("maintenance findings were not collapsed: %s", text)
	}
	if !strings.Contains(text, "3 additional notes hidden") || !strings.Contains(text, "--security-details") {
		t.Fatalf("collapsed output omitted detail guidance: %s", text)
	}
}

func TestSecurityDetailsShowsEveryPublishedFinding(t *testing.T) {
	output := &bytes.Buffer{}
	assessment := testSecurityAssessment(0, 2)
	assessment.Findings = []domain.SecurityFinding{
		{Code: "SEC329", Disposition: "warning", Message: "install note"},
		{Code: "SEC324", Disposition: "warning", Message: "maintenance note"},
	}
	loaded := loadedPackage{security: assessment}
	command := securityTestCommand(output, strings.NewReader(""))
	if err := authorizeSecurityAssessment(command, App{Terminal: true}, &options{format: "human", securityDetails: true}, &loaded); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "SEC329") || !strings.Contains(text, "SEC324") || !strings.Contains(text, "Evidence:") || strings.Contains(text, "hidden") {
		t.Fatalf("detailed output omitted a published finding: %s", text)
	}
}

func securityTestCommand(output *bytes.Buffer, input *strings.Reader) *cobra.Command {
	command := &cobra.Command{}
	command.SetOut(output)
	command.SetIn(input)
	return command
}

func testSecurityAssessment(blocking, warnings int) *domain.SecurityAssessment {
	outcome := domain.SecurityNoBlockingFindings
	if blocking > 0 {
		outcome = domain.SecurityBlockingFindings
	} else if warnings > 0 {
		outcome = domain.SecurityWarnings
	}
	findings := make([]domain.SecurityFinding, 0, blocking+warnings)
	for index := 0; index < blocking; index++ {
		findings = append(findings, domain.SecurityFinding{Code: "SEC330", Disposition: "blocking", Path: "mcp.json", Message: "test blocking finding"})
	}
	for index := 0; index < warnings; index++ {
		findings = append(findings, domain.SecurityFinding{Code: "SEC329", Disposition: "warning", Path: "mcp.json", Message: "test review note"})
	}
	return &domain.SecurityAssessment{
		Scanner: domain.SecurityScanner{ID: domain.SecurityScannerID, Version: domain.SecurityScannerVersion},
		Outcome: outcome, Counts: domain.SecurityCounts{Blocking: blocking, Warnings: warnings, Total: blocking + warnings},
		Findings: findings,
	}
}
