package platformexec

import (
	"strings"
	"testing"
)

func TestValidateGeminiContextFileNameProjectionRejectsMismatch(t *testing.T) {
	t.Parallel()

	diagnostics := validateGeminiContextFileNameProjection(geminiContextSelection{ArtifactName: "GEMINI.md"}, importedGeminiExtension{
		Meta: geminiPackageMeta{ContextFileName: "ALT.md"},
	})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, `sets contextFileName "ALT.md"; expected "GEMINI.md"`) {
		t.Fatalf("diagnostics missing context projection failure:\n%s", text)
	}
}

func TestValidateGeminiContextFileReadableRejectsMissingPrimaryContext(t *testing.T) {
	t.Parallel()

	diagnostics := validateGeminiContextFileReadable(t.TempDir(), geminiContextSelection{ArtifactName: "GEMINI.md"})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, "Gemini primary context file GEMINI.md is not readable") {
		t.Fatalf("diagnostics missing missing-context failure:\n%s", text)
	}
}

func TestGeminiContextFileNameMatchesExpectedTrimsExtensionValue(t *testing.T) {
	t.Parallel()

	if !geminiContextFileNameMatchesExpected(geminiContextSelection{ArtifactName: "GEMINI.md"}, importedGeminiExtension{
		Meta: geminiPackageMeta{ContextFileName: " GEMINI.md "},
	}) {
		t.Fatal("expected trimmed context file name match")
	}
}

func TestAppendGeminiContextDiagnosticsConcatenatesParts(t *testing.T) {
	t.Parallel()

	got := appendGeminiContextDiagnostics([]Diagnostic{{Code: "a"}}, []Diagnostic{{Code: "b"}})
	if len(got) != 2 || got[0].Code != "a" || got[1].Code != "b" {
		t.Fatalf("diagnostics = %+v", got)
	}
}

func TestValidateGeminiExpectedContextContractDelegatesExpectedDiagnostics(t *testing.T) {
	t.Parallel()

	diagnostics := validateGeminiExpectedContextContract(t.TempDir(), geminiContextSelection{ArtifactName: "GEMINI.md"}, importedGeminiExtension{
		Meta: geminiPackageMeta{ContextFileName: "ALT.md"},
	})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, `sets contextFileName "ALT.md"; expected "GEMINI.md"`) {
		t.Fatalf("diagnostics missing expected-context failure:\n%s", text)
	}
}

func TestValidateGeminiUnexpectedContextContractDelegatesUnexpectedDiagnostics(t *testing.T) {
	t.Parallel()

	diagnostics := validateGeminiUnexpectedContextContract(importedGeminiExtension{
		Meta: geminiPackageMeta{ContextFileName: "GEMINI.md"},
	})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, `sets contextFileName "GEMINI.md" without an authored primary context`) {
		t.Fatalf("diagnostics missing unexpected-context failure:\n%s", text)
	}
}

func TestValidateGeminiExpectedContextProjectionDelegatesProjectionDiagnostics(t *testing.T) {
	t.Parallel()

	diagnostics := validateGeminiExpectedContextProjection(geminiContextSelection{ArtifactName: "GEMINI.md"}, importedGeminiExtension{
		Meta: geminiPackageMeta{ContextFileName: "ALT.md"},
	})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, `sets contextFileName "ALT.md"; expected "GEMINI.md"`) {
		t.Fatalf("diagnostics missing projection failure:\n%s", text)
	}
}

func TestValidateGeminiExpectedContextArtifactDelegatesReadableDiagnostics(t *testing.T) {
	t.Parallel()

	diagnostics := validateGeminiExpectedContextArtifact(t.TempDir(), geminiContextSelection{ArtifactName: "GEMINI.md"})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, "Gemini primary context file GEMINI.md is not readable") {
		t.Fatalf("diagnostics missing readability failure:\n%s", text)
	}
}

func TestUnexpectedGeminiContextFileNameTrimsWhitespace(t *testing.T) {
	t.Parallel()

	got := unexpectedGeminiContextFileName(importedGeminiExtension{
		Meta: geminiPackageMeta{ContextFileName: " GEMINI.md "},
	})
	if got != "GEMINI.md" {
		t.Fatalf("file name = %q", got)
	}
}

func TestUnexpectedGeminiContextDiagnosticBuildsFailure(t *testing.T) {
	t.Parallel()

	diagnostic := unexpectedGeminiContextDiagnostic("GEMINI.md")
	if diagnostic.Target != "gemini" || !strings.Contains(diagnostic.Message, `sets contextFileName "GEMINI.md" without an authored primary context`) {
		t.Fatalf("diagnostic = %+v", diagnostic)
	}
}

func TestValidateGeminiExtensionContextDiagnosticsUsesExpectedBranch(t *testing.T) {
	t.Parallel()

	diagnostics := validateGeminiExtensionContextDiagnostics(t.TempDir(), geminiExtensionContextSelection{
		expected: geminiContextSelection{ArtifactName: "GEMINI.md"},
		ok:       true,
	}, importedGeminiExtension{
		Meta: geminiPackageMeta{ContextFileName: "ALT.md"},
	})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, `sets contextFileName "ALT.md"; expected "GEMINI.md"`) {
		t.Fatalf("diagnostics missing expected branch failure:\n%s", text)
	}
}

func TestValidateGeminiExtensionContextDiagnosticsUsesUnexpectedBranch(t *testing.T) {
	t.Parallel()

	diagnostics := validateGeminiExtensionContextDiagnostics(t.TempDir(), geminiExtensionContextSelection{}, importedGeminiExtension{
		Meta: geminiPackageMeta{ContextFileName: "GEMINI.md"},
	})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, `sets contextFileName "GEMINI.md" without an authored primary context`) {
		t.Fatalf("diagnostics missing unexpected branch failure:\n%s", text)
	}
}

func TestGeminiExtensionContextSelectionDiagnosticsUsesExpectedBranch(t *testing.T) {
	t.Parallel()

	diagnostics := (geminiExtensionContextSelection{
		expected: geminiContextSelection{ArtifactName: "GEMINI.md"},
		ok:       true,
	}).contractDiagnostics(t.TempDir(), importedGeminiExtension{
		Meta: geminiPackageMeta{ContextFileName: "ALT.md"},
	})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, `sets contextFileName "ALT.md"; expected "GEMINI.md"`) {
		t.Fatalf("diagnostics missing expected branch failure:\n%s", text)
	}
}
