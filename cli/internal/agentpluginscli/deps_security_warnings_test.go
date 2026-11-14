package agentpluginscli

import (
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestDependencyAuthorizePublishesWarningReasons(t *testing.T) {
	assessment := testSecurityAssessment(0, 1)
	assessment.Findings = []domain.SecurityFinding{{Code: "SEC302", Disposition: "warning", Path: "mcp.json", Line: 7, Message: "configuration contains an insecure http:// endpoint"}}
	loaded := loadedPackage{security: assessment}
	var warnings []string
	adapter := dependencyInstaller{opts: &options{}, previewWarnings: &warnings}
	if err := adapter.authorize(t.Context(), &loaded); err != nil || !loaded.securityAuthorized {
		t.Fatalf("non-CLI warning authorization: %v, authorized=%v", err, loaded.securityAuthorized)
	}
	text := strings.Join(warnings, "\n")
	for _, want := range []string{"SEC302", "configuration contains an insecure http:// endpoint"} {
		if !strings.Contains(text, want) {
			t.Fatalf("review omitted warning reason %q: %q", want, text)
		}
	}
}

func TestDependencyAuthorizeWarningDetailsDoNotExposeSourceText(t *testing.T) {
	for _, tc := range []struct {
		name, code, want string
	}{
		{"known rule", "SEC302", "SEC302: configuration contains an insecure http:// endpoint"},
		{"future rule", "SEC9999", "SEC9999: security warning; details unavailable"},
		{"unsafe rule identity", "SEC302\ncredential-marker", "Security warning; details unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assessment := testSecurityAssessment(0, 1)
			assessment.Findings = []domain.SecurityFinding{{
				Code: tc.code, Disposition: "warning", Line: 99,
				Path:    "/private/credential-marker/\u202emcp.json",
				Message: "endpoint https://user:credential-marker@example.invalid/\n" + strings.Repeat("untrusted ", 1024),
			}}
			loaded := loadedPackage{security: assessment}
			var warnings []string
			adapter := dependencyInstaller{opts: &options{}, previewWarnings: &warnings}
			if err := adapter.authorize(t.Context(), &loaded); err != nil || !loaded.securityAuthorized {
				t.Fatalf("warning changed authorization: %v, authorized=%v", err, loaded.securityAuthorized)
			}
			if len(warnings) != 1 || warnings[0] != tc.want {
				t.Fatalf("unsafe or missing public reason: %#v", warnings)
			}
		})
	}
}

func TestDependencyAuthorizeReportsUnpublishedWarningDetails(t *testing.T) {
	assessment := testSecurityAssessment(0, 4)
	assessment.Evidence = domain.SecurityEvidenceSignedIndex
	assessment.Findings = assessment.Findings[:1]
	loaded := loadedPackage{security: assessment}
	var warnings []string
	adapter := dependencyInstaller{opts: &options{}, previewWarnings: &warnings}
	if err := adapter.authorize(t.Context(), &loaded); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(warnings, "\n")
	if !strings.Contains(text, "SEC329: MCP config launches an external package dynamically at runtime") || !strings.Contains(text, "3 additional warnings; details unavailable") {
		t.Fatalf("review invented or omitted unpublished details: %q", text)
	}
}

func TestDependencyAuthorizeBoundsWarningPreview(t *testing.T) {
	loaded := loadedPackage{security: testSecurityAssessment(0, 6)}
	loaded.security.Findings = loaded.security.Findings[:5]
	var warnings []string
	adapter := dependencyInstaller{opts: &options{}, previewWarnings: &warnings}
	if err := adapter.authorize(t.Context(), &loaded); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(warnings, "\n")
	if strings.Count(text, "SEC329:") != 3 || !strings.Contains(text, "2 additional warnings omitted from this bounded preview") || !strings.Contains(text, "1 additional warning; details unavailable") {
		t.Fatalf("review failed to bound or account for warning details: %q", text)
	}
}

func TestDependencyAuthorizeDoesNotInventWarnings(t *testing.T) {
	loaded := loadedPackage{security: testSecurityAssessment(0, 0)}
	var warnings []string
	adapter := dependencyInstaller{opts: &options{}, previewWarnings: &warnings}
	if err := adapter.authorize(t.Context(), &loaded); err != nil || !loaded.securityAuthorized || len(warnings) != 0 {
		t.Fatalf("clean assessment: %v, authorized=%v, warnings=%#v", err, loaded.securityAuthorized, warnings)
	}
}

func TestDependencyAuthorizeWarningsDoNotOverrideBlockingFindings(t *testing.T) {
	for _, acceptRisk := range []bool{false, true} {
		loaded := loadedPackage{security: testSecurityAssessment(1, 1)}
		var warnings []string
		adapter := dependencyInstaller{opts: &options{acceptSecurityRisk: acceptRisk}, previewWarnings: &warnings}
		err := adapter.authorize(t.Context(), &loaded)
		if acceptRisk {
			if err != nil || !loaded.securityAuthorized {
				t.Fatalf("existing explicit CLI acceptance changed: %v, authorized=%v", err, loaded.securityAuthorized)
			}
		} else if !errors.Is(err, installer.ErrAssessmentRejected) || loaded.securityAuthorized {
			t.Fatalf("warning presentation bypassed blocking assessment: %v, authorized=%v", err, loaded.securityAuthorized)
		}
	}
}
