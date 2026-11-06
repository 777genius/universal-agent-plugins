package shared

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func componentWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func componentFixture(t *testing.T) ProjectComponentMutation {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(root, ".uap", "managed", "demo")
	layout := ProjectComponents{ProjectRoot: root, SkillsRoot: filepath.Join(root, ".agents", "skills"),
		ConfigPaths: nativeconfig.Paths{JSON: filepath.Join(root, ".cursor", "mcp.json")},
		Codec:       nativeconfig.CodecMCPServers, SkillKind: "fixture_project_skill", MCPKind: "fixture_project_mcp"}
	componentWrite(t, layout.ConfigPaths.JSON, `{"mcpServers":{"foreign":{"url":"https://foreign.test/mcp","disabled":true}},"policy":"keep"}`)
	componentWrite(t, filepath.Join(active, "skills", "docs", "SKILL.md"), "inert version one\n")
	skill, err := layout.SkillObject("docs", "skills/docs", active)
	if err != nil {
		t.Fatal(err)
	}
	server := nativeconfig.Server{Type: "remote", URL: "https://managed.test/v1"}
	mcp, err := layout.MCPObject("docs", server, layout.ConfigPaths.JSON)
	if err != nil {
		t.Fatal(err)
	}
	return ProjectComponentMutation{Layout: layout, ActivePath: active, Desired: []domain.NativeObjectOwnership{skill, mcp}, Servers: map[string]nativeconfig.Server{"docs": server}, Kernel: nativeconfig.New()}
}

func componentNext(t *testing.T, mutation ProjectComponentMutation) ProjectComponentMutation {
	t.Helper()
	mutation.Previous = append([]domain.NativeObjectOwnership(nil), mutation.Desired...)
	componentWrite(t, filepath.Join(mutation.ActivePath, "skills", "docs", "SKILL.md"), "inert version two\n")
	skill, err := mutation.Layout.SkillObject("docs", "skills/docs", mutation.ActivePath)
	if err != nil {
		t.Fatal(err)
	}
	server := nativeconfig.Server{Type: "remote", URL: "https://managed.test/v2"}
	mcp, err := mutation.Layout.MCPObject("docs", server, mutation.Layout.ConfigPaths.JSON)
	if err != nil {
		t.Fatal(err)
	}
	mutation.Desired, mutation.Servers = []domain.NativeObjectOwnership{skill, mcp}, map[string]nativeconfig.Server{"docs": server}
	return mutation
}

func TestProjectComponentsLifecycleAndMissingSamePinRepair(t *testing.T) {
	mutation := componentFixture(t)
	if err := mutation.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	mutation.Previous = mutation.Desired
	if err := mutation.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(mutation.Desired[0].Path); err != nil {
		t.Fatal(err)
	}
	if err := mutation.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := mutation.Layout.Verify(mutation.Desired, false, mutation.Kernel); err != nil {
		t.Fatal(err)
	}
	mutation = componentNext(t, mutation)
	if err := mutation.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := mutation.Layout.Verify(mutation.Desired, false, mutation.Kernel); err != nil {
		t.Fatal(err)
	}
	mutation.Previous, mutation.Desired, mutation.Servers = mutation.Desired, nil, nil
	for range 2 {
		if err := mutation.Apply(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	body, err := os.ReadFile(mutation.Layout.ConfigPaths.JSON)
	if err != nil || strings.Contains(string(body), "managed.test") || !strings.Contains(string(body), `"disabled":true`) || !strings.Contains(string(body), `"policy":"keep"`) {
		t.Fatalf("remove lost foreign data: %s, %v", body, err)
	}
	if _, err := os.Stat(filepath.Join(mutation.Layout.SkillsRoot, "docs")); !os.IsNotExist(err) {
		t.Fatalf("skill survived remove: %v", err)
	}
}

func TestProjectComponentsPrewriteTamperRefuses(t *testing.T) {
	for _, test := range []string{"extra-server", "missing-server", "source-content", "source-relative", "outside-active", "receipt"} {
		t.Run(test, func(t *testing.T) {
			mutation := componentFixture(t)
			before, _ := os.ReadFile(mutation.Layout.ConfigPaths.JSON)
			switch test {
			case "extra-server":
				mutation.Servers["extra"] = mutation.Servers["docs"]
			case "missing-server":
				delete(mutation.Servers, "docs")
			case "source-content":
				componentWrite(t, filepath.Join(mutation.ActivePath, "skills", "docs", "SKILL.md"), "foreign drift")
			case "source-relative":
				mutation.Desired[0].SourceRelative = "skills/../docs"
			case "outside-active":
				mutation.ActivePath = t.TempDir()
			case "receipt":
				mutation.Desired[1].ManagedDigest = "sha256:foreign"
			}
			if err := mutation.Apply(context.Background()); err == nil {
				t.Fatal("tampered projection admitted")
			}
			actual, _ := os.ReadFile(mutation.Layout.ConfigPaths.JSON)
			if string(actual) != string(before) {
				t.Fatal("prewrite refusal changed MCP config")
			}
			if _, err := os.Lstat(mutation.Desired[0].Path); !os.IsNotExist(err) {
				t.Fatalf("prewrite refusal installed skill: %v", err)
			}
		})
	}
}

func TestProjectComponentsContainedAliasesRefuse(t *testing.T) {
	for _, target := range []string{"skills", "config"} {
		t.Run(target, func(t *testing.T) {
			mutation := componentFixture(t)
			destination := filepath.Join(mutation.Layout.ProjectRoot, "real-"+target)
			if err := os.Mkdir(destination, 0700); err != nil {
				t.Fatal(err)
			}
			alias := filepath.Join(mutation.Layout.ProjectRoot, "alias-"+target)
			if err := os.Symlink(destination, alias); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if target == "skills" {
				mutation.Layout.SkillsRoot = alias
			} else {
				mutation.Layout.ConfigPaths.JSON = filepath.Join(alias, "mcp.json")
			}
			if err := mutation.Layout.ValidateObjects(nil); err == nil {
				t.Fatal("contained native alias admitted")
			}
		})
	}
}

func TestProjectComponentsOwnershipSpellingRefuses(t *testing.T) {
	for _, relative := range []string{"skills/docs.", "skills/CON", "skills/a?b", "skills/\xff", "skills/cafe\u0301", "skills//docs", "skills/./docs", "skills/../docs"} {
		t.Run(relative, func(t *testing.T) {
			mutation := componentFixture(t)
			mutation.Desired[0].SourceRelative = relative
			if err := mutation.Layout.ValidateObjects(mutation.Desired); err == nil {
				t.Fatalf("nonportable source accepted: %q", relative)
			}
		})
	}
	mutation := componentFixture(t)
	object := &mutation.Desired[0]
	object.LogicalName = " docs "
	object.ObjectID = object.Kind + ":" + object.LogicalName
	object.Path = filepath.Join(mutation.Layout.SkillsRoot, object.LogicalName)
	if err := mutation.Layout.ValidateObjects(mutation.Desired); err == nil {
		t.Fatal("padded logical name admitted")
	}
}

func TestProjectComponentsMCPOnlyDoesNotCreateSkillTransaction(t *testing.T) {
	mutation := componentFixture(t)
	mutation.Desired = mutation.Desired[1:]
	if err := mutation.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	mutation.Previous = mutation.Desired
	server := nativeconfig.Server{Type: "remote", URL: "https://managed.test/mcp-only-v2"}
	object, err := mutation.Layout.MCPObject("docs", server, mutation.Layout.ConfigPaths.JSON)
	if err != nil {
		t.Fatal(err)
	}
	mutation.Desired, mutation.Servers = []domain.NativeObjectOwnership{object}, map[string]nativeconfig.Server{"docs": server}
	if err := mutation.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := mutation.Layout.Verify(mutation.Desired, false, mutation.Kernel); err != nil {
		t.Fatal(err)
	}
	mutation.Previous, mutation.Desired, mutation.Servers = mutation.Desired, nil, nil
	if err := mutation.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(mutation.Layout.ConfigPaths.JSON)
	if err != nil || strings.Contains(string(body), "managed.test") || !strings.Contains(string(body), `"disabled":true`) {
		t.Fatalf("MCP-only foreign preservation failed: %s, %v", body, err)
	}
	if _, err := os.Lstat(mutation.Layout.SkillsRoot); !os.IsNotExist(err) {
		t.Fatalf("MCP-only request created skill transaction/root: %v", err)
	}
}

func TestProjectComponentsMCPJsonNameWithSpacesLifecycle(t *testing.T) {
	mutation := componentFixture(t)
	name := " deployment notes "
	server := nativeconfig.Server{Type: "remote", URL: "https://managed.test/v1"}
	first, err := mutation.Layout.MCPObject(name, server, mutation.Layout.ConfigPaths.JSON)
	if err != nil {
		t.Fatalf("JSON member selector was treated as a filesystem leaf: %v", err)
	}
	mutation.Desired = []domain.NativeObjectOwnership{first}
	mutation.Servers = map[string]nativeconfig.Server{name: server}
	for _, url := range []string{"https://managed.test/v1", "https://managed.test/v2"} {
		if url != server.URL {
			mutation.Previous = append([]domain.NativeObjectOwnership(nil), mutation.Desired...)
			server.URL = url
			next, err := mutation.Layout.MCPObject(name, server, mutation.Layout.ConfigPaths.JSON)
			if err != nil {
				t.Fatal(err)
			}
			mutation.Desired = []domain.NativeObjectOwnership{next}
			mutation.Servers[name] = server
		}
		if err := mutation.Apply(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := mutation.Layout.Verify(mutation.Desired, false, mutation.Kernel); err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(mutation.Layout.ConfigPaths.JSON)
		if err != nil || !strings.Contains(string(body), url) || !strings.Contains(string(body), `"foreign"`) {
			t.Fatalf("native JSON member was lost: %q, %v", body, err)
		}
	}
	mutation.Previous = append([]domain.NativeObjectOwnership(nil), mutation.Desired...)
	mutation.Desired, mutation.Servers = nil, nil
	if err := mutation.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(mutation.Layout.ConfigPaths.JSON)
	if err != nil || strings.Contains(string(body), `" deployment notes "`) || !strings.Contains(string(body), `"foreign"`) {
		t.Fatalf("owned JSON selector removal changed foreign data: %q, %v", body, err)
	}
}

func TestProjectComponentsSkillNamesStayFilesystemSafe(t *testing.T) {
	mutation := componentFixture(t)
	skill := mutation.Desired[0]
	skill.LogicalName = "unsafe skill"
	skill.ObjectID = mutation.Layout.SkillKind + ":" + skill.LogicalName
	skill.Path = filepath.Join(mutation.Layout.SkillsRoot, skill.LogicalName)
	if err := mutation.Layout.ValidateObjects([]domain.NativeObjectOwnership{skill}); err == nil {
		t.Fatal("skill directory accepted a space-bearing filesystem leaf")
	}
}

func TestProjectComponentsForeignSkillRefusalPreservesForeignFiles(t *testing.T) {
	mutation := componentFixture(t)
	occupied := filepath.Join(mutation.Desired[0].Path, "SKILL.md")
	neighbor := filepath.Join(mutation.Layout.SkillsRoot, "foreign-neighbor", "SKILL.md")
	componentWrite(t, occupied, "foreign selected skill\\n")
	componentWrite(t, neighbor, "foreign neighbor\\n")
	beforeConfig, err := os.ReadFile(mutation.Layout.ConfigPaths.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if err := mutation.Apply(context.Background()); !errors.Is(err, domain.ErrForeignSkillDestination) {
		t.Fatalf("foreign project Skill refused without typed ownership cause: %v", err)
	} else {
		var located *domain.ForeignSkillDestinationError
		if errors.As(err, &located) {
			t.Fatalf("non-Codex client acquired Codex-specific occupied path: %+v", located)
		}
	}
	for path, want := range map[string]string{
		occupied: "foreign selected skill\\n",
		neighbor: "foreign neighbor\\n",
	} {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != want {
			t.Fatalf("refusal changed foreign Skill %q: %q, %v", path, got, err)
		}
	}
	gotConfig, err := os.ReadFile(mutation.Layout.ConfigPaths.JSON)
	if err != nil || string(gotConfig) != string(beforeConfig) {
		t.Fatalf("refusal changed foreign MCP settings: %v", err)
	}
}

func TestProjectComponentsPreflightForeignSkillCarriesTypedCause(t *testing.T) {
	mutation := componentFixture(t)
	// Codex receives the new typed source-specific UI refusal; generic clients
	// retain their existing RegistryCollision finding without an error.
	mutation.Layout.Codec = nativeconfig.CodecCodex
	path := mutation.Desired[0].Path
	componentWrite(t, filepath.Join(path, "SKILL.md"), "foreign instructions\n")
	selected := []domain.ComponentDecision{{
		Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportProjected,
	}}
	finding, err := mutation.Layout.Inspect(selected, nil, nativeconfig.New())
	if finding != clients.RegistryCollision || !errors.Is(err, domain.ErrForeignSkillDestination) {
		t.Fatalf("preflight foreign Skill result = %v, %v", finding, err)
	}
	var located *domain.ForeignSkillDestinationError
	if !errors.As(err, &located) || located.RelativePath != ".agents/skills/docs/SKILL.md" {
		t.Fatalf("foreign destination is not a safe project-relative Skill path: %+v", located)
	}
	if got, err := os.ReadFile(filepath.Join(path, "SKILL.md")); err != nil || string(got) != "foreign instructions\n" {
		t.Fatalf("preflight changed foreign bytes: %q, %v", got, err)
	}
}

func TestProjectComponentsExternalExactManagedPackageBoundary(t *testing.T) {
	for _, location := range []string{"default-state", "relocated-state"} {
		t.Run(location, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			home := filepath.Join(base, "home")
			nativeRoot := filepath.Join(home, ".config", "devin")
			stateRoot := filepath.Join(home, ".local", "state", "uap")
			if location == "relocated-state" {
				stateRoot = filepath.Join(base, "independent", "uap-state")
			}
			active := filepath.Join(stateRoot, "managed", "clients", "windsurf", "artifact")
			layout := ProjectComponents{
				ProjectRoot:        filepath.Join(home, ".config"),
				SkillsRoot:         filepath.Join(nativeRoot, "skills"),
				ManagedPackagePath: active,
				ConfigPaths:        nativeconfig.Paths{JSON: filepath.Join(nativeRoot, "mcp_config.json")},
				Codec:              nativeconfig.CodecDevin,
				SkillKind:          "test_devin_global_skill", MCPKind: "test_devin_global_mcp",
			}
			componentWrite(t, filepath.Join(active, "skills", "docs", "SKILL.md"), "managed source\n")
			componentWrite(t, layout.ConfigPaths.JSON, `{"mcpServers":{"foreign":{"url":"https://foreign.test/mcp","transport":"http"}},"policy":"keep"}`)
			legacy := filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")
			componentWrite(t, legacy, "legacy-cascade-unchanged\n")
			projectA := filepath.Join(base, "project-A", ".devin", "mcp_config.json")
			projectB := filepath.Join(base, "project-B", ".devin", "mcp_config.json")
			componentWrite(t, projectA, "project-A-unchanged\n")
			componentWrite(t, projectB, "project-B-unchanged\n")
			skill, err := layout.SkillObject("docs", "skills/docs", active)
			if err != nil {
				t.Fatal(err)
			}
			server := nativeconfig.Server{Type: "remote", URL: "https://managed.test/mcp", RemoteTransport: "streamable-http"}
			mcp, err := layout.MCPObject("docs", server, layout.ConfigPaths.JSON)
			if err != nil {
				t.Fatal(err)
			}
			mutation := ProjectComponentMutation{Layout: layout, ActivePath: active,
				Desired: []domain.NativeObjectOwnership{skill, mcp},
				Servers: map[string]nativeconfig.Server{"docs": server}, Kernel: nativeconfig.New()}
			wrong := filepath.Join(stateRoot, "managed", "clients", "windsurf", "other")
			componentWrite(t, filepath.Join(wrong, "skills", "docs", "SKILL.md"), "foreign source\n")
			if _, err := layout.SkillObject("docs", "skills/docs", wrong); err == nil {
				t.Fatal("independent managed package alias admitted")
			}
			altered := mutation
			altered.ActivePath = wrong
			if err := altered.Apply(context.Background()); err == nil {
				t.Fatal("foreign package supplied to mutation")
			}
			outside := layout
			outside.SkillsRoot = filepath.Join(base, "unrelated", "skills")
			if err := outside.ValidateObjects(nil); err == nil {
				t.Fatal("native output outside selected config authority admitted")
			}
			if err := mutation.Apply(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := mutation.Layout.Verify(mutation.Desired, false, mutation.Kernel); err != nil {
				t.Fatal(err)
			}
			mutation.Previous = append([]domain.NativeObjectOwnership(nil), mutation.Desired...)
			if err := os.RemoveAll(skill.Path); err != nil {
				t.Fatal(err)
			}
			if err := mutation.Apply(context.Background()); err != nil {
				t.Fatalf("exact owned repair: %v", err)
			}
			mutation.Desired, mutation.Servers = nil, nil
			if err := mutation.Apply(context.Background()); err != nil {
				t.Fatalf("exact owned removal: %v", err)
			}
			if _, err := os.Lstat(skill.Path); !os.IsNotExist(err) {
				t.Fatalf("managed skill survived removal: %v", err)
			}
			config, err := os.ReadFile(layout.ConfigPaths.JSON)
			if err != nil || !strings.Contains(string(config), "foreign.test") ||
				!strings.Contains(string(config), `"policy":"keep"`) || strings.Contains(string(config), "managed.test") {
				t.Fatalf("foreign native members were modified: %s, %v", config, err)
			}
			for path, want := range map[string]string{
				legacy: "legacy-cascade-unchanged\n", projectA: "project-A-unchanged\n", projectB: "project-B-unchanged\n",
			} {
				body, err := os.ReadFile(path)
				if err != nil || string(body) != want {
					t.Fatalf("other native profile changed at %q: %q, %v", path, body, err)
				}
			}
		})
	}
}
