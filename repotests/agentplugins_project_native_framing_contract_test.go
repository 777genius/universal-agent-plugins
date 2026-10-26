//go:build linux

package pluginkitairepo_test

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
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/cline"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/grok"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/qoder"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

// These are small inert Stager outputs, not acquired vendor trees or runtime
// observations. Each entry is held through the unchanged strict file reader.
func projectNativeFramingEntries(t *testing.T, root string) []packagedigest.CapturedEntry {
	t.Helper()
	var entries []packagedigest.CapturedEntry
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return err
		}
		if entry.IsDir() {
			entries = append(entries, packagedigest.CapturedEntry{Path: filepath.ToSlash(rel), Kind: "directory"})
			return nil
		}
		body, err := projectNativeRuntimeRead(path, 1<<20)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		entries = append(entries, packagedigest.CapturedEntry{Path: filepath.ToSlash(rel), Kind: "file", Executable: info.Mode().Perm()&0111 != 0, Content: body})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestProjectNativeInstalledFramingMatchesActualSelectedStagers(t *testing.T) {
	for _, adapter := range []clients.Adapter{qoder.New(), grok.New(), kiro.New(), windsurf.New(), cline.New()} {
		t.Run(string(adapter.ID()), func(t *testing.T) {
			if domain.ClientTraitsFor(adapter.ID()).QualifiedLocalInstallerFacade {
				t.Fatal("selected public add route no longer uses the lifecycle Stager")
			}
			root := t.TempDir()
			source, project, home := filepath.Join(root, "source"), filepath.Join(root, "A"), filepath.Join(root, "home")
			projectNativeRuntimeWrite(t, filepath.Join(source, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"framing-probe","version":"1.0.0"}`), 0600)
			projectNativeRuntimeWrite(t, filepath.Join(source, "skills", "framing-probe", "SKILL.md"), []byte("---\nname: framing-probe\ndescription: Inert receipt framing proof\n---\nHeld source bytes.\n"), 0600)
			projectNativeRuntimeWrite(t, filepath.Join(source, "bin", "probe"), []byte("inert probe; never executed\n"), 0700)
			if adapter.ID() == domain.ClientKiro || adapter.ID() == domain.ClientWindsurf || adapter.ID() == domain.ClientCline {
				projectNativeRuntimeWrite(t, filepath.Join(source, "mcp.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"framing-probe":{"type":"stdio","command":"./bin/probe","args":["${PLUGIN_ROOT}/arg","${PLUGIN_DATA}"],"cwd":"${PLUGIN_ROOT}"}}}`), 0600)
			}
			for _, path := range []string{project, filepath.Join(home, "."+string(adapter.ID()))} {
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			snapshot, err := (packagedigest.Builder{TempRoot: t.TempDir()}).Snapshot(t.Context(), source, domain.SourceIdentity{})
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
			envelope, err := (loader.Loader{Registry: schemas}).LoadSnapshot(t.Context(), snapshot)
			if err != nil {
				t.Fatal(err)
			}
			sourceEntries := projectNativeFramingEntries(t, snapshot.Root)
			sourceDigest, err := packagedigest.DigestCaptured(t.Context(), sourceEntries)
			if err != nil || sourceDigest != snapshot.TreeDigest || envelope.TreeDigest != sourceDigest {
				t.Fatalf("acquired source identity changed: %s %s %s, %v", sourceDigest, snapshot.TreeDigest, envelope.TreeDigest, err)
			}
			registry, err := clients.NewRegistry(adapter)
			if err != nil {
				t.Fatal(err)
			}
			client := domain.DetectedClient{ClientID: adapter.ID(), ConfigRoot: filepath.Join(home, "."+string(adapter.ID())), ExecutablePath: filepath.Join(root, string(adapter.ID())), Status: domain.DetectionDetected}
			if adapter.ID() == domain.ClientWindsurf {
				client.ExecutablePath, client.ConfigRoot, client.Version = filepath.Join(root, "devin"), "", "3000.11.205"
			}
			planning := planner.Planner{ProjectRoot: project, ManagedRoot: filepath.Join(project, ".uap", "managed"), Paths: pathpolicy.Policy{}, Registry: registry}
			plan, err := planning.Plan(t.Context(), domain.PlanRequest{Envelope: envelope, Client: client, Scope: domain.ScopeProject, ProjectRoot: project, PhysicalArtifactID: "framing-probe-0123456789ab"})
			if err != nil || plan.Status != domain.PlanReady {
				t.Fatalf("actual selected project plan: %+v, %v", plan, err)
			}
			launcherPath := filepath.Join(root, "launcher")
			projectNativeRuntimeWrite(t, launcherPath, []byte("inert managed launcher; never executed\n"), 0700)
			launcher, err := managedstdio.NewSource(launcherPath, "inert-framing")
			if err != nil {
				t.Fatal(err)
			}
			stager := providers.Stager{Registry: registry, Paths: pathpolicy.Policy{}, LauncherSource: launcher, PluginDataRoot: filepath.Join(project, ".uap", "data")}
			delivery, err := stager.Stage(t.Context(), envelope, plan, "inert-framing", domain.CompatibilityHints{})
			if err != nil {
				t.Fatal(err)
			}
			entries := projectNativeFramingEntries(t, delivery.StagingPath)
			got, err := projectNativeRuntimeInstalledDigest(t.Context(), entries)
			if err != nil {
				t.Fatal(err)
			}
			managed := 0
			for _, object := range delivery.NativeObjects {
				if object.Kind == "managed_package_directory" {
					managed++
					if object.ManagedDigest != got || object.Path != plan.ActivePath || stager.Verify(t.Context(), delivery.StagingPath, got) != nil {
						t.Fatalf("held digest differs from actual selected Stager object: %+v, %s", object, got)
					}
				}
			}
			if managed != 1 {
				t.Fatal("actual managed package receipt missing or duplicated")
			}
			newer, err := packagedigest.DigestCaptured(t.Context(), entries)
			if err != nil || newer == got || stager.Verify(t.Context(), delivery.StagingPath, newer) == nil {
				t.Fatal("newer source framing accepted as installed ownership")
			}
			if adapter.ID() == domain.ClientKiro || adapter.ID() == domain.ClientWindsurf {
				if _, err := projectNativeRuntimeRead(filepath.Join(delivery.StagingPath, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName), 1<<20); err != nil {
					t.Fatalf("mixed managed launcher projection was not staged: %v", err)
				}
			}
			if err := stager.Discard(t.Context(), delivery); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProjectNativeInstalledFramingRetainsCapturedPolicyAndDistinctInputs(t *testing.T) {
	valid := []packagedigest.CapturedEntry{{Path: "bin", Kind: "directory"}, {Path: "bin/probe", Kind: "file", Executable: true, Content: []byte("owned")}}
	baseline, err := projectNativeRuntimeInstalledDigest(t.Context(), valid)
	if err != nil {
		t.Fatal(err)
	}
	for _, changed := range [][]packagedigest.CapturedEntry{
		{{Path: "bin", Kind: "directory"}, {Path: "bin/probe", Kind: "file", Executable: true, Content: []byte("other")}},
		{{Path: "bin", Kind: "directory"}, {Path: "bin/probe", Kind: "file", Content: []byte("owned")}},
		{{Path: "bin", Kind: "directory"}, {Path: "bin/probe", Kind: "file", Executable: true, Content: []byte("owned")}, {Path: "empty", Kind: "directory"}},
	} {
		got, err := projectNativeRuntimeInstalledDigest(t.Context(), changed)
		if err != nil || got == baseline {
			t.Fatal("changed content, executable mode or directory membership retained digest")
		}
	}
	for _, invalid := range [][]packagedigest.CapturedEntry{
		{{Path: "../alias", Kind: "file"}}, {{Path: "same", Kind: "file"}, {Path: "same", Kind: "file"}},
		{{Path: "Alias", Kind: "file"}, {Path: "alias", Kind: "file"}}, {{Path: "parent/file", Kind: "file"}},
		{{Path: "file", Kind: "file"}, {Path: "link", Kind: "symlink", Target: "file"}},
	} {
		if _, err := projectNativeRuntimeInstalledDigest(t.Context(), invalid); err == nil {
			t.Fatal("unsafe captured policy or nonordinary entry accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := projectNativeRuntimeInstalledDigest(ctx, valid); err == nil {
		t.Fatal("cancelled held digest continued")
	}
}
