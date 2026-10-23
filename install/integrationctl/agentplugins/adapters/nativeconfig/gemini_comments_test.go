package nativeconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectGeminiCommentedSettingsExactLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	before := "{\n // retain policy comment\n \"skills\":{\"disabled\":[\"foreign\"]},\n \"mcpServers\":{ /* retain foreign entry */ \"foreign\":{\"url\":\"https://foreign.test/mcp\"}}\n}\n"
	mustWrite(t, path, before)
	kernel := New()
	request := Request{Paths: Paths{JSON: path}, Codec: CodecGemini, Action: ActionAdd, Name: "managed", Server: Server{Type: "remote", URL: "https://managed.test/mcp", RemoteTransport: "streamable-http"}}
	receipt, err := kernel.Apply(request)
	if err != nil {
		t.Fatal(err)
	}
	if present, owned, err := kernel.Inspect(request.Paths, request.Codec, request.Name, &receipt); err != nil || !present || !owned {
		t.Fatalf("commented exact inspection = %v/%v, %v", present, owned, err)
	}
	request.Action, request.Owned = ActionUpdate, &receipt
	request.Server.URL = "https://updated.test/mcp"
	receipt, err = kernel.Apply(request)
	if err != nil {
		t.Fatal(err)
	}
	// A foreign writer may add trivia before the preceding member's comma.
	// Removing our last entry must not create a native-invalid trailing comma.
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	foreign := `"foreign":{"url":"https://foreign.test/mcp"}`
	mustWrite(t, path, strings.Replace(string(updated), foreign, foreign+" /* keep new foreign trivia */", 1))
	request.Action, request.Owned = ActionRemove, &receipt
	if _, err := kernel.Apply(request); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, preserved := range []string{"// retain policy comment", "/* retain foreign entry */", "/* keep new foreign trivia */", `"disabled":["foreign"]`, `"foreign":{"url":"https://foreign.test/mcp"}`} {
		if !strings.Contains(string(body), preserved) {
			t.Fatalf("lost %q in %s", preserved, body)
		}
	}
	if present, _, err := kernel.Inspect(request.Paths, request.Codec, request.Name, nil); err != nil || present {
		t.Fatalf("removed project entry still present = %v, %v", present, err)
	}
}

func TestProjectGeminiCommentsDoNotBroadenInvalidJSON(t *testing.T) {
	for name, body := range map[string]string{
		"root comma":        `{"mcpServers":{},}`,
		"nested comma":      `{"mcpServers":{"foreign":{"args":["x",]}}}`,
		"duplicate":         `{"mcpServers":{},"mcpServers":{}}`,
		"malformed comment": "{/* incomplete",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			mustWrite(t, path, body)
			request := Request{Paths: Paths{JSON: path}, Codec: CodecGemini, Action: ActionAdd, Name: "managed", Server: Server{Type: "stdio", Command: "TEST-never-executed"}}
			if _, err := New().Apply(request); !errors.Is(err, ErrMalformed) {
				t.Fatalf("invalid settings accepted: %v", err)
			}
			assertBytes(t, path, body)
		})
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	mustWrite(t, path, "{// comment\n\"mcpServers\":{}}")
	if _, _, err := New().Inspect(Paths{JSON: path}, CodecMCPServers, "managed", nil); !errors.Is(err, ErrMalformed) {
		t.Fatalf("generic JSON codec was broadened: %v", err)
	}
}
