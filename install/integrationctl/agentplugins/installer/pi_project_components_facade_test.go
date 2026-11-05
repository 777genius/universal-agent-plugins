package installer

import (
	"bytes"
	"context"
	"fmt"
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

// The same public facade and component kernel used by the Project A/B test
// must bind global objects to the original physical Pi profile. The project
// directories below are canaries: they are never eligible user targets.
func TestPiUserNativeSkillsMCPFacadeAndForeignIsolation(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{filepath.Join(root, "home", ".pi", "agent"), filepath.Join(root, "custom-pi-profile")} {
		t.Run(filepath.Base(filepath.Dir(profile))+"-"+filepath.Base(profile), func(t *testing.T) {
			source, state := filepath.Join(root, "source-"+filepath.Base(profile)), filepath.Join(root, "state-"+filepath.Base(profile))
			projectA, projectB := filepath.Join(root, "A-"+filepath.Base(profile)), filepath.Join(root, "B-"+filepath.Base(profile))
			write := func(path, body string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(source, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"uap-pi-global-components","version":"1.0.0"}`)
			skillBody := func(revision string) string {
				return "---\nname: pi-managed\ndescription: Pi global scoped facade E2E\n---\n" + revision + "\n"
			}
			write(filepath.Join(source, "skills", "pi-managed", "SKILL.md"), skillBody("first"))
			servers := func(url string) {
				write(filepath.Join(source, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"pi-managed":{"type":"streamable-http","url":"`+url+`"}}}`)
			}
			servers("https://example.invalid/first")
			foreign := `{"other":"keep","mcpServers":{"foreign":{"url":"https://foreign.invalid/mcp"}}}` + "\n"
			write(filepath.Join(profile, "mcp.json"), foreign)
			write(filepath.Join(profile, "extensions", "foreign.ts"), "// foreign user extension\n")
			for _, project := range []string{projectA, projectB} {
				write(filepath.Join(project, ".pi", "mcp.json"), foreign)
				write(filepath.Join(project, ".pi", "skills", "existing", "SKILL.md"), "foreign project skill\n")
			}
			registry, err := clients.NewRegistry(pi.NewProject())
			if err != nil {
				t.Fatal(err)
			}
			helper, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			// The host must see the durable user binding before it publishes
			// authority. A Pi user NativeActivationAttempt at this point is
			// the original Linux/macOS regression, not recoverable host state.
			var engine *Engine
			handoffs := 0
			config := Config{StateRoot: state, Registry: registry,
				HelperExecutable: helper, TrustedLocalPackages: true}
			config.OnCommittedBinding = func(_ context.Context, facts BindingFacts) error {
				if facts.ClientID != "pi" || facts.Scope != string(domain.ScopeUser) {
					return fmt.Errorf("unexpected Pi user host handoff: %+v", facts)
				}
				persisted, err := engine.store.Load()
				if err != nil {
					return err
				}
				if len(persisted.Installations) != 1 {
					return fmt.Errorf("Pi host handoff has %d persisted installations", len(persisted.Installations))
				}
				for _, installation := range persisted.Installations {
					for _, binding := range installation.Clients {
						if binding.ClientID == domain.ClientPi && binding.NativeActivationAttempt != "" {
							return fmt.Errorf("Pi user native attempt preceded host handoff: %s", binding.NativeActivationAttempt)
						}
					}
				}
				handoffs++
				return nil
			}
			engine, err = New(config)
			if err != nil {
				t.Fatal(err)
			}
			request := func(op Operation, id string) Request {
				return Request{Operation: op, Scope: domain.ScopeUser,
					ClientID: "pi", ClientConfigRoot: profile, PackageRoot: source,
					InstallationID: id}
			}
			do := func(req Request) Result {
				t.Helper()
				prepared, err := engine.Prepare(context.Background(), req)
				if err != nil {
					t.Fatal("Pi global prepare:", err)
				}
				result, applyErr := engine.Apply(context.Background(), prepared, Decision{Confirmed: true})
				closeErr := prepared.Close()
				if applyErr != nil || closeErr != nil || result.Outcome != OutcomeCompleted {
					t.Fatalf("Pi global %s: %+v apply=%v close=%v", req.Operation, result, applyErr, closeErr)
				}
				return result
			}
			// Both foreign directory owners and foreign MCP keys must refuse
			// before a new global binding claims them. The unrelated foreign
			// entry must survive both failed attempts byte for byte.
			refuse := func(reason string) {
				t.Helper()
				prepared, err := engine.Prepare(context.Background(), request(OpInstall, "TEST-pi-global"))
				if err != nil {
					return
				}
				result, applyErr := engine.Apply(context.Background(), prepared, Decision{Confirmed: true})
				closeErr := prepared.Close()
				if closeErr != nil {
					t.Fatal(closeErr)
				}
				if applyErr == nil && result.Outcome == OutcomeCompleted {
					t.Fatal("Pi global adopted foreign " + reason)
				}
			}
			occupiedSkill := filepath.Join(profile, "skills", "pi-managed", "SKILL.md")
			write(occupiedSkill, "foreign Skill\n")
			refuse("Skill")
			if body, err := os.ReadFile(occupiedSkill); err != nil || string(body) != "foreign Skill\n" {
				t.Fatal("Pi global foreign Skill changed after refusal", err)
			}
			if err := os.RemoveAll(filepath.Dir(occupiedSkill)); err != nil {
				t.Fatal(err)
			}
			occupiedMCP := `{"other":"keep","mcpServers":{"foreign":{"url":"https://foreign.invalid/mcp"},"pi-managed":{"url":"https://occupied.invalid/mcp"}}}` + "\n"
			write(filepath.Join(profile, "mcp.json"), occupiedMCP)
			refuse("MCP")
			if body, err := os.ReadFile(filepath.Join(profile, "mcp.json")); err != nil || string(body) != occupiedMCP {
				t.Fatal("Pi global foreign MCP member changed after refusal", err)
			}
			write(filepath.Join(profile, "mcp.json"), foreign)
			installed := do(request(OpInstall, "TEST-pi-global"))
			if handoffs == 0 {
				t.Fatal("Pi user installation bypassed OnCommittedBinding")
			}
			// Reopen the persisted state with the same host handoff contract.
			// All subsequent update, repair and removal go through a fresh SDK.
			engine, err = New(config)
			if err != nil {
				t.Fatal("reopen Pi user engine:", err)
			}
			if installed.Binding.Scope != string(domain.ScopeUser) || filepath.Dir(installed.Binding.TargetPath) != filepath.Join(profile, "extensions") {
				t.Fatal("global Pi binding is not under the selected physical profile")
			}
			if _, err := os.Lstat(filepath.Join(installed.Binding.TargetPath, "index.ts")); !os.IsNotExist(err) {
				t.Fatal("extension-less global Skill/MCP accidentally became executable")
			}
			skill := filepath.Join(profile, "skills", "pi-managed", "SKILL.md")
			if body, err := os.ReadFile(skill); err != nil || !bytes.Equal(body, []byte(skillBody("first"))) {
				t.Fatal("Pi global skill was not installed", err)
			}
			mcp := filepath.Join(profile, "mcp.json")
			if body, err := os.ReadFile(mcp); err != nil || !bytes.Contains(body, []byte("pi-managed")) || !bytes.Contains(body, []byte("foreign")) {
				t.Fatal("Pi global MCP was not installed alongside foreign configuration", err)
			}
			write(filepath.Join(source, "skills", "pi-managed", "SKILL.md"), skillBody("second"))
			servers("https://example.invalid/second")
			do(request(OpUpdate, installed.InstallationID))
			if body, err := os.ReadFile(skill); err != nil || !bytes.Equal(body, []byte(skillBody("second"))) {
				t.Fatal("Pi global owned Skill was not updated", err)
			}
			if err := os.RemoveAll(filepath.Dir(skill)); err != nil {
				t.Fatal(err)
			}
			do(request(OpRepair, installed.InstallationID))
			if _, err := os.Stat(skill); err != nil {
				t.Fatal("Pi global exact missing Skill was not repaired", err)
			}
			managedMCP, err := os.ReadFile(mcp)
			if err != nil {
				t.Fatal(err)
			}
			write(skill, "foreign edit\n")
			prepared, err := engine.Prepare(context.Background(), request(OpRemove, installed.InstallationID))
			if err == nil {
				result, applyErr := engine.Apply(context.Background(), prepared, Decision{Confirmed: true})
				_ = prepared.Close()
				if applyErr == nil && result.Outcome == OutcomeCompleted {
					t.Fatal("edited-owned Pi global Skill was erased")
				}
			}
			if actual, err := os.ReadFile(mcp); err != nil || !bytes.Equal(actual, managedMCP) {
				t.Fatal("failed Global removal mutated Pi MCP", err)
			}
			write(skill, skillBody("second"))
			do(request(OpRemove, installed.InstallationID))
			if _, err := os.Lstat(skill); !os.IsNotExist(err) {
				t.Fatal("global Skill survived exact removal")
			}
			if body, err := os.ReadFile(mcp); err != nil || bytes.Contains(body, []byte("pi-managed")) || !bytes.Contains(body, []byte("foreign")) {
				t.Fatal("global MCP removal deleted foreign member or retained owned server", err)
			}
			if body, err := os.ReadFile(filepath.Join(profile, "extensions", "foreign.ts")); err != nil || string(body) != "// foreign user extension\n" {
				t.Fatal("foreign user Pi extension changed", err)
			}
			for _, project := range []string{projectA, projectB} {
				if body, err := os.ReadFile(filepath.Join(project, ".pi", "mcp.json")); err != nil || !bytes.Equal(body, []byte(foreign)) {
					t.Fatal("Pi global changed independent project MCP", err)
				}
				if body, err := os.ReadFile(filepath.Join(project, ".pi", "skills", "existing", "SKILL.md")); err != nil || string(body) != "foreign project skill\n" {
					t.Fatal("Pi global changed independent project Skill", err)
				}
			}
		})
	}
}
