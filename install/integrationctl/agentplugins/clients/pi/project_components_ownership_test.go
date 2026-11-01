package pi

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This qualifies the existing ownership kernel for Pi's exact project-native
// paths. It does not enable Skills/MCP in Pi's public project adapter.
func TestPiProjectSkillsAndMCPExistingKernelRejectsForeignOrEditedState(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	projectA := filepath.Join(root, "A")
	projectB := filepath.Join(root, "B")
	user := filepath.Join(root, "user", ".pi", "agent")
	packageRoot := filepath.Join(projectA, ".pi", "extensions", "uap-owned-package")
	skillSource := filepath.Join(packageRoot, "skills", "pi-owned", "SKILL.md")
	for _, dir := range []string{projectA, projectB, user, filepath.Dir(skillSource)} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path string, body []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	const initialSkill = "---\nname: pi-owned\ndescription: Managed Pi project proof\n---\nFirst.\n"
	write(skillSource, []byte(initialSkill))
	pathA := filepath.Join(projectA, ".pi", "mcp.json")
	pathB := filepath.Join(projectB, ".pi", "mcp.json")
	pathUser := filepath.Join(user, "mcp.json")
	foreign := map[string]any{
		"mcpServers": map[string]any{"foreign": map[string]any{"url": "https://foreign.invalid/mcp"}},
		"foreignRootKey": "preserve",
	}
	foreignBytes, err := json.Marshal(foreign)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{pathA, pathB, pathUser} {
		write(path, foreignBytes)
	}
	layout := shared.ProjectComponents{
		ProjectRoot: projectA,
		SkillsRoot: filepath.Join(projectA, ".pi", "skills"),
		ConfigPaths: nativeconfig.Paths{JSON: pathA},
		Codec: nativeconfig.CodecMCPServers,
		SkillKind: "pi_project_skill_directory",
		MCPKind: "pi_project_mcp_server",
	}
	server := nativeconfig.Server{Type: "remote", URL: "https://managed.invalid/one"}
	skill, err := layout.SkillObject("pi-owned", "skills/pi-owned", packageRoot)
	if err != nil {
		t.Fatal(err)
	}
	mcp, err := layout.MCPObject("pi-owned", server, pathA)
	if err != nil {
		t.Fatal(err)
	}
	owned := []domain.NativeObjectOwnership{skill, mcp}
	kernel := nativeconfig.New()
	apply := func(before, after []domain.NativeObjectOwnership, servers map[string]nativeconfig.Server) error {
		return (shared.ProjectComponentMutation{
			Layout: layout, ActivePath: packageRoot, Previous: before,
			Desired: after, Servers: servers, Kernel: kernel,
		}).Apply(context.Background())
	}
	if err := apply(nil, owned, map[string]nativeconfig.Server{"pi-owned": server}); err != nil {
		t.Fatal("existing project component kernel refused Pi's native paths", err)
	}
	if err := layout.Verify(owned, false, kernel); err != nil {
		t.Fatal("Pi project skill and MCP ownership not verifiable", err)
	}
	skillPath := filepath.Join(layout.SkillsRoot, "pi-owned", "SKILL.md")
	if body, err := os.ReadFile(skillPath); err != nil || string(body) != initialSkill {
		t.Fatal("Pi project skill was not installed exactly", err)
	}
	assertForeignPiProjectState(t, pathA, pathB, pathUser, foreignBytes, true)

	const updatedSkill = "---\nname: pi-owned\ndescription: Managed Pi project proof\n---\nUpdated.\n"
	write(skillSource, []byte(updatedSkill))
	server.URL = "https://managed.invalid/two"
	updatedSkillObject, err := layout.SkillObject("pi-owned", "skills/pi-owned", packageRoot)
	if err != nil {
		t.Fatal(err)
	}
	updatedMCP, err := layout.MCPObject("pi-owned", server, pathA)
	if err != nil {
		t.Fatal(err)
	}
	updated := []domain.NativeObjectOwnership{updatedSkillObject, updatedMCP}
	if err := apply(owned, updated, map[string]nativeconfig.Server{"pi-owned": server}); err != nil {
		t.Fatal("exact Pi project update failed", err)
	}
	if err := layout.Verify(updated, false, kernel); err != nil {
		t.Fatal("updated Pi component receipts were not verified", err)
	}
	if body, err := os.ReadFile(skillPath); err != nil || string(body) != updatedSkill {
		t.Fatal("Pi project updated skill not observed", err)
	}
	assertForeignPiProjectState(t, pathA, pathB, pathUser, foreignBytes, true)

	write(skillPath, []byte("foreign user edit\n"))
	beforeRefusal, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(updated, nil, nil); err == nil {
		t.Fatal("foreign edit of owned Pi project skill was erased")
	}
	afterRefusal, err := os.ReadFile(pathA)
	if err != nil || !bytes.Equal(beforeRefusal, afterRefusal) {
		t.Fatal("failed Pi skill removal still modified MCP configuration", err)
	}
	write(skillPath, []byte(updatedSkill))
	if err := apply(updated, nil, nil); err != nil {
		t.Fatal("exact owned Pi skill/MCP cleanup failed", err)
	}
	if _, err := os.Lstat(filepath.Dir(skillPath)); !os.IsNotExist(err) {
		t.Fatal("managed Pi skill survived removal", err)
	}
	assertForeignPiProjectState(t, pathA, pathB, pathUser, foreignBytes, false)
}

func assertForeignPiProjectState(t *testing.T, a, b, user string, original []byte, owned bool) {
	t.Helper()
	for _, path := range []string{b, user} {
		body, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(body, original) {
			t.Fatal("foreign project or user Pi MCP configuration changed", err)
		}
	}
	body, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		MCPServers     map[string]json.RawMessage `json:"mcpServers"`
		ForeignRootKey string                     `json:"foreignRootKey"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	var expected struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(original, &expected); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(document.MCPServers["foreign"], expected.MCPServers["foreign"]) ||
		document.ForeignRootKey != "preserve" {
		t.Fatal("Pi project kernel changed foreign MCP data")
	}
	_, exists := document.MCPServers["pi-owned"]
	if exists != owned {
		t.Fatal("Pi project native MCP member presence differs from exact owned state")
	}
}
