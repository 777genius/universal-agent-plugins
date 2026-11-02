package codextoml

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestProjectCodexConcreteCopiesAndLargeIntegers(t *testing.T) {
	const original = "# foreign policy\n[mcp_servers.foreign]\nurl='https://inert.test/mcp'\nlarge=9007199254740993\n"
	input := []byte(original)
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = '!'
	entries, err := doc.MCPEntries()
	if err != nil {
		t.Fatal(err)
	}
	entries["foreign"][0] = '!'
	independent, err := doc.MCPEntries()
	if err != nil || !json.Valid(independent["foreign"]) {
		t.Fatal("concrete entries alias retained document", err)
	}
	noop, err := doc.RenderMCPEntries(independent)
	if err != nil || string(noop) != original {
		t.Fatalf("no-op bytes changed: %q %v", noop, err)
	}
	noop[0] = '!'
	again, err := doc.RenderMCPEntries(independent)
	if err != nil || string(again) != original {
		t.Fatal("no-op output aliases retained document", err)
	}
	independent["managed"] = json.RawMessage(`{"command":"TEST-not-executed","large":9007199254740993}`)
	body, err := doc.RenderMCPEntries(independent)
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]any
	if err := toml.Unmarshal(body, &native); err != nil {
		t.Fatal(err)
	}
	servers := native["mcp_servers"].(map[string]any)
	for _, name := range []string{"foreign", "managed"} {
		if value := servers[name].(map[string]any)["large"]; value != int64(9007199254740993) {
			t.Fatalf("integer changed for %s: %v", name, value)
		}
	}
	if !bytes.Contains(body, []byte("# foreign policy")) {
		t.Fatal("foreign comment lost")
	}
}
