package installer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type claudeSDKFixture struct {
	project, state, user, packageRoot, executable string
	dataRoot                                      string
	failAfterCommit                               bool
	engine                                        *installer.Engine
	runner                                        *claudeSDKRunner
	request                                       installer.Request
}

func claudeSDKWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func newClaudeSDKFixture(t *testing.T, base, project, user, pkg string) *claudeSDKFixture {
	t.Helper()
	f := &claudeSDKFixture{project: project, user: user, packageRoot: pkg, state: filepath.Join(base, "user-uap"), executable: filepath.Join(base, "inert-claude")}
	scope := domain.ScopeUser
	if project != "" {
		f.state = filepath.Join(project, ".uap")
		scope = domain.ScopeProject
	}
	claudeSDKWrite(t, f.executable, "inert test artifact; never executed\n")
	if err := os.Chmod(f.executable, 0700); err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(claude.New())
	if err != nil {
		t.Fatal(err)
	}
	f.runner = &claudeSDKRunner{t: t, user: user, project: project, executable: f.executable}
	f.engine, err = installer.New(installer.Config{ProjectRoot: project, StateRoot: f.state, Registry: registry, Runner: f.runner, HelperExecutable: f.executable, EnableNativeObserver: true, TrustedLocalPackages: true,
		OnCommittedBinding: func(_ context.Context, facts installer.BindingFacts) error {
			if f.failAfterCommit {
				body, err := os.ReadFile(filepath.Join(facts.TargetPath, claude.ClaudeRuntimeDirectory, "skills", "docs", "SKILL.md"))
				if err == nil && strings.Contains(string(body), "Moving source bytes.") {
					f.runner.fail = true
				}
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.request = installer.Request{Scope: scope, ProjectRoot: project, Operation: installer.OpInstall, PackageRoot: pkg, ClientID: "claude", ClientConfigRoot: user, ClientExecutable: f.executable, InstallationID: "00000000-0000-4000-8000-000000000263", OperationID: "claude-project-sdk-install", RequiredComponents: []string{"skills", "mcp"}}
	return f
}

func (f *claudeSDKFixture) apply(t *testing.T, operation installer.Operation, label string) installer.Result {
	t.Helper()
	f.request.Operation, f.request.OperationID = operation, label
	handle, err := f.engine.Prepare(t.Context(), f.request)
	if err != nil {
		t.Fatalf("prepare SDK %s (%s): %v", operation, label, err)
	}
	defer func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	}()
	result, err := f.engine.Apply(t.Context(), handle, installer.Decision{Confirmed: true})
	if err != nil || result.Outcome != installer.OutcomeCompleted && result.Outcome != installer.OutcomeUnchanged {
		t.Fatalf("SDK %s=%+v,%v", operation, result, err)
	}
	if result.Binding.DataRoot != "" {
		if f.dataRoot != "" && f.dataRoot != result.Binding.DataRoot {
			t.Fatal("SDK lifecycle changed persistent data authority")
		}
		f.dataRoot = result.Binding.DataRoot
	}
	return result
}

func (f *claudeSDKFixture) binding(t *testing.T) domain.ClientBinding {
	t.Helper()
	state, err := (statev2.Store{Path: filepath.Join(f.state, "state-v2.json"), ProjectRoot: f.project}).Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("SDK state=%+v,%v", state, err)
	}
	for _, binding := range state.Installations[0].Clients {
		return binding
	}
	t.Fatal("SDK binding absent")
	return domain.ClientBinding{}
}

// This inert runner discovers fixture manifests from the production-selected
// directory. It is a command contract boundary, not genuine Claude evidence.
type claudeSDKRunner struct {
	t                         *testing.T
	user, project, executable string
	fail                      bool
	trusted                   bool
	calls                     int
}

func (r *claudeSDKRunner) Run(_ context.Context, command legacyports.Command) (legacyports.CommandResult, error) {
	r.t.Helper()
	if !reflect.DeepEqual(command.Argv, []string{r.executable, "plugin", "list", "--json"}) {
		r.t.Fatalf("unexpected SDK native command=%+v", command)
	}
	root, scope := r.user, "user"
	if r.project != "" {
		root, scope = filepath.Join(r.project, ".claude"), "project"
		if command.Dir != r.project {
			r.t.Fatalf("SDK project cwd=%q", command.Dir)
		}
	}
	want := "CLAUDE_CONFIG_DIR=" + r.user
	found := false
	for _, entry := range command.Env {
		if entry == want {
			found = true
		}
	}
	if !found {
		r.t.Fatalf("SDK replaced original user config=%q", command.Env)
	}
	if r.fail {
		return legacyports.CommandResult{}, fmt.Errorf("inert unavailable native listing")
	}
	r.calls++
	listing := claudeSDKListing(r.t, r.user, "user")
	if scope == "project" {
		projectRows := claudeSDKListing(r.t, root, "project")
		if len(projectRows) > 0 && !r.trusted {
			listing = append(listing, map[string]any{"id": "(suppressed)@skills-dir", "version": "unknown", "scope": "project", "enabled": false, "installPath": "", "notes": []string{"1 project-scope directory under ./.claude/skills/ that may load as a plugin was skipped because this workspace was not trusted when plugins were scanned. After accepting the trust dialog, run /reload-plugins (or relaunch) to load what qualifies."}})
		} else {
			// Fixture model of the documented same-name user precedence. This
			// is deliberately not represented as genuine trusted native evidence.
			userIDs := map[any]bool{}
			for _, row := range listing {
				userIDs[row["id"]] = true
			}
			for _, row := range projectRows {
				if !userIDs[row["id"]] {
					listing = append(listing, row)
				}
			}
		}
	}

	body, err := json.Marshal(listing)
	return legacyports.CommandResult{Stdout: body}, err
}

func claudeSDKListing(t *testing.T, root, scope string) []map[string]any {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "skills"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	listing := []map[string]any{}
	for _, entry := range entries {
		path := filepath.Join(root, "skills", entry.Name())
		body, err := os.ReadFile(filepath.Join(path, ".claude-plugin", "plugin.json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		var manifest map[string]any
		if err := json.Unmarshal(body, &manifest); err != nil {
			t.Fatal(err)
		}
		listing = append(listing, map[string]any{"id": fmt.Sprint(manifest["name"]) + "@skills-dir", "version": manifest["version"], "scope": scope, "enabled": true, "installPath": path})
	}
	return listing
}
