package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func (qoderAdapter) Validate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]Diagnostic, error) {
	manifest, diagnostics := validateQoderManifestRead(root)
	if len(diagnostics) > 0 {
		return diagnostics, nil
	}
	diagnostics = append(diagnostics, validateQoderManifestRequiredKeys(manifest)...)
	diagnostics = append(diagnostics, validateQoderPluginIdentity(manifest, graph)...)
	diagnostics = append(diagnostics, validateQoderPortableSkillsContract(root, manifest, graph)...)
	mcpDiagnostics, err := validateQoderPortableMCPContract(root, manifest, graph)
	if err != nil {
		return nil, err
	}
	return append(diagnostics, mcpDiagnostics...), nil
}
