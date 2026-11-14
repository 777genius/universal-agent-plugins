package platformexec

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestFirstDiagnosticMessageReturnsFirstFailure(t *testing.T) {
	t.Parallel()

	got, ok := firstDiagnosticMessage([]Diagnostic{
		{Severity: SeverityWarning, Message: "warn"},
		{Severity: SeverityFailure, Message: "first"},
		{Severity: SeverityFailure, Message: "second"},
	}, SeverityFailure)
	if !ok || got != "first" {
		t.Fatalf("message = %q ok=%v", got, ok)
	}
}

func TestValidateGeminiRenderReadyReturnsNilWithoutFailures(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := validateGeminiRenderReady(root, pluginmodel.PackageGraph{}, pluginmodel.NewTargetState("gemini"), geminiPackageMeta{}); err != nil {
		t.Fatalf("expected no failure, got %v", err)
	}
}
