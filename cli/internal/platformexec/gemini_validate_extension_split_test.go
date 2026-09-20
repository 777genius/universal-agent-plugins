package platformexec

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestValidateGeminiExtensionMCPContractRejectsGeneratedServersWithoutPortableMCP(t *testing.T) {
	t.Parallel()

	diagnostics, err := validateGeminiExtensionProjectedMCP(pluginmodel.PackageGraph{}, importedGeminiExtension{
		MCPServers: map[string]any{"demo": map[string]any{"command": "node"}},
	})
	if err != nil {
		t.Fatalf("validateGeminiExtensionProjectedMCP error = %v", err)
	}
	if text := diagnosticsText(diagnostics); !strings.Contains(text, "may not define mcpServers when portable MCP is absent") {
		t.Fatalf("diagnostics missing mcp contract failure:\n%s", text)
	}
}

func TestValidateGeminiExtensionContextContractRejectsContextFileWithoutAuthoredContext(t *testing.T) {
	t.Parallel()

	diagnostics, err := validateGeminiExtensionContextContract(t.TempDir(), pluginmodel.PackageGraph{}, pluginmodel.NewTargetState("gemini"), geminiPackageMeta{}, importedGeminiExtension{
		Meta: geminiPackageMeta{ContextFileName: "GEMINI.md"},
	})
	if err != nil {
		t.Fatalf("validateGeminiExtensionContextContract error = %v", err)
	}
	if text := diagnosticsText(diagnostics); !strings.Contains(text, `sets contextFileName "GEMINI.md" without an authored primary context`) {
		t.Fatalf("diagnostics missing context failure:\n%s", text)
	}
}
