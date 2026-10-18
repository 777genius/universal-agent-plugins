package conformance

import (
	"encoding/json"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"strings"
)

var pluginManifestFields = map[string]struct{}{
	"$schema": {}, "name": {}, "version": {}, "description": {}, "author": {},
	"homepage": {}, "repository": {}, "license": {}, "keywords": {}, "extensions": {},
}

func (loader InstallerDecoder) Plugin(body []byte) (domain.PluginManifest, []domain.Diagnostic, string, error) {
	rawFields, decoded, err := decodeJSONObject(body)
	if err != nil {
		return domain.PluginManifest{}, nil, "", domain.FatalLoad("plugin_manifest_malformed", "plugin.json", "parse root plugin.json", err)
	}
	for _, key := range sortedKeys(rawFields) {
		raw := rawFields[key]
		if _, known := pluginManifestFields[key]; known {
			if err := rejectDuplicateJSONKeys(raw); err != nil {
				return domain.PluginManifest{}, nil, "", domain.FatalLoad("plugin_manifest_malformed", "plugin.json", fmt.Sprintf("parse plugin.json field %q", key), err)
			}
		}
	}
	schemaURI, ok := decoded["$schema"].(string)
	if !ok || strings.TrimSpace(schemaURI) == "" {
		return domain.PluginManifest{}, nil, "", domain.FatalLoad("plugin_schema_missing", "plugin.json", "plugin.json requires a string $schema", nil)
	}
	if schemaURI != domain.PluginSchemaV1 || !loader.Registry.Supports(schemaURI) {
		return domain.PluginManifest{}, nil, "", domain.FatalLoad("plugin_schema_unsupported", "plugin.json", fmt.Sprintf("unsupported Agent Plugins schema %q", schemaURI), nil)
	}

	var diagnostics []domain.Diagnostic
	validationDocument := make(map[string]any, len(decoded))
	for _, key := range sortedKeys(decoded) {
		value := decoded[key]
		if _, known := pluginManifestFields[key]; !known {
			diagnostics = append(diagnostics, domain.Diagnostic{
				Severity: domain.SeverityWarning,
				Boundary: domain.BoundaryPlugin,
				Code:     "plugin_unknown_field",
				Path:     "plugin.json",
				Item:     key,
				Message:  fmt.Sprintf("unknown plugin.json field %q was preserved but is not interpreted", key),
			})
			continue
		}
		validationDocument[key] = value
	}

	extensions := map[string]json.RawMessage{}
	var rawExtensions json.RawMessage
	if raw, exists := rawFields["extensions"]; exists {
		var extensionFields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &extensionFields); err != nil || extensionFields == nil {
			delete(validationDocument, "extensions")
			diagnostics = append(diagnostics, domain.Diagnostic{
				Severity: domain.SeverityWarning,
				Boundary: domain.BoundaryPlugin,
				Code:     "plugin_extensions_ignored",
				Path:     "plugin.json",
				Item:     "extensions",
				Message:  "plugin.json extensions was reported and ignored because it is not an object",
			})
		} else {
			rawExtensions = append(json.RawMessage(nil), raw...)
			for _, namespace := range sortedKeys(extensionFields) {
				extensionRaw := extensionFields[namespace]
				var extensionValue any
				if err := decodeJSON(extensionRaw, &extensionValue); err != nil {
					return domain.PluginManifest{}, diagnostics, "", domain.FatalLoad("plugin_schema_invalid", "plugin.json", fmt.Sprintf("plugin extension %q is malformed", namespace), err)
				}
				if _, object := extensionValue.(map[string]any); !object {
					return domain.PluginManifest{}, diagnostics, "", domain.FatalLoad("plugin_schema_invalid", "plugin.json", fmt.Sprintf("plugin extension %q must be an object", namespace), nil)
				}
				extensions[namespace] = append(json.RawMessage(nil), extensionRaw...)
			}
		}
	}
	dependencies, err := pluginDependencies(extensions)
	if err != nil {
		return domain.PluginManifest{}, diagnostics, "", domain.FatalLoad("plugin_schema_invalid", "plugin.json", "invalid plugin dependencies", err)
	}
	if err := loader.Registry.Validate(schemaURI, validationDocument); err != nil {
		return domain.PluginManifest{}, diagnostics, "", domain.FatalLoad("plugin_schema_invalid", "plugin.json", "plugin.json does not conform to Agent Plugins 1.0", err)
	}

	var typed struct {
		Schema      string         `json:"$schema"`
		Name        string         `json:"name"`
		Version     string         `json:"version"`
		Description string         `json:"description"`
		Author      *domain.Author `json:"author"`
		Homepage    string         `json:"homepage"`
		Repository  string         `json:"repository"`
		License     string         `json:"license"`
		Keywords    []string       `json:"keywords"`
	}
	canonical := map[string]json.RawMessage{}
	for _, key := range sortedKeys(rawFields) {
		raw := rawFields[key]
		if _, known := pluginManifestFields[key]; known {
			canonical[key] = raw
		}
	}
	canonicalBody, _ := json.Marshal(canonical)
	if err := json.Unmarshal(canonicalBody, &typed); err != nil {
		return domain.PluginManifest{}, diagnostics, "", domain.FatalLoad("plugin_manifest_decode_failed", "plugin.json", "decode plugin.json fields", err)
	}
	unknown := map[string]json.RawMessage{}
	for _, key := range sortedKeys(rawFields) {
		raw := rawFields[key]
		if _, known := pluginManifestFields[key]; !known {
			unknown[key] = append(json.RawMessage(nil), raw...)
		}
	}
	return domain.PluginManifest{
		Dependencies:  dependencies,
		SchemaURI:     typed.Schema,
		Name:          typed.Name,
		Version:       typed.Version,
		Description:   typed.Description,
		Author:        typed.Author,
		Homepage:      typed.Homepage,
		Repository:    typed.Repository,
		License:       typed.License,
		Keywords:      append([]string(nil), typed.Keywords...),
		Extensions:    extensions,
		RawExtensions: rawExtensions,
		Unknown:       unknown,
		Raw:           append(json.RawMessage(nil), body...),
	}, diagnostics, sha256Digest(body), nil
}

// Dependencies remain in the lossless extension payload; this view is used by
// resolution without changing the pinned standard schema or raw-byte digest.
func pluginDependencies(extensions map[string]json.RawMessage) (map[string]string, error) {
	raw, exists := extensions[domain.ExtensionNamespace]
	if !exists {
		return nil, nil
	}
	var namespace map[string]json.RawMessage
	if err := json.Unmarshal(raw, &namespace); err != nil {
		return nil, err
	}
	declaration, exists := namespace["dependencies"]
	if !exists {
		return nil, nil
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(declaration, &values); err != nil {
		return nil, fmt.Errorf("dependencies must be an object: %w", err)
	}
	if values == nil {
		return nil, fmt.Errorf("dependencies must be an object")
	}
	dependencies := make(map[string]string, len(values))
	for _, distribution := range sortedKeys(values) {
		var selector string
		if err := json.Unmarshal(values[distribution], &selector); err != nil || selector == "" {
			return nil, fmt.Errorf("dependency %q requires a nonempty version string", distribution)
		}
		dependencies[distribution] = selector
	}
	return dependencies, nil
}
