package nativeconfig

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestProjectCodexTOMLExactLifecyclePreservesForeignData(t *testing.T) {
	for name, collection := range map[string]string{
		"inline": "mcp_servers = {foreign = {url = 'https://foreign.test/mcp'}} # inline policy\n",
		"dotted": "mcp_servers.foreign.url = 'https://foreign.test/mcp' # dotted policy\n",
		"parent": "[mcp_servers]\nforeign = {url = 'https://foreign.test/mcp'} # parent policy\n",
		"child":  "[mcp_servers.foreign]\nurl = 'https://foreign.test/mcp' # child policy\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			before := "# preserve native settings\ncreated = 2026-10-23\n" + collection + "\n[foreign]\nkeep = [1, 2, 3] # foreign comment\n"
			mustWrite(t, path, before)
			kernel := New()
			request := Request{Paths: Paths{JSON: path}, Codec: CodecCodex, Action: ActionAdd, Name: "managed", Server: Server{Type: "remote", URL: "https://managed.test/mcp", RemoteTransport: "streamable-http", Headers: map[string]string{"X-Fixture": "inert"}}}
			receipt, err := kernel.Apply(request)
			if err != nil {
				t.Fatal(err)
			}
			present, owned, err := kernel.Inspect(request.Paths, request.Codec, request.Name, &receipt)
			if err != nil || !present || !owned {
				t.Fatalf("exact Codex entry = %v/%v, %v", present, owned, err)
			}
			projectCodexAssertForeignTOML(t, path, before)
			request.Action, request.Owned = ActionUpdate, &receipt
			request.Server = Server{Type: "stdio", Command: "TEST-never-executed", Args: []string{"fixture"}, Env: map[string]string{"FIXTURE": "inert"}, CWD: filepath.Dir(path)}
			receipt, err = kernel.Apply(request)
			if err != nil {
				t.Fatal(err)
			}
			request.Owned = &receipt
			unchanged, _ := os.ReadFile(path)
			if _, err := kernel.Apply(request); err != nil {
				t.Fatal(err)
			}
			assertBytes(t, path, string(unchanged))
			request.Action = ActionRemove
			if _, err := kernel.Apply(request); err != nil {
				t.Fatal(err)
			}
			projectCodexAssertForeignTOML(t, path, before)
			if present, _, err := kernel.Inspect(request.Paths, request.Codec, request.Name, nil); err != nil || present {
				t.Fatalf("managed Codex entry survived: %v, %v", present, err)
			}
		})
	}
}

func projectCodexAssertForeignTOML(t *testing.T, path, before string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var original, actual map[string]any
	if err := toml.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := toml.Unmarshal(body, &actual); err != nil {
		t.Fatal(err)
	}
	delete(actual["mcp_servers"].(map[string]any), "managed")
	if !reflect.DeepEqual(original, actual) {
		t.Fatalf("foreign TOML data changed: %+v", actual)
	}
	for _, comment := range []string{"preserve native settings", "policy", "foreign comment"} {
		if !strings.Contains(string(body), comment) {
			t.Fatalf("foreign TOML comment %q lost", comment)
		}
	}
}

func TestProjectCodexTOMLRejectsMalformedCollisionAndForeignEdit(t *testing.T) {
	for name, body := range map[string]string{
		"duplicate": "[mcp_servers]\nx={command='one'}\nx={command='two'}\n",
		"malformed": "[mcp_servers\n",
		"scalar":    "mcp_servers = false\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			mustWrite(t, path, body)
			request := Request{Paths: Paths{JSON: path}, Codec: CodecCodex, Action: ActionAdd, Name: "managed", Server: Server{Type: "remote", URL: "https://managed.test/mcp"}}
			if _, err := New().Apply(request); !errors.Is(err, ErrMalformed) {
				t.Fatalf("invalid TOML admitted: %v", err)
			}
			assertBytes(t, path, body)
		})
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	request := Request{Paths: Paths{JSON: path}, Codec: CodecCodex, Action: ActionAdd, Name: "managed", Server: Server{Type: "remote", URL: "https://managed.test/mcp"}}
	receipt, err := New().Apply(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New().Apply(request); !errors.Is(err, ErrCollision) {
		t.Fatalf("foreign entry was adopted: %v", err)
	}
	const edited = "[mcp_servers.managed]\nurl='https://managed.test/mcp'\nenabled=false # foreign policy\n"
	mustWrite(t, path, edited)
	for _, action := range []Action{ActionUpdate, ActionRemove} {
		request.Action, request.Owned = action, &receipt
		if _, err := New().Apply(request); !errors.Is(err, ErrNotOwned) {
			t.Fatalf("edited native entry accepted by %s: %v", action, err)
		}
		assertBytes(t, path, edited)
	}
}

func TestProjectCodexTOMLRemovesCompleteNestedOwnedEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	const body = "[mcp_servers.managed]\ncommand='TEST-never-executed'\n[mcp_servers.managed.env]\nFIXTURE='inert'\n[mcp_servers.foreign]\nurl='https://foreign.test/keep' # keep\n"
	mustWrite(t, path, body)
	server := Server{Type: "stdio", Command: "TEST-never-executed", Env: map[string]string{"FIXTURE": "inert"}}
	receipt, err := DesiredReceipt(path, CodecCodex, "managed", server, Placeholders{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New().Apply(Request{Paths: Paths{JSON: path}, Codec: CodecCodex, Action: ActionRemove, Name: "managed", Owned: &receipt}); err != nil {
		t.Fatal(err)
	}
	actual, _ := os.ReadFile(path)
	if strings.Contains(string(actual), "managed") || !strings.Contains(string(actual), "# keep") {
		t.Fatalf("nested native removal changed foreign entry or left owned child: %s", actual)
	}
	if _, err := DesiredReceipt(path, CodecCodex, "sse", Server{Type: "remote", URL: "https://managed.test/mcp", RemoteTransport: "sse"}, Placeholders{}); err == nil {
		t.Fatal("Codex silently accepted unsupported SSE transport")
	}
}
