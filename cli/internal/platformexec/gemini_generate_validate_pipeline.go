package platformexec

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateGeminiRenderReadyDiagnostics(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, validateGeminiExcludeTools(state.DocPath("package_metadata"), meta.ExcludeTools)...)
	projected, err := validateGeminiRenderReadyMCP(graph)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, projected...)
	diagnostics = append(diagnostics, validateGeminiContext(graph, state, meta)...)
	diagnostics = append(diagnostics, validateGeminiSettings(root, state.ComponentPaths("settings"))...)
	diagnostics = append(diagnostics, validateGeminiThemes(root, state.ComponentPaths("themes"))...)
	diagnostics = append(diagnostics, validateGeminiPolicies(root, state.ComponentPaths("policies"))...)
	diagnostics = append(diagnostics, validateGeminiCommands(root, state.ComponentPaths("commands"))...)
	diagnostics = append(diagnostics, validateGeminiHookFiles(root, state.ComponentPaths("hooks"))...)
	return append(diagnostics, validateGeminiRenderReadyHookEntrypoint(root, graph, state)...), nil
}

func validateGeminiRenderReadyMCP(graph pluginmodel.PackageGraph) ([]Diagnostic, error) {
	if graph.Portable.MCP == nil {
		return nil, nil
	}
	projected, err := renderPortableMCPForTarget(graph.Portable.MCP, "gemini")
	if err != nil {
		return nil, err
	}
	return validateGeminiMCPServers(graph.Portable.MCP.Path, projected), nil
}

func validateGeminiRenderReadyHookEntrypoint(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) []Diagnostic {
	if graph.Launcher == nil {
		return nil
	}
	return validateGeminiHookEntrypointConsistency(root, state.ComponentPaths("hooks"), strings.TrimSpace(graph.Launcher.Entrypoint))
}
