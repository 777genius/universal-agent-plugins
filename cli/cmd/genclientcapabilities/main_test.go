package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The landing ships static metadata: detect SDK changes before it can publish
// stale support or transport claims.
func TestGeneratedCapabilitiesAreCurrent(t *testing.T) {
	want, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate generator source")
	}
	path := filepath.Join(filepath.Dir(source), "../../../landing/data/clientCapabilities.generated.json")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("landing client capabilities are stale; run from cli: go run ./cmd/genclientcapabilities")
	}
}

func TestProjectMetadataMatchesAdmittedComponentContracts(t *testing.T) {
	generated, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	var rows []clientCapability
	if err := json.Unmarshal(generated, &rows); err != nil {
		t.Fatal(err)
	}
	byClient := make(map[string]clientCapability, len(rows))
	for _, row := range rows {
		byClient[string(row.ID)] = row
	}
	if byClient["chatgpt"].Project != nil {
		t.Fatal("remote ChatGPT has no native Project filesystem route")
	}
	for _, client := range []string{"codex", "grok", "qoder", "windsurf", "hermes", "antigravity", "pi"} {
		if byClient[client].Project == nil {
			t.Fatal(client, "lost scoped client adapter metadata")
		}
	}
	for client, support := range map[string]string{
		"grok": "native", "qoder": "native", "windsurf": "prepared",
	} {
		if got := string(byClient[client].Project.MCPTransports["stdio"]); got != support {
			t.Fatalf("%s project MCP support = %s, expected %s", client, got, support)
		}
	}
	if byClient["hermes"].Project.MCPTransports["stdio"] != "unsupported" ||
		byClient["antigravity"].Project.MCPTransports["stdio"] != "native" ||
		byClient["antigravity"].Project.MCPTransports["streamable-http"] != "native" {
		t.Fatal("Hermes/Antigravity Project metadata diverged from scoped native contracts")
	}
	if byClient["pi"].Project.ExtensionSupport != "native" ||
		byClient["codex"].Project.ExtensionSupport != "unsupported" {
		t.Fatal("Project extension interpreter support is not scoped")
	}
}
