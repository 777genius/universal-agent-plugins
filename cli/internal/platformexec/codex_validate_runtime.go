package platformexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/codexconfig"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (codexRuntimeAdapter) Validate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]Diagnostic, error) {
	config, _, err := codexconfig.ReadImportedConfig(root)
	if err != nil {
		path := filepath.ToSlash(filepath.Join(".codex", "config.toml"))
		code := CodeManifestInvalid
		message := fmt.Sprintf("Codex config file %s is invalid: %v", path, err)
		if os.IsNotExist(err) {
			code = CodeGeneratedContractInvalid
			message = fmt.Sprintf("Codex config file %s is not readable: %v", path, err)
		}
		return []Diagnostic{{
			Severity: SeverityFailure,
			Code:     code,
			Path:     path,
			Target:   "codex-runtime",
			Message:  message,
		}}, nil
	}
	configPath := filepath.ToSlash(filepath.Join(".codex", "config.toml"))
	if graph.Launcher == nil {
		return nil, nil
	}
	var diagnostics []Diagnostic
	expectedModel := "gpt-5.4-mini"
	configExtra, err := loadNativeExtraDoc(root, state, "config_extra", pluginmodel.NativeDocFormatTOML)
	if err != nil {
		return nil, err
	}
	expectedNotify := []string{graph.Launcher.Entrypoint, "notify"}
	if len(config.Notify) != len(expectedNotify) || len(config.Notify) == 0 || strings.TrimSpace(config.Notify[0]) != expectedNotify[0] || (len(config.Notify) > 1 && strings.TrimSpace(config.Notify[1]) != expectedNotify[1]) {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeEntrypointMismatch,
			Path:     configPath,
			Target:   "codex-runtime",
			Message:  fmt.Sprintf("entrypoint mismatch: Codex notify argv uses %q; expected %q from launcher.yaml entrypoint", config.Notify, expectedNotify),
		})
	}
	if meta, ok, err := readYAMLDoc[codexRuntimeMeta](root, state.DocPath("package_metadata")); err != nil {
		return nil, fmt.Errorf("parse %s: %w", state.DocPath("package_metadata"), err)
	} else if ok && strings.TrimSpace(meta.ModelHint) != "" {
		expectedModel = strings.TrimSpace(meta.ModelHint)
	}
	if strings.TrimSpace(config.Model) != expectedModel {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     configPath,
			Target:   "codex-runtime",
			Message:  fmt.Sprintf("Codex config model %q does not match expected model %q", strings.TrimSpace(config.Model), expectedModel),
		})
	}
	if len(configExtra.Fields) > 0 {
		if !jsonDocumentsEqual(configExtra.Fields, config.Extra) {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeGeneratedContractInvalid,
				Path:     configPath,
				Target:   "codex-runtime",
				Message:  "Codex config .codex/config.toml passthrough fields do not match targets/codex-runtime/config.extra.toml",
			})
		}
	} else if len(config.Extra) > 0 {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     configPath,
			Target:   "codex-runtime",
			Message:  "Codex config .codex/config.toml may not define passthrough fields when targets/codex-runtime/config.extra.toml is absent",
		})
	}
	return diagnostics, nil
}
