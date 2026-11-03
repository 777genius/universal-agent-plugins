package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestDependencyCheckProjectScopeCLI(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		name := "isolated_project"
		if unsafe {
			name = "unsafe_namespace"
		}
		t.Run(name, func(t *testing.T) {
			user := newDependencyCheckFixture(t)
			root := filepath.Join(user.root, "project")
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			root, err := canonicalProjectRoot(root)
			if err != nil {
				t.Fatal(err)
			}
			project := user
			stateRoot := filepath.Join(root, ".uap")
			project.store = statev2.Store{Path: filepath.Join(stateRoot, "state-v2.json"), ProjectRoot: root}
			project.app.Scope, project.app.ProjectRoot = domain.ScopeProject, root
			project.app.DependenciesRoot = root
			project.app.ManagedRoot = filepath.Join(stateRoot, "managed")
			project.app.EnvironmentStoreRoot = filepath.Join(stateRoot, "store")
			project.app.EnvironmentLockPath = filepath.Join(stateRoot, "environment.lock")
			project.app.StateStore = project.store
			project.app.Installer, err = installer.New(installer.Config{ProjectRoot: root, StateRoot: stateRoot, Registry: project.app.ClientRegistry})
			if err != nil {
				t.Fatal(err)
			}
			pin := seedDependencyCheckPackage(t, project, "demo", "")
			writeDependencyCheckAgreement(t, project, []environment.LockedPackage{pin}, "demo")
			// Invalid user intent must neither influence project inspection nor be repaired.
			manifest, lock := user.app.dependencyPaths()
			dependencyWrite(t, manifest, "invalid user intent: preserve\n")
			dependencyWrite(t, lock, "invalid user lock: preserve\n")
			if unsafe {
				project.app.EnvironmentLockPath = filepath.Join(user.root, "outside-project.lock")
			}
			composed := 0
			user.app.ForScope = func(scope domain.InstallScope, selectedRoot string) (App, error) {
				composed++
				if scope != domain.ScopeProject || selectedRoot != root {
					t.Fatalf("wrong scope composition: %s %s", scope, selectedRoot)
				}
				return project.app, nil
			}
			before := observeDependencyCheckFiles(t, user.root)
			outcome := dependencyCheckValid
			if unsafe {
				outcome = dependencyCheckError
			}
			report, _ := executeDependencyCheck(t, user, outcome, "--scope", "project", "--project-root", root)
			if composed != 1 || report.Scope != "project" {
				t.Fatalf("scope selection was bypassed: calls=%d report=%+v", composed, report)
			}
			if unsafe {
				requireDependencyCheckCode(t, report, "scope_unavailable")
			} else if report.ManifestPath != filepath.Join(root, "uap.toml") || report.LockPath != filepath.Join(root, "uap.lock") {
				t.Fatalf("project check read another agreement: %+v", report)
			}
			assertDependencyCheckFilesUnchanged(t, user.root, before)
		})
	}
}

func TestDependencyCheckCancellationCleansSnapshotCLI(t *testing.T) {
	f := newDependencyCheckFixture(t)
	pin := seedDependencyCheckPackage(t, f, "demo", "")
	writeDependencyCheckAgreement(t, f, []environment.LockedPackage{pin}, "demo")
	before := observeDependencyCheckFiles(t, f.root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var snapshotRoot string
	f.app.PackageLoader = dependencyCheckHookLoader{delegate: f.app.PackageLoader, hook: func(input domain.LoadInput) {
		snapshotRoot = input.SnapshotRoot
		cancel()
	}}
	var output bytes.Buffer
	f.app.Output = &output
	command := NewRoot(f.app)
	command.SetArgs([]string{"deps", "check", "--format", "json"})
	if err := command.ExecuteContext(ctx); !errors.Is(err, errDependencyCheckFailed) {
		t.Fatalf("mid-inspection cancellation did not fail: %v", err)
	}
	var report dependencyCheckReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil || report.Outcome != dependencyCheckCanceled {
		t.Fatalf("canceled report: %s %v", output.String(), err)
	}
	if snapshotRoot == "" {
		t.Fatal("test did not cancel after snapshot creation")
	}
	if _, err := os.Stat(snapshotRoot); !os.IsNotExist(err) {
		t.Fatalf("snapshot leaked after cancellation: %s %v", snapshotRoot, err)
	}
	assertDependencyCheckFilesUnchanged(t, f.root, before)
}
