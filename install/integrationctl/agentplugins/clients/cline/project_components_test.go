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

func TestClineProjectObjectsUsesSingleStagedMCPProjectionForSelectedMembers(t *testing.T) {
	project := t.TempDir()
	staging := filepath.Join(project, ".uap", "package")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	projection := ClineProjection{Servers: map[string]nativeconfig.Server{
		"first":   {Type: "stdio", Command: "first-command"},
		"second":  {Type: "stdio", Command: "second-command"},
		"foreign": {Type: "stdio", Command: "foreign-command"},
	}}
	body, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(staging, ClineProjectionFile)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{
		TargetContext: domain.TargetContext{ProjectRoot: project},
		Components: []domain.ComponentDecision{
			{Kind: domain.ComponentMCPServer, Name: "first", Support: domain.SupportPrepared},
			{Kind: domain.ComponentMCPServer, Name: "second", Support: domain.SupportPrepared},
		},
	}
	objects, err := clineProjectObjects(staging, domain.PackageEnvelope{}, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 2 {
		t.Fatalf("expected only the two selected MCP ownership receipts, got %#v", objects)
	}
	for i, name := range []string{"first", "second"} {
		object := objects[i]
		if object.LogicalName != name || object.Kind != clineProjectMCPKind ||
			object.ObjectID != clineProjectMCPKind+":"+name || object.ManagedDigest == "" ||
			object.Path != filepath.Join(project, ".uap", "clients", "cline", "mcp.json") {
			t.Fatalf("invalid selected MCP ownership receipt: %#v", object)
		}
	}
	unchanged, err := os.ReadFile(path)
	if err != nil || string(unchanged) != string(body) {
		t.Fatalf("owned receipt projection modified its staged source: %v", err)
	}
}

func TestClineProjectObjectsMissingSelectedMCPRefusesWithoutWritingConfig(t *testing.T) {
	project := t.TempDir()
	staging := filepath.Join(project, ".uap", "package")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(ClineProjection{Servers: map[string]nativeconfig.Server{
		"foreign": {Type: "stdio", Command: "foreign-command"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(staging, ClineProjectionFile)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{
		TargetContext: domain.TargetContext{ProjectRoot: project},
		Components: []domain.ComponentDecision{{
			Kind: domain.ComponentMCPServer, Name: "selected", Support: domain.SupportPrepared,
		}},
	}
	if _, err := clineProjectObjects(staging, domain.PackageEnvelope{}, plan); err == nil {
		t.Fatal("missing selected source MCP member must fail closed")
	}
	unchanged, err := os.ReadFile(path)
	if err != nil || string(unchanged) != string(body) {
		t.Fatalf("refused ownership changed foreign source projection: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".uap", "clients", "cline", "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("refused ownership created native configuration: %v", err)
	}
}
