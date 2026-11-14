package platformexec

import (
	"errors"
	"strings"
	"testing"
)

func TestInvalidGeminiGeneratedExtensionDiagnosticsPreserveMessage(t *testing.T) {
	t.Parallel()

	text := diagnosticsText(invalidGeminiGeneratedExtensionDiagnostics(errors.New("boom")))
	if !strings.Contains(text, "gemini-extension.json is invalid: boom") {
		t.Fatalf("diagnostics missing invalid-manifest failure:\n%s", text)
	}
}

func TestMissingGeminiGeneratedExtensionDiagnosticsPreserveMessage(t *testing.T) {
	t.Parallel()

	text := diagnosticsText(missingGeminiGeneratedExtensionDiagnostics())
	if !strings.Contains(text, "gemini-extension.json is not readable") {
		t.Fatalf("diagnostics missing unreadable-manifest failure:\n%s", text)
	}
}
