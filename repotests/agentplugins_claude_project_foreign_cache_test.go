package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

const claudeForeignCachePluginID = "uap-foreign-cache@uap-native-foreign"

// Claude's SDK garbage-collects malformed, unregistered cache directories.
// A genuine user plugin supplies the foreign cache whose bytes must survive
// every subsequent project installation, native observation, and removal.
func claudeProjectSeedForeignCache(t *testing.T, f *claudeProjectNativeFixture) {
	t.Helper()
	source := filepath.Join(f.base.Root, "foreign-native-marketplace")
	files := map[string]string{
		".claude-plugin/marketplace.json":      `{"name":"uap-native-foreign","owner":{"name":"Native fixture"},"plugins":[{"name":"uap-foreign-cache","source":"./plugin"}]}`,
		"plugin/.claude-plugin/plugin.json":    `{"name":"uap-foreign-cache","version":"1.0.0"}`,
		"plugin/skills/foreign-cache/SKILL.md": "---\nname: foreign-cache\ndescription: Preserve foreign native cache\n---\nInert foreign bytes.\n",
		"plugin/keep.txt":                      "foreign user plugin cache\n",
	}
	for relative, body := range files {
		f.keep(t, filepath.Join(source, filepath.FromSlash(relative)), body)
	}
	// Native setup legitimately adds its marketplace and enabled-plugin entry.
	// Pin the resulting user settings once, before any public UAP operation.
	settings := filepath.Join(f.base.ConfigDir, "settings.json")
	delete(f.foreign, settings)
	claudeProjectCapture(t, f, "foreign-marketplace-add", f.base.Root, 0, "plugin", "marketplace", "add", source, "--scope", "user")
	claudeProjectCapture(t, f, "foreign-plugin-install", f.base.Root, 0, "plugin", "install", claudeForeignCachePluginID, "--scope", "user")
	claudeProjectPinForeignSettings(t, f, settings)
	cache := filepath.Join(f.base.ConfigDir, "plugins", "cache", "uap-native-foreign", "uap-foreign-cache", "1.0.0")
	body := claudeProjectCapture(t, f, "foreign-plugin-inventory", f.base.Root, 0, "plugin", "list", "--json")
	if err := validateClaudeForeignCacheListing(body, cache); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{".claude-plugin/plugin.json", "skills/foreign-cache/SKILL.md", "keep.txt"} {
		path := filepath.Join(cache, filepath.FromSlash(relative))
		digest := nativeSHA(t, path)
		if digest != nativeSHA(t, filepath.Join(source, "plugin", filepath.FromSlash(relative))) {
			t.Fatal("native foreign cache differs from its inert source")
		}
		f.foreign[path] = digest
	}
}

func claudeProjectPinForeignSettings(t *testing.T, f *claudeProjectNativeFixture, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	var settings struct {
		EnabledPlugins map[string]bool `json:"enabledPlugins"`
	}
	if err != nil || json.Unmarshal(body, &settings) != nil {
		t.Fatal("native fixture user settings became unreadable", err)
	}
	if enabled, exists := settings.EnabledPlugins["foreign-project-fixture@skills-dir"]; !exists || enabled ||
		len(settings.EnabledPlugins) != 2 || !settings.EnabledPlugins[claudeForeignCachePluginID] {
		t.Fatal("native fixture setup changed the existing foreign disabled plugin")
	}
	f.foreign[path] = nativeSHA(t, path)
}

func validateClaudeForeignCacheListing(body []byte, cache string) error {
	value, err := shared.DecodeUniqueJSONValue(json.NewDecoder(bytes.NewReader(body)))
	rows, ok := value.([]any)
	if err != nil || !json.Valid(body) || !ok {
		return fmt.Errorf("foreign native plugin inventory is not one unique JSON array")
	}
	count := 0
	for _, item := range rows {
		row, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("foreign native plugin inventory contains a malformed row")
		}
		if row["id"] != claudeForeignCachePluginID {
			continue
		}
		count++
		if row["scope"] != "user" || row["enabled"] != true || row["version"] != "1.0.0" || row["installPath"] != cache {
			return fmt.Errorf("foreign native plugin has the wrong scope, state, version, or cache path")
		}
	}
	if count != 1 {
		return fmt.Errorf("foreign native plugin registration is absent or duplicated")
	}
	return nil
}

func TestClaudeForeignCacheRequiresExactNativeUserRegistration(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cache")
	row := map[string]any{"id": claudeForeignCachePluginID, "scope": "user", "enabled": true, "version": "1.0.0", "installPath": cache}
	validBody, err := json.Marshal([]any{row})
	if err != nil || validateClaudeForeignCacheListing(validBody, cache) != nil {
		t.Fatal("exact foreign user cache registration refused", err)
	}
	for key, value := range map[string]any{"id": "other", "scope": "project", "enabled": false, "version": "2.0.0", "installPath": cache + "-other"} {
		original := row[key]
		row[key] = value
		body, err := json.Marshal([]any{row})
		if err != nil || validateClaudeForeignCacheListing(body, cache) == nil {
			t.Fatalf("foreign native %s mismatch accepted: %v", key, err)
		}
		row[key] = original
	}
	duplicate, err := json.Marshal([]any{row, row})
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]byte{[]byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`[false]`), duplicate, append(validBody, []byte(`[]`)...)} {
		if validateClaudeForeignCacheListing(invalid, cache) == nil {
			t.Fatalf("invalid foreign native registration accepted: %s", invalid)
		}
	}
}
