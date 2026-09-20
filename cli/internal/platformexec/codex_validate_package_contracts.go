package platformexec

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/codexmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateCodexPackageMetadataDiagnostics(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, pluginManifest codexmanifest.ImportedPluginManifest) ([]Diagnostic, error) {
	if meta, ok, err := readYAMLDoc[codexPackageMeta](root, state.DocPath("package_metadata")); err != nil {
		return nil, fmt.Errorf("parse %s: %w", state.DocPath("package_metadata"), err)
	} else {
		expectedMeta := codexPackageMeta{
			Author:     manifestAuthorToCodex(graph.Manifest.Author),
			Homepage:   strings.TrimSpace(graph.Manifest.Homepage),
			Repository: strings.TrimSpace(graph.Manifest.Repository),
			License:    strings.TrimSpace(graph.Manifest.License),
			Keywords:   append([]string(nil), graph.Manifest.Keywords...),
		}
		if ok {
			mergeCodexPackageMeta(&expectedMeta, meta)
		}
		expectedMeta.Normalize()
		if !codexPackageMetaEqual(expectedMeta, pluginManifest.PackageMeta) {
			return []Diagnostic{{
				Severity: SeverityFailure,
				Code:     CodeGeneratedContractInvalid,
				Path:     codexmanifest.PluginManifestPath(),
				Target:   "codex-package",
				Message:  "Codex plugin manifest .codex-plugin/plugin.json package metadata does not match plugin.yaml plus optional targets/codex-package/package.yaml overrides",
			}}, nil
		}
	}
	return nil, nil
}

func validateCodexSkillsDiagnostics(graph pluginmodel.PackageGraph, pluginManifest codexmanifest.ImportedPluginManifest) []Diagnostic {
	var diagnostics []Diagnostic
	if hasSkills := len(graph.Portable.Paths("skills")) > 0; hasSkills {
		if strings.TrimSpace(pluginManifest.SkillsPath) == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeGeneratedContractInvalid,
				Path:     codexmanifest.PluginManifestPath(),
				Target:   "codex-package",
				Message:  "Codex plugin manifest .codex-plugin/plugin.json must reference ./skills/ when portable skills are authored",
			})
		}
	} else if strings.TrimSpace(pluginManifest.SkillsPath) != "" {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  "Codex plugin manifest .codex-plugin/plugin.json may not reference skills when no portable skills are authored",
		})
	}
	if ref := strings.TrimSpace(pluginManifest.SkillsPath); ref != "" && ref != codexmanifest.SkillsRef {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex plugin manifest .codex-plugin/plugin.json must use %q for skills when present", codexmanifest.SkillsRef),
		})
	}
	return diagnostics
}
