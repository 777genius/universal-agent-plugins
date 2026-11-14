//go:build linux

package grok

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

func inventoryInputsFixture(t *testing.T) (*enablementInventoryInputs, enablementFixture) {
	t.Helper()
	fixture := newEnablementFixture(t)
	home := filepath.Dir(fixture.recovery.UserRoot)
	project := fixture.recovery.ProjectRoot
	executable := prepareInventoryInputFixture(t, project)
	profile, err := fixture.recovery.openEnablementProfile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := profile.close(); err != nil {
			t.Error(err)
		}
	})
	lease, err := captureEnablementInventoryInputs(t.Context(), profile, executable, project, home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lease.close(); err != nil {
			t.Error(err)
		}
	})
	return lease, fixture
}

func prepareInventoryInputFixture(t *testing.T, project string) string {
	t.Helper()
	for _, relative := range []string{".git/objects/info", ".git/refs/heads"} {
		if err := os.MkdirAll(filepath.Join(project, relative), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for relative, body := range map[string]string{
		".git/config": "[core]\nrepositoryformatversion = 0\nfilemode = true\nbare = false\nlogallrefupdates = true\n",
		".git/HEAD":   "ref: refs/heads/main\n",
	} {
		if err := os.WriteFile(filepath.Join(project, relative), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	executable := filepath.Join(filepath.Dir(project), "native")
	if err := os.WriteFile(executable, []byte("inert original executable"), 0700); err != nil {
		t.Fatal(err)
	}
	return executable
}

func TestEnablementInventoryInputsKeepOriginalRolesAcrossConfigCAS(t *testing.T) {
	lease, fixture := inventoryInputsFixture(t)
	original := lease.profile.config.Original()
	roles := lease.inputs
	if err := lease.profile.config.Apply(fixture.next); err != nil {
		t.Fatal(err)
	}
	if err := lease.revalidate(context.Background(), original); err == nil {
		t.Fatal("old config postimage admitted")
	}
	current := nativeconfig.FileSnapshot{Body: fixture.next, Mode: 0600, Exists: true}
	if err := lease.revalidate(context.Background(), current); err != nil {
		t.Fatal(err)
	}
	if lease.inputs != roles || lease.inputs.Worktree != lease.inputs.Project {
		t.Fatal("config CAS refreshed original input authority")
	}
	if err := lease.close(); err != nil {
		t.Fatal(err)
	}
	if err := lease.revalidate(context.Background(), current); err == nil {
		t.Fatal("closed input authority reused")
	}
	if !bytes.Equal(lease.profile.config.Original().Body, original.Body) {
		t.Fatal("borrowed profile predecessor changed")
	}
}

func TestEnablementInventoryInputsRefuseParentAndExecutableReplacement(t *testing.T) {
	for _, changed := range []string{"home-parent", "executable"} {
		t.Run(changed, func(t *testing.T) {
			lease, fixture := inventoryInputsFixture(t)
			before, err := os.Stat(fixture.config)
			if err != nil {
				t.Fatal(err)
			}
			if changed == "home-parent" {
				home := lease.inputs.Home.Path
				if err := os.Rename(home, home+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(home, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(home+"-original", ".grok"), fixture.recovery.UserRoot); err != nil {
					t.Fatal(err)
				}
			} else {
				path := lease.inputs.Executable.Path
				if err := os.Rename(path, path+"-original"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("inert original executable"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := lease.revalidate(context.Background(), lease.profile.config.Original()); err == nil {
				t.Fatal("replacement adopted under an original config leaf")
			}
			after, err := os.Stat(fixture.config)
			if err != nil || !os.SameFile(before, after) {
				t.Fatal("test did not retain original config leaf", err)
			}
		})
	}
}

func TestEnablementInventoryInputsKeepAbsentDiscoveryAuthority(t *testing.T) {
	lease, fixture := inventoryInputsFixture(t)
	path := filepath.Join(fixture.recovery.ProjectRoot, "not-present", "registry.json")
	if err := lease.captureAbsent(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	pin := lease.pins[len(lease.pins)-1]
	if err := pin.revalidate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := pin.revalidate(context.Background()); err == nil {
		t.Fatal("new discovery ancestor replaced original absence")
	}
}

func TestEnablementInventoryInputsRejectOutsideDiscoveryPaths(t *testing.T) {
	for _, body := range []string{
		"[plugins]\npaths=['/outside']\n", "[plugins]\ninstall_dir='/outside'\n",
		"[[version_overrides]]\nversion='*'\n", "include='/outside/config'\n",
	} {
		if err := inventoryInputConfig([]byte(body)); err == nil {
			t.Fatal("outside config admitted", body)
		}
	}
	if err := inventoryInputConfig([]byte("[marketplace]\ndefault_skills_installs_purged=true\n[features]\nturn_summary=false\n[plugins]\nenabled=['foreign']\ndisabled=[]\n")); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{"name":"owned","skills":"../../outside"}`, `{"name":"owned","mcpServers":"/outside/mcp.json"}`, `{"name":"owned","hooks":{"path":"/outside"}}`} {
		if err := inventoryInputManifest([]byte(body)); err == nil {
			t.Fatal("outside component admitted", body)
		}
	}
	if err := inventoryInputManifest([]byte(`{"name":"owned","skills":"./skills/","mcpServers":"./.mcp.json"}`)); err != nil {
		t.Fatal(err)
	}
	ordinary := "[core]\nrepositoryformatversion = 0\nfilemode = true\nbare = false\nlogallrefupdates = true\n"
	if !inventoryInputGitConfig([]byte(ordinary)) {
		t.Fatal("ordinary Git config refused")
	}
	for _, suffix := range []string{"[include]\npath=/outside\n", "worktree=/outside\n", "repositoryformatversion=1\n", "[extensions]\nworktreeConfig=true\n"} {
		if inventoryInputGitConfig([]byte(ordinary + suffix)) {
			t.Fatal("outside Git source admitted", suffix)
		}
	}
	if !inventoryInputGitHEAD([]byte("ref: refs/heads/main\n")) {
		t.Fatal("ordinary unborn HEAD refused")
	}
	for _, head := range []string{"", "ref: /outside\n", "ref: refs/heads/../outside\n", "ref: refs/heads/\n", "ref: refs/heads/main\nextra\n"} {
		if inventoryInputGitHEAD([]byte(head)) {
			t.Fatal("invalid HEAD could fall back to an ancestor repository", head)
		}
	}
}

func TestEnablementInventoryInputsCancelBeforeSourceIO(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := captureEnablementInventoryInputs(ctx, nil, "not-a-path", "not-a-path", "not-a-path"); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled acquisition reached source validation", err)
	}
	if _, _, err := inventoryInputRead(ctx, "not-a-path", 1); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled read reached pathname IO", err)
	}
}
