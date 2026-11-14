package loader

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	appAliasPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	appIDPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~:-]{0,255}$`)
)

func (loader Loader) loadApp(path string, declared, acceptUndeclared bool) (domain.AppComponent, []domain.Diagnostic) {
	body, exists, err := readRegularFile(path)
	component := domain.AppComponent{Present: exists, Declared: declared, Raw: append(json.RawMessage(nil), body...)}
	if !exists {
		if declared {
			return component, []domain.Diagnostic{appDiagnostic("app_manifest_missing", "official manifest declares .app.json but the file is missing", nil)}
		}
		return component, nil
	}
	if err != nil {
		return component, []domain.Diagnostic{appDiagnostic("app_manifest_read_failed", "read root .app.json", err)}
	}
	if !declared && !acceptUndeclared {
		return component, []domain.Diagnostic{{
			Severity: domain.SeverityWarning, Boundary: domain.BoundaryApp,
			Code: "undeclared_app_manifest_ignored", Path: ".app.json",
			Message: "root .app.json is ignored because .codex-plugin/plugin.json does not declare apps",
		}}
	}
	component.Declared = true
	rawFields, _, err := decodeJSONObject(body)
	if err != nil {
		return component, []domain.Diagnostic{appDiagnostic("app_manifest_malformed", "parse root .app.json", err)}
	}
	appsRaw, ok := rawFields["apps"]
	if !ok {
		return component, []domain.Diagnostic{appDiagnostic("app_entries_missing", ".app.json requires an apps object", nil)}
	}
	var entries map[string]json.RawMessage
	if err := decodeRawJSONObject(appsRaw, &entries); err != nil || entries == nil {
		return component, []domain.Diagnostic{appDiagnostic("app_entries_invalid", ".app.json apps must be an object", err)}
	}
	if len(entries) == 0 {
		return component, []domain.Diagnostic{appDiagnostic("app_entries_empty", ".app.json apps must contain at least one registered connection", nil)}
	}

	component.Bindings = make(map[string]domain.AppBinding, len(entries))
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	var diagnostics []domain.Diagnostic
	for _, name := range names {
		raw := entries[name]
		if !appAliasPattern.MatchString(name) {
			diagnostics = append(diagnostics, appEntryDiagnostic(name, "app_alias_invalid", "app alias must use letters, digits, dot, underscore, or hyphen"))
			continue
		}
		var entry map[string]json.RawMessage
		if err := decodeJSON(raw, &entry); err != nil || entry == nil {
			diagnostics = append(diagnostics, appEntryDiagnostic(name, "app_entry_invalid", "app entry must be an object"))
			continue
		}
		var id string
		if rawID, ok := entry["id"]; !ok || json.Unmarshal(rawID, &id) != nil || !appIDPattern.MatchString(id) {
			diagnostics = append(diagnostics, appEntryDiagnostic(name, "app_id_invalid", "app id must be a non-empty opaque safe ASCII token"))
			continue
		}
		binding := domain.AppBinding{Alias: name, ID: id, Raw: append(json.RawMessage(nil), raw...)}
		valid := true
		for _, field := range []struct {
			name   string
			target *bool
		}{
			{name: "optional", target: &binding.Optional},
			{name: "required", target: &binding.Required},
		} {
			if value, ok := entry[field.name]; ok && json.Unmarshal(value, field.target) != nil {
				diagnostics = append(diagnostics, appEntryDiagnostic(name, "app_entry_"+field.name+"_invalid", fmt.Sprintf("app entry %s must be a boolean", field.name)))
				valid = false
			}
		}
		if valid {
			component.Bindings[name] = binding
		}
	}
	component.Enabled = len(component.Bindings) == len(entries)
	return component, diagnostics
}

func appDiagnostic(code, message string, cause error) domain.Diagnostic {
	if cause != nil {
		message += ": " + cause.Error()
	}
	return domain.Diagnostic{Severity: domain.SeverityError, Boundary: domain.BoundaryApp, Code: code, Path: ".app.json", Message: message}
}

func appEntryDiagnostic(name, code, message string) domain.Diagnostic {
	return domain.Diagnostic{Severity: domain.SeverityError, Boundary: domain.BoundaryApp, Code: code, Path: ".app.json", Item: name, Message: message}
}
