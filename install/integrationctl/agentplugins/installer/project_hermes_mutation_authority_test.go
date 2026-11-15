//go:build !windows

package installer_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/hermes"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

type hermesMutationSwap struct {
	*hermes.Adapter
	t        *testing.T
	root     string
	remove   bool
	swapped  bool
	original []byte
}

type hermesMutationPublisher struct {
	ports.NativeProjectRootPublisher
	swap *hermesMutationSwap
}

func (publisher hermesMutationPublisher) PublishProjectRoot(ctx context.Context, req domain.ActivationRequest, spec ports.NativeProjectRoot) error {
	if err := publisher.NativeProjectRootPublisher.PublishProjectRoot(ctx, req, spec); err != nil {
		return err
	}
	if !publisher.swap.remove {
		publisher.swap.replace()
	}
	return nil
}

func (publisher hermesMutationPublisher) VerifyProjectRoot(ctx context.Context, path string, object domain.NativeObjectOwnership) error {
	if err := publisher.NativeProjectRootPublisher.VerifyProjectRoot(ctx, path, object); err != nil {
		return err
	}
	if publisher.swap.remove {
		publisher.swap.replace()
	}
	return nil
}

func (swap *hermesMutationSwap) replace() {
	swap.t.Helper()
	if swap.swapped {
		return
	}
	swap.swapped = true
	entries, err := os.ReadDir(swap.root)
	if err != nil {
		swap.t.Fatal(err)
	}
	swap.original, err = os.ReadFile(filepath.Join(swap.root, "config.yaml"))
	if err != nil {
		swap.t.Fatal(err)
	}
	if err := os.Rename(swap.root, swap.root+"-original"); err != nil {
		swap.t.Fatal(err)
	}
	if err := os.Mkdir(swap.root, 0700); err != nil {
		swap.t.Fatal(err)
	}
	for _, entry := range entries {
		body, err := os.ReadFile(filepath.Join(swap.root+"-original", entry.Name()))
		if err != nil {
			swap.t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(swap.root+"-original", entry.Name()))
		if err != nil {
			swap.t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(swap.root, entry.Name()), body, info.Mode().Perm()); err != nil {
			swap.t.Fatal(err)
		}
	}
}

func (swap *hermesMutationSwap) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	env.ProjectRoots = hermesMutationPublisher{env.ProjectRoots, swap}
	return swap.Adapter.Activate(ctx, env, req)
}

func (swap *hermesMutationSwap) Deactivate(ctx context.Context, env clients.Env, req domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	env.ProjectRoots = hermesMutationPublisher{env.ProjectRoots, swap}
	return swap.Adapter.Deactivate(ctx, env, req)
}

// Only the boundary scheduler is injected. Publication receipts, native kernel,
// filesystem locks, YAML projection and the SDK transaction remain real.
func TestProjectHermesMutationRejectsIdenticalRootAfterPublicationCheck(t *testing.T) {
	for _, operation := range []installer.Operation{installer.OpUpdate, installer.OpRemove} {
		t.Run(string(operation), func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			pkg := filepath.Join(base, "package")
			claudeSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sdk-hermes","version":"1.0.0"}`)
			claudeSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Inert fixture\n---\nOriginal.\n")
			claudeSDKWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://example.test/one"}}}`)
			fixture := newHermesSDKFixture(t, filepath.Join(base, "project"), filepath.Join(base, "user"), pkg)
			fixture.apply(t, installer.OpInstall, "first-install")
			swap := &hermesMutationSwap{Adapter: hermes.New(), t: t, root: filepath.Join(fixture.project, ".hermes", "uap-session"), remove: operation == installer.OpRemove}
			registry, err := clients.NewRegistry(swap)
			if err != nil {
				t.Fatal(err)
			}
			helper, _ := os.Executable()
			fixture.engine, err = installer.New(installer.Config{ProjectRoot: fixture.project, StateRoot: filepath.Join(fixture.project, ".uap"), Registry: registry, Runner: fixture.runner, EnableNativeObserver: true, TrustedLocalPackages: true, HelperExecutable: helper})
			if err != nil {
				t.Fatal(err)
			}
			claudeSDKWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://example.test/two"}}}`)
			fixture.request.Operation, fixture.request.OperationID = operation, "substituted-root"
			prepared, err := fixture.engine.Prepare(t.Context(), fixture.request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := prepared.Close(); err != nil {
					t.Error(err)
				}
			}()
			if _, err := fixture.engine.Apply(t.Context(), prepared, installer.Decision{Confirmed: true}); err == nil || !swap.swapped {
				t.Fatal("identical foreign root was accepted", err, swap.swapped)
			}
			for _, root := range []string{swap.root, swap.root + "-original"} {
				body, err := os.ReadFile(filepath.Join(root, "config.yaml"))
				if err != nil || !bytes.Equal(body, swap.original) {
					t.Fatal("mutation or compensation changed original/replacement config", root, err)
				}
			}
		})
	}
}
