package platformexec

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestValidateGeminiExtensionUnexpectedSettingsRejectsGeneratedSettings(t *testing.T) {
	t.Parallel()

	diagnostics := validateGeminiExtensionUnexpectedSettings(importedGeminiExtension{
		Settings: []any{map[string]any{"envVar": "DEMO_API_KEY"}},
	})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, "may not define settings when targets/gemini/settings/** is absent") {
		t.Fatalf("diagnostics missing unexpected settings failure:\n%s", text)
	}
}

func TestValidateGeminiExtensionUnexpectedThemesRejectsGeneratedThemes(t *testing.T) {
	t.Parallel()

	diagnostics := validateGeminiExtensionUnexpectedThemes(importedGeminiExtension{
		Themes: []any{map[string]any{"accent": "orange"}},
	})
	if text := diagnosticsText(diagnostics); !strings.Contains(text, "may not define themes when targets/gemini/themes/** is absent") {
		t.Fatalf("diagnostics missing unexpected themes failure:\n%s", text)
	}
}

func TestValidateGeminiExtensionProjectedMCPRejectsUnexpectedServers(t *testing.T) {
	t.Parallel()

	diagnostics, err := validateGeminiExtensionProjectedMCP(pluginmodel.PackageGraph{}, importedGeminiExtension{
		MCPServers: map[string]any{"demo": map[string]any{"command": "node"}},
	})
	if err != nil {
		t.Fatalf("validateGeminiExtensionProjectedMCP error = %v", err)
	}
	if text := diagnosticsText(diagnostics); !strings.Contains(text, "may not define mcpServers when portable MCP is absent") {
		t.Fatalf("diagnostics missing unexpected MCP failure:\n%s", text)
	}
}
