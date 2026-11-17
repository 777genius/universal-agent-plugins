package nativeconfig

import (
	"path/filepath"
	"testing"
)

// Regression: removing the last owned server left the "mcp" collection the
// add had created, as an empty, reformatted object. A collection that existed
// before the add, even an empty one, stays.
func TestOpenCodeRemovePrunesOnlyTheCreatedCollection(t *testing.T) {
	const schemaOnly = "{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n"
	for _, tc := range []struct {
		name     string
		codec    Codec
		original string
		keepMCP  bool
	}{
		{name: "created v1", codec: CodecOpenCode, original: schemaOnly},
		{name: "created v2", codec: CodecOpenCodeV2, original: schemaOnly},
		{name: "foreign empty", codec: CodecOpenCode, original: "{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"mcp\": {}\n}\n", keepMCP: true},
		{name: "foreign minified empty", codec: CodecOpenCode, original: `{"mcp":{}}`, keepMCP: true},
		{name: "foreign v2 empty", codec: CodecOpenCodeV2, original: "{\n  \"mcp\": {\n    \"servers\": {}\n  }\n}\n", keepMCP: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "opencode.json")
			mustWrite(t, path, tc.original)
			kernel := New()
			req := Request{Paths: Paths{JSON: path, JSONC: filepath.Join(root, "opencode.jsonc")}, Codec: tc.codec, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node"}}
			owned, err := kernel.Apply(req)
			if err != nil {
				t.Fatal(err)
			}
			req.Action, req.Owned = ActionRemove, &owned
			if _, err := kernel.Apply(req); err != nil {
				t.Fatal(err)
			}
			if !tc.keepMCP {
				assertBytes(t, path, tc.original)
				return
			}
			var doc map[string]any
			mustReadJSON(t, path, &doc)
			mcp, ok := doc["mcp"].(map[string]any)
			if !ok || tc.codec == CodecOpenCodeV2 && mcp["servers"] == nil {
				t.Fatalf("foreign collection removed: %s", mustRead(t, path))
			}
		})
	}
}
