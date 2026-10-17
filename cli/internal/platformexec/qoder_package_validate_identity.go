package platformexec

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateQoderPluginIdentity(manifest map[string]any, graph pluginmodel.PackageGraph) []Diagnostic {
	diagnostics := append(
		append(
			validateQoderIdentityField(manifest, "name", strings.TrimSpace(graph.Manifest.Name)),
			validateQoderIdentityField(manifest, "version", strings.TrimSpace(graph.Manifest.Version))...,
		),
		validateQoderIdentityField(manifest, "description", strings.TrimSpace(graph.Manifest.Description))...,
	)
	return append(diagnostics, validateQoderIdentityAuthor(manifest, graph.Manifest.Author)...)
}

func validateQoderIdentityField(manifest map[string]any, field string, expected string) []Diagnostic {
	got := stringMapField(manifest, field)
	if got == expected {
		return nil
	}
	return []Diagnostic{qoderPluginManifestDiagnostic(
		CodeGeneratedContractInvalid,
		fmt.Sprintf("Qoder plugin manifest %s sets %s %q; expected %q from plugin.yaml", qoderPluginManifestPath, field, got, expected),
	)}
}

func validateQoderIdentityAuthor(manifest map[string]any, author *pluginmodel.Author) []Diagnostic {
	expected := qoderPluginAuthor(author)
	value, ok := manifest["author"]
	if len(expected) == 0 {
		if !ok {
			return nil
		}
		return []Diagnostic{qoderPluginManifestDiagnostic(
			CodeGeneratedContractInvalid,
			fmt.Sprintf("Qoder plugin manifest %s may not define %q when plugin.yaml has no author", qoderPluginManifestPath, "author"),
		)}
	}
	got, ok := value.(map[string]any)
	if ok && jsonDocumentsEqual(expected, got) {
		return nil
	}
	return []Diagnostic{qoderPluginManifestDiagnostic(
		CodeGeneratedContractInvalid,
		fmt.Sprintf("Qoder plugin manifest %s field %q does not match author from plugin.yaml", qoderPluginManifestPath, "author"),
	)}
}
