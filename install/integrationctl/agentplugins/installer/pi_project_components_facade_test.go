package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/pi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The public Engine must own exact native Pi project Skills/MCP entries, not
// merely stage the files into its sealed package tree.
func TestPiProjectNativeSkillsMCPFacadeAAndB(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user, projectA, projectB := filepath.Join(root, "user", ".pi", "agent"), filepath.Join(root, "project-A"), filepath.Join(root, "project-B")
	source := filepath.Join(root, "source")
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	manifest := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"uap-pi-project-components","version":"1.0.0"}`
	sourceMCP := func(url string) {
		write(filepath.Join(source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"pi-managed":{"type":"streamable-http","url":"`+url+`"}}}`)
	}
	skill := func(text string) {
		write(filepath.Join(source, "skills", "pi-managed", "SKILL.md"), "---\nname: pi-managed\ndescription: Project-native Pi Skills/MCP E2E\n---\n"+text+"\n")
	}
	write(filepath.Join(source, "plugin.json"), manifest)
	sourceMCP("http://127.0.0.1:9/first")
	skill("First revision.")
	foreign := `{"other":"retain","mcpServers":{"foreign":{"url":"https://foreign.invalid/mcp"}}}` + "\n"
	for _, path := range []string{
		filepath.Join(user, "mcp.json"),
		filepath.Join(projectA, ".pi", "mcp.json"),
		filepath.Join(projectB, ".pi", "mcp.json"),
	} {
		write(path, foreign)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	setup := func(project string) *Engine {
		t.Helper()
		registry, err := clients.NewRegistry(pi.NewProject())
		if err != nil {
			t.Fatal(err)
		}
		engine, err := New(Config{
			ProjectRoot: project, StateRoot: filepath.Join(project, ".uap"),
			Registry: registry, HelperExecutable: helper, TrustedLocalPackages: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		return engine
	}
	a, b := setup(projectA), setup(projectB)
	do := func(engine *Engine, request Request) Result {
		t.Helper()
		prepared, err := engine.Prepare(context.Background(), request)
		if err != nil {
			t.Fatal("Pi project prepare:", request.Operation, err)
		}
		result, applyErr := engine.Apply(context.Background(), prepared, Decision{Confirmed: true})
		if err := prepared.Close(); err != nil {
			t.Fatal("Pi project close:", err)
		}
		if applyErr != nil || result.Outcome != OutcomeCompleted {
			t.Fatal("Pi project apply:", request.Operation, result.Outcome, applyErr)
		}
		return result
	}
	request := func(op Operation, project, id string) Request {
		return Request{
			Operation: op, Scope: domain.ScopeProject, ProjectRoot: project,
			PackageRoot: source, ClientID: "pi", ClientConfigRoot: user,
			InstallationID: id,
		}
	}
	firstA := do(a, request(OpInstall, projectA, "TEST-pi-project-components-A"))
	firstB := do(b, request(OpInstall, projectB, "TEST-pi-project-components-B"))
	skillA := filepath.Join(projectA, ".pi", "skills", "pi-managed", "SKILL.md")
	skillB := filepath.Join(projectB, ".pi", "skills", "pi-managed", "SKILL.md")
	if _, err := os.Stat(skillA); err != nil {
		t.Fatal("Pi native project Skill missing:", err)
	}
	beforeB, err := os.ReadFile(skillB)
	if err != nil {
		t.Fatal(err)
	}
	mcpA := filepath.Join(projectA, ".pi", "mcp.json")
	mcpB := filepath.Join(projectB, ".pi", "mcp.json")
	beforeMCPB, err := os.ReadFile(mcpB)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(beforeMCPB, []byte("")) || bytes.Equal(beforeMCPB, []byte(foreign)) {
		// A managed member must really have been installed in B.
		t.Fatal("Pi native MCP entry not committed")
	}
	if firstA.Binding.Scope != "project" || firstB.Binding.Scope != "project" {
		t.Fatal("Pi component bindings lost project authority")
	}
	sourceMCP("http://127.0.0.1:9/second")
	skill("Second revision.")
	do(a, request(OpUpdate, projectA, firstA.InstallationID))
	if data, err := os.ReadFile(skillA); err != nil || !bytes.Contains(data, []byte("Second revision.")) {
		t.Fatal("Pi project A Skill update missing", err)
	}
	if data, err := os.ReadFile(skillB); err != nil || !bytes.Equal(data, beforeB) {
		t.Fatal("Pi project B Skill changed with A", err)
	}
	if data, err := os.ReadFile(mcpB); err != nil || !bytes.Equal(data, beforeMCPB) {
		t.Fatal("Pi project B MCP changed with A", err)
	}
	// A missing managed Skill means the entire owned directory is absent.
	// Deleting only SKILL.md leaves an edited directory, which correctly
	// fails closed instead of silently adopting its remaining foreign files.
	if err := os.RemoveAll(filepath.Dir(skillA)); err != nil {
		t.Fatal(err)
	}
	do(a, request(OpRepair, projectA, firstA.InstallationID))
	if _, err := os.Stat(skillA); err != nil {
		t.Fatal("Pi project A Skill repair missing", err)
	}
	managedMCP, err := os.ReadFile(mcpA)
	if err != nil {
		t.Fatal(err)
	}
	write(skillA, "user modified the Skill\n")
	if prepared, err := a.Prepare(context.Background(), request(OpRemove, projectA, firstA.InstallationID)); err == nil {
		result, applyErr := a.Apply(context.Background(), prepared, Decision{Confirmed: true})
		_ = prepared.Close()
		if applyErr == nil && result.Outcome == OutcomeCompleted {
			t.Fatal("modified Pi Skill was deleted without ownership")
		}
	}
	if after, err := os.ReadFile(mcpA); err != nil || !bytes.Equal(after, managedMCP) {
		t.Fatal("Pi MCP mutated despite Skill edit refusal", err)
	}
	skill("Second revision.")
	write(skillA, "---\nname: pi-managed\ndescription: Project-native Pi Skills/MCP E2E\n---\nSecond revision.\n")
	do(a, request(OpRemove, projectA, firstA.InstallationID))
	if actual, err := os.ReadFile(mcpA); err != nil || bytes.Contains(actual, []byte(`"pi-managed"`)) || !bytes.Contains(actual, []byte(`"foreign"`)) {
		t.Fatal("Pi project A MCP removal did not remove exactly its owned server", err)
	}
	if _, err := os.Stat(skillA); !os.IsNotExist(err) {
		t.Fatal("owned Pi project Skill survived removal", err)
	}
	if _, err := os.Stat(skillB); err != nil {
		t.Fatal("Pi project B Skill removed with A", err)
	}
	for _, file := range []string{mcpB, filepath.Join(user, "mcp.json")} {
		expected := []byte(foreign)
		if file == mcpB {
			expected = beforeMCPB
		}
		if actual, err := os.ReadFile(file); err != nil || !bytes.Equal(actual, expected) {
			t.Fatal("foreign Pi project/user configuration altered", err)
		}
	}
}
