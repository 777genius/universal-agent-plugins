package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/codexmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func validateCodexBundleInterfaceDiagnostics(root string, state pluginmodel.TargetState, pluginManifest codexmanifest.ImportedPluginManifest) ([]Diagnostic, error) {
	if rel := strings.TrimSpace(state.DocPath("interface")); rel != "" {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return []Diagnostic{{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     filepath.ToSlash(rel),
				Target:   "codex-package",
				Message:  fmt.Sprintf("Codex interface doc %s is not readable: %v", filepath.ToSlash(rel), err),
			}}, err
		}
		interfaceDoc, err := codexmanifest.ParseInterfaceDoc(body)
		if err != nil {
			return []Diagnostic{{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     filepath.ToSlash(rel),
				Target:   "codex-package",
				Message:  fmt.Sprintf("Codex interface doc %s is invalid: %v", filepath.ToSlash(rel), err),
			}}, err
		}
		if !jsonDocumentsEqual(interfaceDoc, pluginManifest.Interface) {
			return []Diagnostic{{
				Severity: SeverityFailure,
				Code:     CodeGeneratedContractInvalid,
				Path:     codexmanifest.PluginManifestPath(),
				Target:   "codex-package",
				Message:  "Codex plugin manifest .codex-plugin/plugin.json interface does not match targets/codex-package/interface.json",
			}}, nil
		}
		return nil, nil
	}
	if pluginManifest.Interface != nil {
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     codexmanifest.PluginManifestPath(),
			Target:   "codex-package",
			Message:  "Codex plugin manifest .codex-plugin/plugin.json may not define interface when targets/codex-package/interface.json is absent",
		}}, nil
	}
	return nil, nil
}
