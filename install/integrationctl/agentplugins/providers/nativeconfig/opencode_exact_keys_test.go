package nativeconfig

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestOpenCodeExactKeysBindReceipts(t *testing.T) {
	for _, ext := range []string{"json", "jsonc"} {
		t.Run(ext, func(t *testing.T) {
			root := t.TempDir()
			paths := Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
			selected := filepath.Join(root, "opencode."+ext)
			mustWrite(t, selected, `{"mcp":{}}`)
			keys := []string{"api/server", "api server", "con", `api"server`, `api\server`, "сервер", "api-server"}
			receipts := make([]Receipt, len(keys))
			kernel := New()
			for i, name := range keys {
				receipt, err := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCode, Action: ActionAdd, Name: name, Server: Server{Type: "remote", URL: "https://example.test/v1"}})
				if err != nil {
					t.Fatal(err)
				}
				receipts[i] = receipt
				if receipt.Name != name || receipt.Path != selected {
					t.Fatalf("identity changed: %+v", receipt)
				}
			}
			for i, name := range keys {
				receipt := receipts[i]
				present, owned, err := kernel.Inspect(paths, CodecOpenCode, name, &receipt)
				if err != nil || !present || !owned {
					t.Fatalf("exact lookup %q: %v %v %v", name, present, owned, err)
				}
				wrong := receipts[(i+1)%len(keys)]
				before := mustRead(t, selected)
				if _, err := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCode, Action: ActionRemove, Name: name, Owned: &wrong}); !errors.Is(err, ErrNotOwned) {
					t.Fatalf("cross-key receipt accepted: %v", err)
				}
				if mustRead(t, selected) != before {
					t.Fatal("rejected receipt changed file")
				}
				updated, err := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCode, Action: ActionUpdate, Name: name, Owned: &receipt, Server: Server{Type: "remote", URL: "https://example.test/v2"}})
				if err != nil {
					t.Fatal(err)
				}
				if updated.Digest == receipt.Digest {
					t.Fatal("changed value kept digest")
				}
				if _, err := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCode, Action: ActionRemove, Name: name, Owned: &updated}); err != nil {
					t.Fatal(err)
				}
				present, _, err = kernel.Inspect(paths, CodecOpenCode, name, nil)
				if err != nil || present {
					t.Fatalf("removed key remains: %q %v", name, err)
				}
			}
		})
	}
}
