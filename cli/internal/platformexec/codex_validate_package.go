package platformexec

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/codexmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (codexPackageAdapter) Validate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]Diagnostic, error) {
	pluginManifest, diagnostics, err := loadCodexPackageManifest(root)
	if err != nil || len(diagnostics) > 0 {
		return diagnostics, err
	}

	for _, path := range codexmanifest.UnexpectedBundleSidecars(root, pluginManifest) {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     path,
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex package bundle may not include %s without a matching .codex-plugin/plugin.json ref", path),
		})
	}
	diagnostics = append(diagnostics, validateCodexPackageIdentityDiagnostics(graph, pluginManifest)...)
	metaDiagnostics, err := validateCodexPackageMetadataDiagnostics(root, graph, state, pluginManifest)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, metaDiagnostics...)
	diagnostics = append(diagnostics, validateCodexSkillsDiagnostics(graph, pluginManifest)...)
	diagnostics = append(diagnostics, validateCodexHooksDiagnostics(root, state)...)
	mcpDiagnostics, err := validateCodexMCPDiagnostics(root, graph, pluginManifest)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, mcpDiagnostics...)
	interfaceDiagnostics, err := validateCodexInterfaceDiagnostics(root, state, pluginManifest)
	if err != nil {
		return interfaceDiagnostics, nil
	}
	diagnostics = append(diagnostics, interfaceDiagnostics...)
	appDiagnostics, err := validateCodexAppDiagnostics(root, state, pluginManifest)
	if err != nil {
		return appDiagnostics, nil
	}
	diagnostics = append(diagnostics, appDiagnostics...)
	return diagnostics, nil
}

func loadCodexPackageManifest(root string) (codexmanifest.ImportedPluginManifest, []Diagnostic, error) {
	if err := codexmanifest.ValidatePluginDirLayout(root); err != nil {
		path := codexmanifest.PluginManifestPath()
		var layoutErr *codexmanifest.PluginDirLayoutError
		if errors.As(err, &layoutErr) && strings.TrimSpace(layoutErr.Path) != "" {
			path = layoutErr.Path
		}
		return codexmanifest.ImportedPluginManifest{}, []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     path,
			Target:   "codex-package",
			Message:  err.Error(),
		}}, nil
	}
	body, err := os.ReadFile(filepath.Join(root, codexmanifest.PluginDir, codexmanifest.PluginFileName))
	if err != nil {
		return codexmanifest.ImportedPluginManifest{}, []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex plugin manifest %s is not readable: %v", codexmanifest.PluginManifestPath(), err),
		}}, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return codexmanifest.ImportedPluginManifest{}, []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex plugin manifest %s is invalid JSON: %v", codexmanifest.PluginManifestPath(), err),
		}}, nil
	}
	pluginManifest, err := codexmanifest.DecodeImportedPluginManifest(body)
	if err != nil {
		return codexmanifest.ImportedPluginManifest{}, []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex plugin manifest %s is invalid: %v", codexmanifest.PluginManifestPath(), err),
		}}, nil
	}
	return pluginManifest, nil, nil
}

func validateCodexPackageIdentityDiagnostics(graph pluginmodel.PackageGraph, pluginManifest codexmanifest.ImportedPluginManifest) []Diagnostic {
	var diagnostics []Diagnostic
	if strings.TrimSpace(pluginManifest.Name) != strings.TrimSpace(graph.Manifest.Name) {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex plugin manifest .codex-plugin/plugin.json sets name %q; expected %q from plugin.yaml", strings.TrimSpace(pluginManifest.Name), strings.TrimSpace(graph.Manifest.Name)),
		})
	}
	if strings.TrimSpace(pluginManifest.Version) != strings.TrimSpace(graph.Manifest.Version) {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex plugin manifest .codex-plugin/plugin.json sets version %q; expected %q from plugin.yaml", strings.TrimSpace(pluginManifest.Version), strings.TrimSpace(graph.Manifest.Version)),
		})
	}
	if strings.TrimSpace(pluginManifest.Description) != strings.TrimSpace(graph.Manifest.Description) {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex plugin manifest .codex-plugin/plugin.json sets description %q; expected %q from plugin.yaml", strings.TrimSpace(pluginManifest.Description), strings.TrimSpace(graph.Manifest.Description)),
		})
	}
	return diagnostics
}
