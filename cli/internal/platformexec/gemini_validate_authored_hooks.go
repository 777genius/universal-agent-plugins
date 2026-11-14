package platformexec

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateGeminiAuthoredHookContracts(root string, graph pluginmodel.PackageGraph, hookPaths []string) []Diagnostic {
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, validateGeminiHookFiles(root, hookPaths)...)
	diagnostics = append(diagnostics, validateGeminiHookEntrypointContract(root, graph, hookPaths)...)
	diagnostics = append(diagnostics, validateGeminiGeneratedHooks(root, graph, hookPaths)...)
	return diagnostics
}

func validateGeminiHookEntrypointContract(root string, graph pluginmodel.PackageGraph, hookPaths []string) []Diagnostic {
	if graph.Launcher == nil {
		return nil
	}
	return validateGeminiHookEntrypointConsistency(root, hookPaths, strings.TrimSpace(graph.Launcher.Entrypoint))
}
