package platformexec

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateGeminiAuthoredCore(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, validateGeminiDirName(root, graph.Manifest.Name)...)
	projected, err := validateGeminiPortableMCPProjection(graph)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, projected...)
	diagnostics = append(diagnostics, validateGeminiExcludeTools(state.DocPath("package_metadata"), meta.ExcludeTools)...)
	diagnostics = append(diagnostics, validateGeminiContext(graph, state, meta)...)
	return diagnostics, nil
}

func validateGeminiDirName(root, extensionName string) []Diagnostic {
	if base := geminiExtensionDirBase(root); base != extensionName {
		return []Diagnostic{{
			Severity: SeverityWarning,
			Code:     CodeGeminiDirNameMismatch,
			Path:     root,
			Target:   "gemini",
			Message:  fmt.Sprintf("Gemini extension directory basename %q does not match extension name %q", base, extensionName),
		}}
	}
	return nil
}

func validateGeminiPortableMCPProjection(graph pluginmodel.PackageGraph) ([]Diagnostic, error) {
	if graph.Portable.MCP == nil {
		return nil, nil
	}
	projected, err := renderPortableMCPForTarget(graph.Portable.MCP, "gemini")
	if err != nil {
		return nil, err
	}
	return validateGeminiMCPServers(graph.Portable.MCP.Path, projected), nil
}
