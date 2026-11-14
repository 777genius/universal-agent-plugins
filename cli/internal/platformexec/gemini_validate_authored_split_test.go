package platformexec

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestValidateGeminiPortableMCPProjectionSkipsWhenPortableMCPAbsent(t *testing.T) {
	t.Parallel()

	diagnostics, err := validateGeminiPortableMCPProjection(pluginmodel.PackageGraph{})
	if err != nil {
		t.Fatalf("validateGeminiPortableMCPProjection error = %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestValidateGeminiHookEntrypointContractSkipsWithoutLauncher(t *testing.T) {
	t.Parallel()

	if diagnostics := validateGeminiHookEntrypointContract(t.TempDir(), pluginmodel.PackageGraph{}, []string{"targets/gemini/hooks/hooks.json"}); len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestValidateGeminiDirNamePreservesMismatchMessage(t *testing.T) {
	t.Parallel()

	text := diagnosticsText(validateGeminiDirName(t.TempDir(), "demo-extension"))
	if !strings.Contains(text, "does not match extension name") {
		t.Fatalf("diagnostics missing dir-name mismatch:\n%s", text)
	}
}
