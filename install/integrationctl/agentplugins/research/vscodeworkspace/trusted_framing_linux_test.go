//go:build linux

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func trustedFramingWrite(t *testing.T, path string, body []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, mode); err != nil {
		t.Fatal(err)
	}
}

// No native executable runs: actual adapters, planning and staging produce the
// receipts independently of the trusted fixture's held-byte expectations.
func TestTrustedInstalledFramingMatchesActualVSCodeStagers(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			source, project, profile := filepath.Join(root, "source"), filepath.Join(root, "A"), filepath.Join(root, "original-user-Code")
			trustedFramingWrite(t, filepath.Join(source, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"uap-vscode-framing","version":"1.0.0"}`), 0600)
			trustedFramingWrite(t, filepath.Join(source, "skills", "uap-vscode-framing", "SKILL.md"), []byte("---\nname: uap-vscode-framing\ndescription: Inert source and receipt identity\n---\nOwned source.\n"), 0600)
			if scope == domain.ScopeProject {
				trustedFramingWrite(t, filepath.Join(source, "bin", "probe"), []byte("inert probe; never executed\n"), 0700)
				trustedFramingWrite(t, filepath.Join(source, "mcp.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"uap-vscode-framing":{"type":"stdio","command":"./bin/probe","args":["${PLUGIN_ROOT}/arg","${PLUGIN_DATA}"],"cwd":"${PLUGIN_ROOT}"}}}`), 0600)
			}
			for _, path := range []string{project, profile} {
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			snapshot, err := (packagedigest.Builder{TempRoot: t.TempDir()}).Snapshot(ctx, source, domain.SourceIdentity{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := packagedigest.Remove(snapshot); err != nil {
					t.Error(err)
				}
			})
			schemas, err := specregistry.New()
			if err != nil {
				t.Fatal(err)
			}
			envelope, err := (loader.Loader{Registry: schemas}).LoadSnapshot(ctx, snapshot)
			if err != nil {
				t.Fatal(err)
			}
			sourceDigest, err := trustedPackageDigest(ctx, source)
			if err != nil || sourceDigest != snapshot.TreeDigest || envelope.TreeDigest != sourceDigest {
				t.Fatalf("acquired source identity changed: %s %s %s, %v", sourceDigest, snapshot.TreeDigest, envelope.TreeDigest, err)
			}
			var adapter clients.Adapter = vscode.New()
			managed, selectedProject := filepath.Join(root, "user-managed"), ""
			if scope == domain.ScopeProject {
				adapter, managed, selectedProject = vscode.NewProject(), filepath.Join(project, ".uap", "managed"), project
			}
			if domain.ClientTraitsFor(adapter.ID()).QualifiedLocalInstallerFacade {
				t.Fatal("VS Code public add no longer uses the lifecycle Stager")
			}
			registry, err := clients.NewRegistry(adapter)
			if err != nil {
				t.Fatal(err)
			}
			client := domain.DetectedClient{ClientID: domain.ClientVSCode, ConfigRoot: profile, ExecutablePath: filepath.Join(root, "code"), Status: domain.DetectionDetected}
			planning := planner.Planner{ProjectRoot: selectedProject, ManagedRoot: managed, Paths: pathpolicy.Policy{}, Registry: registry}
			plan, err := planning.Plan(ctx, domain.PlanRequest{Envelope: envelope, Client: client, Scope: scope, ProjectRoot: selectedProject, PhysicalArtifactID: "uap-vscode-framing-0123456789ab"})
			if err != nil || plan.Status == domain.PlanUnsupported {
				t.Fatalf("actual VS Code %s plan refused: %+v, %v", scope, plan, err)
			}
			launcherPath := filepath.Join(root, "launcher")
			trustedFramingWrite(t, launcherPath, []byte("inert managed launcher; never executed\n"), 0700)
			launcher, err := managedstdio.NewSource(launcherPath, "inert-trusted-framing")
			if err != nil {
				t.Fatal(err)
			}
			stager := providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}, LauncherSource: launcher, PluginDataRoot: filepath.Join(project, ".uap", "plugin-data")}
			delivery, err := stager.Stage(ctx, envelope, plan, "inert-trusted-framing", domain.CompatibilityHints{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := stager.Discard(ctx, delivery); err != nil {
					t.Error(err)
				}
			})
			installed, err := trustedInstalledPackageDigest(ctx, delivery.StagingPath)
			if err != nil {
				t.Fatal(err)
			}
			managedObjects := 0
			for _, object := range delivery.NativeObjects {
				if object.Kind == "managed_package_directory" {
					managedObjects++
					if object.ManagedDigest != installed || object.Path != plan.ActivePath || stager.Verify(ctx, delivery.StagingPath, installed) != nil {
						t.Fatalf("held digest differs from actual VS Code staged receipt: %+v, %s", object, installed)
					}
				}
			}
			newer, err := trustedPackageDigest(ctx, delivery.StagingPath)
			if managedObjects != 1 || err != nil || newer == installed || stager.Verify(ctx, delivery.StagingPath, newer) == nil {
				t.Fatal("source framing accepted for installed ownership or managed receipt missing")
			}
			if scope == domain.ScopeProject {
				if _, err := readOrdinary(filepath.Join(delivery.StagingPath, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName), 1<<20); err != nil {
					t.Fatalf("mixed managed launcher was not staged: %v", err)
				}
			}
		})
	}
}

func TestTrustedInstalledFramingPreservesChangedBytesAndLinkRefusal(t *testing.T) {
	for _, change := range []string{"content", "executable", "directory", "symlink", "hardlink"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "owned")
			trustedFramingWrite(t, file, []byte("owned bytes"), 0600)
			baseline, err := trustedInstalledPackageDigest(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "content":
				err = os.WriteFile(file, []byte("changed bytes"), 0600)
			case "executable":
				err = os.Chmod(file, 0700)
			case "directory":
				err = os.Mkdir(filepath.Join(root, "empty"), 0700)
			case "symlink":
				err = os.Symlink(file, filepath.Join(root, "alias"))
			case "hardlink":
				err = os.Link(file, filepath.Join(root, "alias"))
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := trustedInstalledPackageDigest(context.Background(), root)
			if change == "symlink" || change == "hardlink" {
				if err == nil {
					t.Fatal("nonordinary installed tree was accepted")
				}
			} else if err != nil || got == baseline {
				t.Fatal("changed installed bytes, executable mode or directory membership retained identity")
			}
		})
	}
}
