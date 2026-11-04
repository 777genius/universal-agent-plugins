package nativeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQoderNativeHTTPTransportAndForeignMembership(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mcp.json")
	original := `{"mcpServers":{"foreign":{"command":"left-alone","args":["x"]}},"other":{"doNotTouch":true}}`
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	kernel := New()
	paths := Paths{JSON: path}
	server := Server{Type: "remote", URL: "https://example.test/mcp", Headers: map[string]string{"X-Example": "value"}}
	receipt, err := DesiredReceipt(path, CodecQoder, "owned", server, Placeholders{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Apply(Request{Paths: paths, Codec: CodecQoder, Action: ActionAdd, Name: "owned", Server: server, Desired: &receipt}); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"type":"http"`) || !strings.Contains(string(body), `"left-alone"`) || !strings.Contains(string(body), `"doNotTouch"`) {
		t.Fatalf("qoder HTTP/foreign projection was lost: %s", body)
	}
	if present, owned, err := kernel.Inspect(paths, CodecQoder, "owned", &receipt); err != nil || !present || !owned {
		t.Fatalf("owned HTTP receipt missing: %v %v %v", present, owned, err)
	}
	if _, err := kernel.Apply(Request{Paths: paths, Codec: CodecQoder, Action: ActionRemove, Name: "owned", Owned: &receipt}); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(path)
	if err != nil || strings.Contains(string(body), `"owned"`) || !strings.Contains(string(body), `"foreign"`) {
		t.Fatalf("qoder remove affected unowned membership: %s %v", body, err)
	}
}

func TestQoderNativeRejectsUnknownTransport(t *testing.T) {
	_, err := DesiredReceipt(filepath.Join(t.TempDir(), ".mcp.json"), CodecQoder, "not-sse",
		Server{Type: "remote", URL: "https://example.test/sse", RemoteTransport: "sse"}, Placeholders{})
	if err == nil {
		t.Fatal("unqualified qoder SSE transport accepted")
	}
}
