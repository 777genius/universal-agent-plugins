package cline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestClineProjectServersSelectsOneProjectionForMultipleOwnedMembers(t *testing.T) {
	root := t.TempDir()
	projection := ClineProjection{Servers: map[string]nativeconfig.Server{
		"first":   {Type: "stdio", Command: "first-command"},
		"second":  {Type: "stdio", Command: "second-command"},
		"foreign": {Type: "stdio", Command: "foreign-command"},
	}}
	body, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ClineProjectionFile), body, 0o600); err != nil {
		t.Fatal(err)
	}
	objects := []domain.NativeObjectOwnership{
		{Kind: clineProjectSkillKind, LogicalName: "skill"},
		{Kind: clineProjectMCPKind, LogicalName: "first"},
		{Kind: clineProjectMCPKind, LogicalName: "second"},
	}
	servers, err := clineProjectServers(root, objects)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 || servers["first"].Command != "first-command" || servers["second"].Command != "second-command" {
		t.Fatalf("selected MCP members differ: %#v", servers)
	}
	if _, exists := servers["foreign"]; exists {
		t.Fatal("unselected foreign MCP entry must not be installed")
	}
}

func TestClineProjectServersSkillsOnlyRequiresNoMCPProjection(t *testing.T) {
	servers, err := clineProjectServers(t.TempDir(), []domain.NativeObjectOwnership{
		{Kind: clineProjectSkillKind, LogicalName: "skill"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 0 {
		t.Fatalf("skills-only request unexpectedly selected MCP: %#v", servers)
	}
}

func TestClineProjectServersRefusesMissingOrIncompleteMCPProjection(t *testing.T) {
	root := t.TempDir()
	objects := []domain.NativeObjectOwnership{{Kind: clineProjectMCPKind, LogicalName: "required"}}
	if _, err := clineProjectServers(root, objects); err == nil {
		t.Fatal("missing selected MCP projection must fail closed")
	}
	body, err := json.Marshal(ClineProjection{Servers: map[string]nativeconfig.Server{
		"foreign": {Type: "stdio", Command: "foreign-command"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ClineProjectionFile), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := clineProjectServers(root, objects); err == nil {
		t.Fatal("projection without the selected MCP member must fail closed")
	}
	unchanged, err := os.ReadFile(filepath.Join(root, ClineProjectionFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != string(body) {
		t.Fatal("rejected selection changed foreign projection")
	}
}
