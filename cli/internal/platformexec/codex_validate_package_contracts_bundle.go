package platformexec

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/codexmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateCodexMCPDiagnostics(root string, graph pluginmodel.PackageGraph, pluginManifest codexmanifest.ImportedPluginManifest) ([]Diagnostic, error) {
	return validateCodexBundleMCPDiagnostics(root, graph, pluginManifest)
}

func validateCodexInterfaceDiagnostics(root string, state pluginmodel.TargetState, pluginManifest codexmanifest.ImportedPluginManifest) ([]Diagnostic, error) {
	return validateCodexBundleInterfaceDiagnostics(root, state, pluginManifest)
}

func validateCodexAppDiagnostics(root string, state pluginmodel.TargetState, pluginManifest codexmanifest.ImportedPluginManifest) ([]Diagnostic, error) {
	return validateCodexBundleAppDiagnostics(root, state, pluginManifest)
}
