package platformexec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tailscale/hujson"
)

func validateOpenCodeConfigDoc(root string, configPath string) (map[string]any, []Diagnostic) {
	configReadPath := filepath.Join(root, configPath)
	body, err := os.ReadFile(configReadPath)
	if err != nil {
		return nil, []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     configPath,
			Target:   "opencode",
			Message:  fmt.Sprintf("OpenCode config %s is not readable: %v", configPath, err),
		}}
	}
	body, err = hujson.Standardize(body)
	if err != nil {
		return nil, []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     configPath,
			Target:   "opencode",
			Message:  fmt.Sprintf("OpenCode config %s is invalid JSON/JSONC: %v", configPath, err),
		}}
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, []Diagnostic{{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     configPath,
			Target:   "opencode",
			Message:  fmt.Sprintf("OpenCode config %s is invalid JSON/JSONC: %v", configPath, err),
		}}
	}
	var diagnostics []Diagnostic
	if schema, _ := doc["$schema"].(string); strings.TrimSpace(schema) != "https://opencode.ai/config.json" {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeGeneratedContractInvalid,
			Path:     configPath,
			Target:   "opencode",
			Message:  fmt.Sprintf("OpenCode config %s must declare $schema %q", configPath, "https://opencode.ai/config.json"),
		})
	}
	if raw, ok := doc["plugin"]; ok {
		values, ok := raw.([]any)
		if !ok {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     configPath,
				Target:   "opencode",
				Message:  `OpenCode config field "plugin" must be an array of strings or [name, options] tuples`,
			})
		} else {
			for i, value := range values {
				if _, err := normalizeImportedOpenCodePluginRef(value); err != nil {
					diagnostics = append(diagnostics, Diagnostic{
						Severity: SeverityFailure,
						Code:     CodeManifestInvalid,
						Path:     configPath,
						Target:   "opencode",
						Message:  fmt.Sprintf(`OpenCode config field "plugin" has invalid entry at index %d: %v`, i, err),
					})
				}
			}
		}
	}
	if raw, ok := doc["mcp"]; ok {
		if _, ok := raw.(map[string]any); !ok {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     configPath,
				Target:   "opencode",
				Message:  `OpenCode config field "mcp" must be a JSON object`,
			})
		}
	}
	if raw, ok := doc["default_agent"]; ok {
		text, ok := raw.(string)
		if !ok || strings.TrimSpace(text) == "" {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     configPath,
				Target:   "opencode",
				Message:  `OpenCode config field "default_agent" must be a non-empty string`,
			})
		}
	}
	if raw, ok := doc["instructions"]; ok {
		values, ok := raw.([]any)
		if !ok {
			diagnostics = append(diagnostics, Diagnostic{
				Severity: SeverityFailure,
				Code:     CodeManifestInvalid,
				Path:     configPath,
				Target:   "opencode",
				Message:  `OpenCode config field "instructions" must be an array of strings`,
			})
		} else {
			for i, value := range values {
				text, ok := value.(string)
				if !ok || strings.TrimSpace(text) == "" {
					diagnostics = append(diagnostics, Diagnostic{
						Severity: SeverityFailure,
						Code:     CodeManifestInvalid,
						Path:     configPath,
						Target:   "opencode",
						Message:  fmt.Sprintf(`OpenCode config field "instructions" must contain non-empty strings (invalid entry at index %d)`, i),
					})
				}
			}
		}
	}
	if raw, ok := doc["permission"]; ok && !isOpenCodePermissionValue(raw) {
		diagnostics = append(diagnostics, Diagnostic{
			Severity: SeverityFailure,
			Code:     CodeManifestInvalid,
			Path:     configPath,
			Target:   "opencode",
			Message:  `OpenCode config field "permission" must be a string or JSON object`,
		})
	}
	return doc, diagnostics
}
