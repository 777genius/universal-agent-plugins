package platformexec

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateCursorPluginIdentity(manifest map[string]any, graph pluginmodel.PackageGraph) []Diagnostic {
	return append(
		append(
			validateCursorIdentityField(manifest, "name", strings.TrimSpace(graph.Manifest.Name)),
			validateCursorIdentityField(manifest, "version", strings.TrimSpace(graph.Manifest.Version))...,
		),
		validateCursorIdentityField(manifest, "description", strings.TrimSpace(graph.Manifest.Description))...,
	)
}

func validateCursorIdentityField(manifest map[string]any, field string, expected string) []Diagnostic {
	got := stringMapField(manifest, field)
	if got == expected {
		return nil
	}
	return []Diagnostic{cursorPluginManifestDiagnostic(
		CodeGeneratedContractInvalid,
		fmt.Sprintf("Cursor plugin manifest %s sets %s %q; expected %q from plugin.yaml", cursorPluginManifestPath, field, got, expected),
	)}
}
