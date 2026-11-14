package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func validateGeminiExtensionMCPContracts(graph pluginmodel.PackageGraph, extension importedGeminiExtension) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	if len(extension.MCPServers) > 0 {
		diagnostics = append(diagnostics, validateGeminiMCPServers("gemini-extension.json", extension.MCPServers)...)
	}
	projected, err := validateGeminiExtensionProjectedMCP(graph, extension)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, projected...)
	return diagnostics, nil
}

func validateGeminiExtensionProjectedMCP(graph pluginmodel.PackageGraph, extension importedGeminiExtension) ([]Diagnostic, error) {
	if graph.Portable.MCP != nil {
		projected, err := renderPortableMCPForTarget(graph.Portable.MCP, "gemini")
		if err != nil {
			return nil, err
		}
		if jsonDocumentsEqual(projected, extension.MCPServers) {
			return nil, nil
		}
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     "gemini-extension.json",
			Target:   "gemini",
			Message:  "Gemini extension manifest gemini-extension.json mcpServers do not match authored portable MCP projection",
		}}, nil
	}
	if len(extension.MCPServers) == 0 {
		return nil, nil
	}
	return []Diagnostic{{
		Severity: SeverityFailure,
		Code:     CodeGeneratedContractInvalid,
		Path:     "gemini-extension.json",
		Target:   "gemini",
		Message:  "Gemini extension manifest gemini-extension.json may not define mcpServers when portable MCP is absent",
	}}, nil
}
