package nativeconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tailscale/hujson"
)

const vscodeForeignDocument = `{
  // retain this native workspace comment
  "inputs": [{"id":"token", "type":"promptString", "description":"Native input",},],
  "sandbox": {"enabled":true,},
  "servers": {
    "foreign": {"type":"stdio", "command":"foreign", "env":{"PORT":8123}, "disabled":true,},
  },
  "mcpServers": {"copilot-marker":{"command":"keep-separate"}},
}`

func readVSCodeDocument(t *testing.T, path string) (map[string]any, string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	original := string(body)
	standard, err := hujson.Standardize(body)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(standard, &document); err != nil {
		t.Fatal(err)
	}
	return document, original
}

func requireVSCodeForeignPreserved(t *testing.T, path string, before map[string]any) map[string]any {
	t.Helper()
	document, body := readVSCodeDocument(t, path)
	servers := document["servers"].(map[string]any)
	managed, _ := servers["managed"].(map[string]any)
	delete(servers, "managed")
	if !reflect.DeepEqual(document, before) || !strings.Contains(body, "// retain this native workspace comment") {
		t.Fatalf("VS Code mutation changed foreign values or comment: %s", body)
	}
	return managed
}

func TestVSCodeProjectCodecOwnedLifecyclePreservesNativeJSONC(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mcp.json")
	mustWrite(t, path, vscodeForeignDocument)
	before, _ := readVSCodeDocument(t, path)
	placeholders := Placeholders{PackageRoot: filepath.Join(root, "package-${PLUGIN_DATA}"), DataRoot: filepath.Join(root, "data")}
	stdio := Server{Type: "stdio", Command: "TEST-never-executed", Args: []string{"${PLUGIN_ROOT}", "${PLUGIN_DATA}"},
		Env: map[string]string{"CACHE": "${PLUGIN_DATA}/cache", "TOKEN": "${input:token}"}, CWD: "${PLUGIN_ROOT}"}
	req := Request{Paths: Paths{JSON: path}, Codec: CodecVSCode, Action: ActionAdd, Name: "managed", Server: stdio, Placeholders: placeholders}
	desired, err := DesiredReceipt(path, CodecVSCode, req.Name, stdio, placeholders)
	if err != nil {
		t.Fatal(err)
	}
	req.Desired = &desired
	kernel := New()
	receipt, err := kernel.Apply(req)
	if err != nil || receipt != desired {
		t.Fatalf("VS Code staged/apply receipt differs: %+v, %v", receipt, err)
	}
	managed := requireVSCodeForeignPreserved(t, path, before)
	want := map[string]any{"type": "stdio", "command": stdio.Command, "args": []any{placeholders.PackageRoot, placeholders.DataRoot},
		"env": map[string]any{"CACHE": placeholders.DataRoot + "/cache", "TOKEN": "${input:token}"}, "cwd": placeholders.PackageRoot}
	if !reflect.DeepEqual(managed, want) {
		t.Fatalf("VS Code stdio projection = %#v, want %#v", managed, want)
	}
	for _, transport := range []struct{ portable, native string }{{"streamable-http", "http"}, {"sse", "sse"}} {
		req.Action, req.Owned, req.Desired = ActionUpdate, &receipt, nil
		req.Server = Server{Type: "remote", RemoteTransport: transport.portable, URL: "https://inert.test/mcp", Headers: map[string]string{"X-Fixture": "${input:token}"}}
		receipt, err = kernel.Apply(req)
		if err != nil {
			t.Fatal(err)
		}
		managed = requireVSCodeForeignPreserved(t, path, before)
		want = map[string]any{"type": transport.native, "url": req.Server.URL, "headers": map[string]any{"X-Fixture": "${input:token}"}}
		if !reflect.DeepEqual(managed, want) {
			t.Fatalf("VS Code remote projection = %#v, want %#v", managed, want)
		}
		if present, owned, err := kernel.Inspect(req.Paths, req.Codec, req.Name, &receipt); err != nil || !present || !owned {
			t.Fatalf("native JSONC ownership = %t/%t, %v", present, owned, err)
		}
	}
	req.Action, req.Owned = ActionRemove, &receipt
	if _, err := kernel.Apply(req); err != nil {
		t.Fatal(err)
	}
	if managed := requireVSCodeForeignPreserved(t, path, before); managed != nil {
		t.Fatal("exact owned member survived removal")
	}
}
