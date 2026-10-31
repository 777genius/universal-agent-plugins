//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func windsurfAdditionalFieldsCapture(t *testing.T, additions map[string]any) windsurfConfigChange {
	t.Helper()
	f, path, original := newWindsurfConfigDiagnostic(t)
	var document map[string]any
	if err := json.Unmarshal(original, &document); err != nil {
		t.Fatal(err)
	}
	for key, value := range additions {
		document[key] = value
	}
	body, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	unchanged := windsurfConfigDiagnosticUnchanged(t, path, path)
	guard := f.protected[path]
	observed, err := projectNativeRuntimeDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	f.captureWindsurfConfigChange(path, guard, observed, nil)
	change := windsurfConfigDiagnosticResult(t, f, path, guard, "captured")
	if change.AdditionalFields == nil || !change.AutoUpdate.MatchesAuthored || !change.Permissions.MatchesAuthored || (len(additions) != 0 && observed == guard) {
		t.Fatal("classification lost the changed digest or authored safety comparisons")
	}
	unchanged()
	return change
}

func TestWindsurfDocumentedAddedFieldsExposeOnlyFixedNamesAndJSONTypes(t *testing.T) {
	keys := []string{
		"agent", "attribution", "include_gitignored_files", "keymap", "local_tools_bypass_permissions", "mcpServers", "notify", "proxy",
		"read_config_from", "respect_gitignore", "sandbox", "show_hints", "show_path", "subagents_enabled", "theme_mode", "unicode_mode",
	}
	values := []struct {
		kind  string
		value any
	}{
		{"null", nil}, {"boolean", true}, {"string", "PRIVATE-VALUE https://private.invalid/token"}, {"number", json.Number("17")},
		{"array", []any{"PRIVATE-VALUE"}}, {"object", map[string]any{"PRIVATE-KEY": "PRIVATE-VALUE"}},
	}
	for _, key := range keys {
		for _, value := range values {
			t.Run(key+"/"+value.kind, func(t *testing.T) {
				change := windsurfAdditionalFieldsCapture(t, map[string]any{key: value.value})
				want := &windsurfConfigAdditionalFields{Documented: map[string]string{key: value.kind}}
				if !reflect.DeepEqual(change.AdditionalFields, want) {
					t.Fatal("documented fixed name/type or unknown count differs")
				}
			})
		}
	}
}

func TestWindsurfAddedFieldsKeepUnknownNamesPrivateAndBoundTheCompleteRegistry(t *testing.T) {
	additions := map[string]any{
		"agent": nil, "attribution": true, "include_gitignored_files": false, "keymap": map[string]any{"PRIVATE-KEY": "PRIVATE-VALUE"},
		"local_tools_bypass_permissions": false, "mcpServers": map[string]any{"PRIVATE-KEY": "PRIVATE-VALUE"}, "notify": "PRIVATE-VALUE", "proxy": "PRIVATE-VALUE",
		"read_config_from": []any{"PRIVATE-VALUE"}, "respect_gitignore": true, "sandbox": nil, "show_hints": true, "show_path": false,
		"subagents_enabled": false, "theme_mode": "PRIVATE-VALUE", "unicode_mode": "PRIVATE-VALUE",
		"PRIVATE-KEY\n\x1b": "PRIVATE-VALUE", strings.Repeat("PRIVATE-KEY", 100): "PRIVATE-VALUE", "Theme_Mode": "PRIVATE-VALUE", "theme_mоde": "PRIVATE-VALUE",
	}
	change := windsurfAdditionalFieldsCapture(t, additions)
	if len(change.AdditionalFields.Documented) != 16 || change.AdditionalFields.UnknownCount != 4 || *change.TopLevelFields != 22 {
		t.Fatal("full static registry/unknown count was broadened or truncated")
	}
	plain := windsurfAdditionalFieldsCapture(t, map[string]any{})
	if plain.AdditionalFields.Documented == nil || len(plain.AdditionalFields.Documented) != 0 || plain.AdditionalFields.UnknownCount != 0 {
		t.Fatal("empty additional field classification must be an empty object/count")
	}
}

func TestWindsurfAddedFieldsNeverAppearBeforeTheHeldStrictObjectBoundary(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"theme_mode":"PRIVATE-VALUE"`), []byte(`{"theme_mode":1,"theme_mode":2}`), []byte(`{} {}`), []byte(`[]`),
		bytes.Repeat([]byte{' '}, windsurfConfigDiagnosticLimit+1),
	} {
		change := windsurfConfigDiagnostic(strings.Repeat("a", 64), windsurfConfigDigest(body), nil, func() ([]byte, error) { return body, nil })
		if change.AdditionalFields != nil {
			t.Fatal("refused or non-object input exposed an additional-field classification")
		}
	}
	valid := []byte(`{"theme_mode":"PRIVATE-VALUE"}`)
	change := windsurfConfigDiagnostic(strings.Repeat("a", 64), windsurfConfigDigest(valid), nil, func() ([]byte, error) { return []byte(`{}`), nil })
	if change.Status != "observed-digest-changed" || change.AdditionalFields != nil {
		t.Fatal("classification was not correlated to the original held guard digest")
	}
}
