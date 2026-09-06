package loader

import (
	"encoding/json"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (loader Loader) loadPluginManifest(path string) (domain.PluginManifest, []domain.Diagnostic, string, error) {
	body, exists, err := readRegularFile(path)
	if err != nil {
		return domain.PluginManifest{}, nil, "", domain.FatalLoad("plugin_manifest_read_failed", "plugin.json", "read root plugin.json", err)
	}
	if !exists {
		return domain.PluginManifest{}, nil, "", domain.FatalLoad("plugin_manifest_missing", "plugin.json", "root plugin.json is required", nil)
	}
	manifest, diagnostics, digest, err := (conformance.InstallerDecoder{Registry: loader.Registry}).Plugin(body)
	if err != nil {
		return manifest, diagnostics, digest, err
	}
	return manifest, diagnostics, digest, nil
}
func decodeJSONObject(body []byte) (map[string]json.RawMessage, map[string]any, error) {
	return conformance.DecodeJSONObject(body)
}
func decodeJSON(body []byte, target any) error { return conformance.DecodeJSON(body, target) }
func decodeRawJSONObject(body []byte, target any) error {
	return conformance.DecodeRawJSONObject(body, target)
}
func rejectDuplicateJSONKeys(body []byte) error { return conformance.RejectDuplicateJSONKeys(body) }
