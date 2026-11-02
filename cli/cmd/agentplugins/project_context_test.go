package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func projectCompositionFixture(t *testing.T) (agentpluginscli.App, string, string, string) {
	t.Helper()
	home := projectJourneyTemp(t)
	user := filepath.Join(home, "user-uap")
	a, b := projectJourneyTemp(t), projectJourneyTemp(t)
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	app, err := newAgentpluginsCLIApp(home, user, nil, nil, nil, registry)
	if err != nil {
		t.Fatal(err)
	}
	return app, user, a, b
}

func TestProjectRuntimeSeparatesMutableStateAndSharesContentCache(t *testing.T) {
	app, user, a, b := projectCompositionFixture(t)
	for _, root := range []string{a, b} {
		project, err := app.ForScope(domain.ScopeProject, root)
		if err != nil {
			t.Fatal(err)
		}
		if project.Scope != domain.ScopeProject || project.ProjectRoot != root || project.DependenciesRoot != root || project.ManagedRoot != filepath.Join(root, ".uap", "managed") || project.EnvironmentLockPath != filepath.Join(root, ".uap", "environment.lock") {
			t.Fatalf("wrong selected context: %+v", project)
		}
		if project.UserHome != app.UserHome || project.EnvironmentStoreRoot != filepath.Join(user, "store") || project.SourceAcquirer == nil {
			t.Fatal("project composition lost original profile/cache context")
		}
		if project.StateMigrator != nil || project.LegacyLifecycle != nil || project.LegacyStateLock != nil {
			t.Fatal("project composition exposes user migration routes")
		}
		if _, err := project.Installer.Inspect(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(root, ".uap")); !os.IsNotExist(err) {
			t.Fatalf("read-only composition created namespace: %v", err)
		}
	}
	if app.Scope != domain.ScopeUser || app.ProjectRoot != "" || app.DependenciesRoot != user {
		t.Fatal("project selection modified user runtime")
	}
}

func TestProjectCLIInitAndTransferSelectOnlyExplicitProject(t *testing.T) {
	app, user, a, b := projectCompositionFixture(t)
	run := func(args ...string) error {
		t.Helper()
		out := &bytes.Buffer{}
		root := agentpluginscli.NewRoot(app)
		root.SetOut(out)
		root.SetErr(out)
		root.SetArgs(args)
		return root.Execute()
	}
	if err := run("deps", "init", "--scope", "project", "--project-root", a, "--format", "json"); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(a, "uap.toml")
	before, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, other := range []string{user, b} {
		if _, err := os.Stat(filepath.Join(other, "uap.toml")); !os.IsNotExist(err) {
			t.Fatalf("init touched other environment %s: %v", other, err)
		}
	}
	if err := run("deps", "export", a, "--scope", "project", "--project-root", a, "--format", "json"); err == nil || !strings.Contains(err.Error(), "different exchange directory") {
		t.Fatalf("same-pair transfer = %v", err)
	}
	after, err := os.ReadFile(manifest)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("refused transfer changed manifest: %v", err)
	}
	if err := run("deps", "init", "--project-root", b); err == nil || !strings.Contains(err.Error(), "requires --scope project") {
		t.Fatalf("user/root mismatch = %v", err)
	}
}

func TestProjectCLIUnsupportedTargetRejectsBeforeSourceAcquisition(t *testing.T) {
	app, user, a, _ := projectCompositionFixture(t)
	root := agentpluginscli.NewRoot(app)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"add", "https://invalid.example/never-acquire", "--target", "chatgpt", "--no-save", "--scope", "project", "--project-root", a})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "does not support project scope") {
		t.Fatalf("unsupported project boundary = %v", err)
	}
	for _, path := range []string{user, filepath.Join(a, ".uap"), filepath.Join(a, "uap.toml")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("unsupported request created %s: %v", path, err)
		}
	}
}
