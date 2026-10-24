package codextoml

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
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
			kernel := newKernel()
			request := nativeconfig.Request{nativeconfig.Paths: nativeconfig.Paths{JSON: path}, Codec: nativeconfig.CodecCodex, nativeconfig.Action: nativeconfig.ActionAdd, Name: "managed", nativeconfig.Server: nativeconfig.Server{Type: "remote", URL: "https://managed.test/mcp", RemoteTransport: "streamable-http", Headers: map[string]string{"X-Fixture": "inert"}}}
			receipt, err := kernel.Apply(request)
			if err != nil {
				t.Fatal(err)
			}
			present, owned, err := kernel.Inspect(request.nativeconfig.Paths, request.Codec, request.Name, &receipt)
			if err != nil || !present || !owned {
				t.Fatalf("exact Codex entry = %v/%v, %v", present, owned, err)
			}
			projectCodexAssertForeignTOML(t, path, before)
			request.nativeconfig.Action, request.Owned = nativeconfig.ActionUpdate, &receipt
			request.nativeconfig.Server = nativeconfig.Server{Type: "stdio", Command: "TEST-never-executed", Args: []string{"fixture"}, Env: map[string]string{"FIXTURE": "inert"}, CWD: filepath.Dir(path)}
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
			request.nativeconfig.Action = nativeconfig.ActionRemove
			if _, err := kernel.Apply(request); err != nil {
				t.Fatal(err)
			}
			projectCodexAssertForeignTOML(t, path, before)
			if present, _, err := kernel.Inspect(request.nativeconfig.Paths, request.Codec, request.Name, nil); err != nil || present {
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
			request := nativeconfig.Request{nativeconfig.Paths: nativeconfig.Paths{JSON: path}, Codec: nativeconfig.CodecCodex, nativeconfig.Action: nativeconfig.ActionAdd, Name: "managed", nativeconfig.Server: nativeconfig.Server{Type: "remote", URL: "https://managed.test/mcp"}}
			if _, err := newKernel().Apply(request); !errors.Is(err, nativeconfig.ErrMalformed) {
				t.Fatalf("invalid TOML admitted: %v", err)
			}
			assertBytes(t, path, body)
		})
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	request := nativeconfig.Request{nativeconfig.Paths: nativeconfig.Paths{JSON: path}, Codec: nativeconfig.CodecCodex, nativeconfig.Action: nativeconfig.ActionAdd, Name: "managed", nativeconfig.Server: nativeconfig.Server{Type: "remote", URL: "https://managed.test/mcp"}}
	receipt, err := newKernel().Apply(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newKernel().Apply(request); !errors.Is(err, nativeconfig.ErrCollision) {
		t.Fatalf("foreign entry was adopted: %v", err)
	}
	const edited = "[mcp_servers.managed]\nurl='https://managed.test/mcp'\nenabled=false # foreign policy\n"
	mustWrite(t, path, edited)
	for _, action := range []nativeconfig.Action{nativeconfig.ActionUpdate, nativeconfig.ActionRemove} {
		request.nativeconfig.Action, request.Owned = action, &receipt
		if _, err := newKernel().Apply(request); !errors.Is(err, nativeconfig.ErrNotOwned) {
			t.Fatalf("edited native entry accepted by %s: %v", action, err)
		}
		assertBytes(t, path, edited)
	}
}

func TestProjectCodexTOMLRemovesCompleteNestedOwnedEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	const body = "[mcp_servers.managed]\ncommand='TEST-never-executed'\n[mcp_servers.managed.env]\nFIXTURE='inert'\n[mcp_servers.foreign]\nurl='https://foreign.test/keep' # keep\n"
	mustWrite(t, path, body)
	server := nativeconfig.Server{Type: "stdio", Command: "TEST-never-executed", Env: map[string]string{"FIXTURE": "inert"}}
	receipt, err := nativeconfig.DesiredReceipt(path, nativeconfig.CodecCodex, "managed", server, nativeconfig.Placeholders{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newKernel().Apply(nativeconfig.Request{nativeconfig.Paths: nativeconfig.Paths{JSON: path}, Codec: nativeconfig.CodecCodex, nativeconfig.Action: nativeconfig.ActionRemove, Name: "managed", Owned: &receipt}); err != nil {
		t.Fatal(err)
	}
	actual, _ := os.ReadFile(path)
	if strings.Contains(string(actual), "managed") || !strings.Contains(string(actual), "# keep") {
		t.Fatalf("nested native removal changed foreign entry or left owned child: %s", actual)
	}
	if _, err := nativeconfig.DesiredReceipt(path, nativeconfig.CodecCodex, "sse", nativeconfig.Server{Type: "remote", URL: "https://managed.test/mcp", RemoteTransport: "sse"}, nativeconfig.Placeholders{}); err == nil {
		t.Fatal("Codex silently accepted unsupported SSE transport")
	}
}

func newKernel() nativeconfig.Kernel { return nativeconfig.New().WithCodexDocumentParser(Parse) }
func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}
func assertBytes(t *testing.T, path, expected string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil || string(body) != expected {
		t.Fatalf("bytes=%q, error=%v, expected=%q", body, err, expected)
	}
}
