package nativeconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevinCodecPreservesExactCurrentTransportAndForeignPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp_config.json")
	original := `{"policy":"keep","mcpServers":{"disabled":{"url":"https://foreign.test","disabled":true}}}`
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	kernel := New()
	for _, transport := range []string{"streamable-http", "sse"} {
		server := Server{Type: "remote", URL: "https://managed.test/mcp", Headers: map[string]string{"X-Token": "literal"}, RemoteTransport: transport}
		desired, err := DesiredReceipt(path, CodecDevin, "owned", server, Placeholders{})
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := kernel.Apply(Request{Paths: Paths{JSON: path}, Codec: CodecDevin, Name: "owned", Action: ActionAdd, Server: server})
		if err != nil || receipt != desired {
			t.Fatalf("current transport=%+v,%v", receipt, err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var document map[string]any
		if err := json.Unmarshal(body, &document); err != nil {
			t.Fatal(err)
		}
		entries := document["mcpServers"].(map[string]any)
		managed := entries["owned"].(map[string]any)
		expected := "http"
		if transport == "sse" {
			expected = "sse"
		}
		if managed["url"] != server.URL || managed["transport"] != expected || managed["serverUrl"] != nil || entries["disabled"].(map[string]any)["disabled"] != true || document["policy"] != "keep" {
			t.Fatalf("native Devin shape=%s", body)
		}
		if _, err := kernel.Apply(Request{Paths: Paths{JSON: path}, Codec: CodecDevin, Name: "owned", Action: ActionRemove, Owned: &receipt}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDevinCodecRejectsUnsupportedNativeIntent(t *testing.T) {
	for _, server := range []Server{
		{Type: "stdio", Command: "node", CWD: "/tmp"},
		{Type: "stdio", Command: "node", RemoteTransport: "sse"},
		{Type: "remote", URL: "https://managed.test"},
		{Type: "remote", URL: "https://managed.test", RemoteTransport: "websocket"},
		{Type: "remote", URL: "https://managed.test", RemoteTransport: "sse", OpenCodeV2: &OpenCodeV2Options{}},
	} {
		if _, err := DesiredReceipt(filepath.Join(t.TempDir(), "mcp_config.json"), CodecDevin, "x", server, Placeholders{}); err == nil {
			t.Fatalf("unsupported native intent accepted: %+v", server)
		}
	}
}

func TestDevinNativeCommentsPreserveForeignBytesWithoutRelaxingLegacy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp_config.json")
	const original = `{ // target foreign comment
 "mcpServers": {"foreign": { "url": "https://foreign.test", "disabled": true }},
 "policy": "keep" /* target policy comment */
}`
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	kernel := New()
	server := Server{Type: "remote", URL: "https://managed.test", RemoteTransport: "streamable-http"}
	receipt, err := kernel.Apply(Request{Paths: Paths{JSON: path}, Codec: CodecDevin, Name: "owned", Action: ActionAdd, Server: server})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, foreign := range []string{"// target foreign comment", `"foreign": { "url": "https://foreign.test", "disabled": true }`, `"policy": "keep" /* target policy comment */`} {
		if !strings.Contains(string(body), foreign) {
			t.Fatalf("comment/foreign bytes lost: %s", body)
		}
	}
	if _, err := kernel.Apply(Request{Paths: Paths{JSON: path}, Codec: CodecDevin, Name: "owned", Action: ActionRemove, Owned: &receipt}); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{`{"mcpServers":{},}`, `{"mcpServers":{"x":{},"x":{}}}`} {
		if _, err := parseCodecDocument([]byte(invalid), false, CodecDevin); err == nil {
			t.Fatalf("invalid native grammar accepted: %s", invalid)
		}
		if _, err := StandardizeCommentedJSON([]byte(invalid)); err == nil {
			t.Fatalf("invalid policy grammar accepted: %s", invalid)
		}
	}
	if _, err := parseCodecDocument([]byte(original), false, CodecWindsurf); err == nil {
		t.Fatal("legacy Cascade codec grammar broadened")
	}
}
