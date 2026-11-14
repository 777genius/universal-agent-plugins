package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/codexmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateCodexBundleAppDiagnostics(root string, state pluginmodel.TargetState, pluginManifest codexmanifest.ImportedPluginManifest) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	authoredAppDoc, authoredAppEnabled, authoredDiagnostics, err := loadAuthoredCodexAppDoc(root, state)
	if err != nil || len(authoredDiagnostics) > 0 {
		return authoredDiagnostics, err
	}
	if authoredAppEnabled && strings.TrimSpace(pluginManifest.AppsRef) == "" {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex plugin manifest .codex-plugin/plugin.json must reference %q when targets/codex-package/app.json is enabled", codexmanifest.AppsRef),
		})
	}
	if !authoredAppEnabled && strings.TrimSpace(pluginManifest.AppsRef) != "" {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  "Codex plugin manifest .codex-plugin/plugin.json may not reference apps when targets/codex-package/app.json is empty or absent",
		})
	}
	if ref := strings.TrimSpace(pluginManifest.AppsRef); ref != "" {
		refDiagnostics := validateCodexBundleAppRefDiagnostics(root, ref, authoredAppEnabled, authoredAppDoc)
		diagnostics = append(diagnostics, refDiagnostics...)
	}
	return diagnostics, nil
}

func loadAuthoredCodexAppDoc(root string, state pluginmodel.TargetState) (map[string]any, bool, []Diagnostic, error) {
	if rel := strings.TrimSpace(state.DocPath("app_manifest")); rel != "" {
		sourceBody, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return nil, false, []Diagnostic{{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     filepath.ToSlash(rel),
				Target:   "codex-package",
				Message:  fmt.Sprintf("Codex app manifest %s is not readable: %v", filepath.ToSlash(rel), err),
			}}, err
		}
		appDoc, err := codexmanifest.ParseAppManifestDoc(sourceBody)
		if err != nil {
			return nil, false, []Diagnostic{{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     filepath.ToSlash(rel),
				Target:   "codex-package",
				Message:  fmt.Sprintf("Codex app manifest %s is invalid: %v", filepath.ToSlash(rel), err),
			}}, err
		}
		return appDoc, codexmanifest.AppManifestEnabled(appDoc), nil, nil
	}
	return nil, false, nil, nil
}

func validateCodexBundleAppRefDiagnostics(root, ref string, authoredAppEnabled bool, authoredAppDoc map[string]any) []Diagnostic {
	var diagnostics []Diagnostic
	if ref != codexmanifest.AppsRef {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex plugin manifest .codex-plugin/plugin.json must use %q for apps when present", codexmanifest.AppsRef),
		})
	}
	refPath, err := resolveRelativeRef(root, ref)
	if err != nil {
		return append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex plugin manifest .codex-plugin/plugin.json uses an invalid apps ref %q: %v", ref, err),
		})
	}
	body, err := os.ReadFile(filepath.Join(root, refPath))
	if err != nil {
		return append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     filepath.ToSlash(refPath),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex app manifest %s is not readable: %v", filepath.ToSlash(refPath), err),
		})
	}
	renderedAppDoc, err := codexmanifest.ParseAppManifestDoc(body)
	if err != nil {
		return append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     filepath.ToSlash(refPath),
			Target:   "codex-package",
			Message:  fmt.Sprintf("Codex app manifest %s is invalid: %v", filepath.ToSlash(refPath), err),
		})
	}
	if authoredAppEnabled && !jsonDocumentsEqual(authoredAppDoc, renderedAppDoc) {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     filepath.ToSlash(refPath),
			Target:   "codex-package",
			Message:  "Codex app manifest .app.json does not match targets/codex-package/app.json",
		})
	}
	return diagnostics
}
