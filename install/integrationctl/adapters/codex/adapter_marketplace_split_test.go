package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarketplaceDocPreservesExtraFields(t *testing.T) {
	t.Parallel()
	var doc marketplaceDoc
	body := []byte(`{"name":"demo","plugins":[],"extra_flag":true}`)
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, ok := doc.Extra["extra_flag"].(bool); !ok || !got {
		t.Fatalf("Extra = %#v", doc.Extra)
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(encoded) == "" || !json.Valid(encoded) {
		t.Fatalf("encoded = %s", encoded)
	}
	if !containsJSONField(string(encoded), `"extra_flag":true`) {
		t.Fatalf("encoded missing extra field: %s", encoded)
	}
}

func TestReadPluginConfigStateDetectsDisabledPlugin(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "config.toml")
	if err := os.WriteFile(path, []byte("[plugins.demo]\nenabled = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state, warning := readPluginConfigState(path, "demo")
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if !state.Present || !state.Disabled {
		t.Fatalf("state = %+v", state)
	}
}

func containsJSONField(body, want string) bool {
	return strings.Contains(body, want)
}
