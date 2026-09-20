package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func (cursorAdapter) Validate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]Diagnostic, error) {
	manifest, diagnostics := validateCursorManifestRead(root)
	if len(diagnostics) > 0 {
		return diagnostics, nil
	}
	diagnostics = append(diagnostics, validateCursorPluginIdentity(manifest, graph)...)
	diagnostics = append(diagnostics, validateCursorPortableSkillsContract(root, manifest, graph)...)
	mcpDiagnostics, err := validateCursorPortableMCPContract(root, manifest, graph)
	if err != nil {
		return nil, err
	}
	return append(diagnostics, mcpDiagnostics...), nil
}
