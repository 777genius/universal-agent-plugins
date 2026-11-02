package nativeconfig

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectKimiCodecPreservesExplicitTransportsAndForeignMembers(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".mcp.json")
	const foreign = `{"mcpServers":{"foreign":{"transport":"http","url":"https://foreign.test/mcp","enabled":false}},"foreignPreference":"keep"}`
	mustWrite(t, path, foreign)
	kernel := New()
	request := Request{Paths: Paths{JSON: path}, Codec: CodecKimi, Action: ActionAdd, Name: "managed",
		Server: Server{Type: "remote", RemoteTransport: "streamable-http", URL: "https://managed.test/mcp", Headers: map[string]string{"X-Fixture": "inert"}}}
	receipt, err := kernel.Apply(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, transport := range []struct {
		native string
		server Server
	}{{"http", request.Server}, {"sse", Server{Type: "remote", RemoteTransport: "sse", URL: "https://managed.test/events"}}, {"stdio", Server{Type: "stdio", Command: "TEST-never-executed", Args: []string{"inert"}}}} {
		request.Action, request.Owned, request.Server = ActionUpdate, &receipt, transport.server
		receipt, err = kernel.Apply(request)
		if err != nil {
			t.Fatal(err)
		}
		present, owned, err := kernel.Inspect(request.Paths, request.Codec, request.Name, &receipt)
		body, readErr := os.ReadFile(path)
		var observed struct {
			Servers map[string]struct{ Transport string } `json:"mcpServers"`
		}
		decodeErr := json.Unmarshal(body, &observed)
		if err != nil || readErr != nil || decodeErr != nil || !present || !owned || observed.Servers["managed"].Transport != transport.native || !strings.Contains(string(body), "foreign.test") || !strings.Contains(string(body), "foreignPreference") {
			t.Fatalf("Kimi native transport %s = %s, %v, %v", transport.native, body, err, readErr)
		}
	}
	request.Action, request.Owned = ActionRemove, &receipt
	if _, err := kernel.Apply(request); err != nil {
		t.Fatal(err)
	}
	if present, _, err := kernel.Inspect(request.Paths, request.Codec, request.Name, nil); err != nil || present {
		t.Fatalf("Kimi exact removal = %t, %v", present, err)
	}
	request.Action, request.Name, request.Owned = ActionAdd, "foreign", nil
	if _, err := kernel.Apply(request); !errors.Is(err, ErrCollision) {
		t.Fatalf("disabled foreign Kimi server was adopted: %v", err)
	}
}

func TestProjectKimiCodecRejectsUnknownRemoteTransport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	for _, server := range []Server{
		{Type: "remote", URL: "https://managed.test/mcp"},
		{Type: "remote", URL: "https://managed.test/mcp", RemoteTransport: "unknown"},
		{Type: "stdio", Command: "TEST-never-executed", RemoteTransport: "sse"},
	} {
		if _, err := DesiredReceipt(path, CodecKimi, "managed", server, Placeholders{}); err == nil {
			t.Fatalf("unqualified native transport admitted: %+v", server)
		}
	}
}
