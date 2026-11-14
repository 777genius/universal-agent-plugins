package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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
	if err := syncArtifacts(path, want, true); err != nil {
		t.Fatal(err)
	}
}

func TestProjectMetadataMatchesAdmittedComponentContracts(t *testing.T) {
	generated, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	var rows []clientCapability
	if err := json.Unmarshal(generated.capabilities, &rows); err != nil {
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

func TestNewClientAppearsInBothGeneratedContracts(t *testing.T) {
	metadata := append(capabilityMetadata(), clientCapability{ID: "fixture-next-agent"})
	generated, err := renderArtifacts(metadata)
	if err != nil {
		t.Fatal(err)
	}
	var rows []clientCapability
	if err := json.Unmarshal(generated.capabilities, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(metadata) || rows[len(rows)-1].ID != "fixture-next-agent" {
		t.Fatal("new client missing from capability contract")
	}
	if !strings.Contains(string(generated.identities), "  'fixture-next-agent',\n") {
		t.Fatal("new client missing from TypeScript identity contract")
	}
}

func TestInvalidClientIdentitiesRefuseGeneration(t *testing.T) {
	for _, id := range []domain.ClientID{"codex", "", "Bad-ID", "empty--segment", "trailing-", "injected';"} {
		t.Run(string(id), func(t *testing.T) {
			metadata := append(capabilityMetadata(), clientCapability{ID: id})
			if _, err := renderArtifacts(metadata); err == nil {
				t.Fatal("invalid or duplicate identity accepted")
			}
		})
	}
}

func TestContractCheckDetectsDriftWithoutWriting(t *testing.T) {
	generated, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{"clientCapabilities.generated.json", "clientIds.generated.ts"} {
		t.Run(filename, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "clientCapabilities.generated.json")
			if err := syncArtifacts(output, generated, false); err != nil {
				t.Fatal(err)
			}
			if err := syncArtifacts(output, generated, true); err != nil {
				t.Fatal(err)
			}
			changed := filepath.Join(filepath.Dir(output), filename)
			if err := os.WriteFile(changed, []byte("stale\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := syncArtifacts(output, generated, true); err == nil {
				t.Fatal("stale contract passed check")
			}
			current, err := os.ReadFile(changed)
			if err != nil || string(current) != "stale\n" {
				t.Fatal("check mode rewrote stale contract", err)
			}
			if err := os.Remove(changed); err != nil {
				t.Fatal(err)
			}
			if err := syncArtifacts(output, generated, true); err == nil {
				t.Fatal("missing contract passed check")
			}
		})
	}
}
