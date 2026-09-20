package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func (claudeAdapter) Validate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	diagnostics = append(diagnostics, validateClaudeLauncherContract(graph, state)...)
	diagnostics = append(diagnostics, validateClaudeHookComponents(root, graph, state)...)
	diagnostics = append(diagnostics, validateClaudeSettings(root, state.DocPath("settings"))...)
	diagnostics = append(diagnostics, validateClaudeLSP(root, state.DocPath("lsp"))...)
	diagnostics = append(diagnostics, validateClaudeUserConfig(root, state.DocPath("user_config"))...)
	return diagnostics, nil
}
