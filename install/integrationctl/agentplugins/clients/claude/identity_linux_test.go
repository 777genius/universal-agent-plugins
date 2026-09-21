//go:build linux

package claude

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestLinuxPreparedRegistryRealSkillLayouts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		seed    func(*testing.T, string, string)
		owned   bool
		want    clients.RegistryFinding
		errText string
	}{
		{
			name: "missing-root-is-clear",
			seed: func(*testing.T, string, string) {},
			want: clients.RegistryClear,
		},
		{
			name: "relative-dangling-skill-is-ignored",
			seed: func(t *testing.T, root, _ string) {
				mustMkdir(t, root)
				mustSymlink(t, "removed-relative-skill", filepath.Join(root, "stale-skill"))
			},
			want: clients.RegistryClear,
		},
		{
			name: "absolute-dangling-skill-is-ignored",
			seed: func(t *testing.T, root, _ string) {
				mustMkdir(t, root)
				mustSymlink(t, filepath.Join(t.TempDir(), "removed-shared-skill"), filepath.Join(root, "stale-skill"))
			},
			want: clients.RegistryClear,
		},
		{
			name: "mixed-real-layout-stale-links-fifo-and-plain-skill",
			seed: func(t *testing.T, root, _ string) {
				mustMkdir(t, root)
				mustWrite(t, filepath.Join(root, ".DS_Store"), []byte("finder"))
				mustWrite(t, filepath.Join(root, "notes.txt"), []byte("not a skill"))
				plain := filepath.Join(root, "plain-skill")
				mustMkdir(t, plain)
				mustWrite(t, filepath.Join(plain, "SKILL.md"), []byte("# shared\n"))
				linked := filepath.Join(t.TempDir(), "linked-skill")
				mustMkdir(t, linked)
				mustWrite(t, filepath.Join(linked, "SKILL.md"), []byte("# linked\n"))
				mustSymlink(t, linked, filepath.Join(root, "linked-skill"))
				mustSymlink(t, "gone", filepath.Join(root, "stale-relative"))
				mustSymlink(t, filepath.Join(t.TempDir(), "gone-abs"), filepath.Join(root, "stale-absolute"))
				mustSymlink(t, "gone", filepath.Join(root, "skill with spaces"))
				if err := syscall.Mkfifo(filepath.Join(root, "queue.fifo"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: clients.RegistryClear,
		},
		{
			name: "dangling-active-path-fails-closed",
			seed: func(t *testing.T, root, active string) {
				mustMkdir(t, root)
				mustSymlink(t, filepath.Join(t.TempDir(), "missing-active"), active)
			},
			errText: "symlink target does not exist",
		},
		{
			name: "relative-dangling-active-path-fails-closed",
			seed: func(t *testing.T, root, active string) {
				mustMkdir(t, root)
				mustSymlink(t, "missing-active-package", active)
			},
			errText: "symlink target does not exist",
		},
		{
			name: "circular-skill-link-fails-closed",
			seed: func(t *testing.T, root, _ string) {
				mustMkdir(t, root)
				mustSymlink(t, "loop", filepath.Join(root, "loop"))
			},
			errText: "too many levels of symbolic links",
		},
		{
			name: "unrelated-file-symlink-fails-closed",
			seed: func(t *testing.T, root, _ string) {
				mustMkdir(t, root)
				file := filepath.Join(t.TempDir(), "not-a-skill")
				mustWrite(t, file, []byte("file"))
				mustSymlink(t, file, filepath.Join(root, "file-link"))
			},
			errText: "does not resolve to a directory",
		},
		{
			name: "owned-active-directory-is-expected",
			seed: func(t *testing.T, root, active string) {
				mustMkdir(t, root)
				mustWrite(t, filepath.Join(active, ".claude-plugin", "plugin.json"), []byte(`{"name":"demo"}`))
			},
			owned: true,
			want:  clients.RegistryExpected,
		},
		{
			name: "foreign-plugin-directory-is-collision",
			seed: func(t *testing.T, root, _ string) {
				mustMkdir(t, root)
				mustWrite(t, filepath.Join(root, "foreign-demo", ".claude-plugin", "plugin.json"), []byte(`{"name":"demo"}`))
			},
			errText: "",
			want:    clients.RegistryCollision,
		},
		{
			name: "dangling-skills-root-fails-closed",
			seed: func(t *testing.T, root, _ string) {
				mustSymlink(t, filepath.Join(t.TempDir(), "removed-skills"), root)
			},
			errText: "symlink target does not exist",
		},
		{
			name: "skills-root-symlink-to-real-dir-with-stale-child",
			seed: func(t *testing.T, root, _ string) {
				realRoot := filepath.Join(t.TempDir(), "real-skills")
				mustMkdir(t, realRoot)
				mustSymlink(t, "gone", filepath.Join(realRoot, "stale-skill"))
				mustSymlink(t, realRoot, root)
			},
			want: clients.RegistryClear,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parent := t.TempDir()
			root := filepath.Join(parent, "skills")
			active := filepath.Join(root, "managed-demo")
			tc.seed(t, root, active)
			finding, err := inspectPrepared(root, active, "demo", tc.owned)
			if tc.errText != "" {
				if err == nil || finding != clients.RegistryIndeterminate || !strings.Contains(err.Error(), tc.errText) {
					t.Fatalf("finding=%v err=%v, want indeterminate containing %q", finding, err, tc.errText)
				}
				return
			}
			if err != nil || finding != tc.want {
				t.Fatalf("finding=%v err=%v, want %v", finding, err, tc.want)
			}
		})
	}
}

func TestLinuxPreparedRegistryIgnoresDanglingChainThatIsNotActivePath(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "skills")
	mustMkdir(t, root)
	missing := filepath.Join(t.TempDir(), "removed")
	mid := filepath.Join(t.TempDir(), "mid-link")
	mustSymlink(t, missing, mid)
	mustSymlink(t, mid, filepath.Join(root, "stale-chain"))
	finding, err := inspectPrepared(root, filepath.Join(root, "managed-demo"), "demo", false)
	if err != nil || finding != clients.RegistryClear {
		t.Fatalf("finding=%v err=%v", finding, err)
	}
}

func TestLinuxIgnoreUnrelatedDanglingDoesNotApplyToActivePath(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "skills")
	active := filepath.Join(root, "managed-demo")
	if !ignoreUnrelatedDanglingClaudeSkill(errClaudeSkillSymlinkDangling, filepath.Join(root, "stale"), active) {
		t.Fatal("unrelated dangling skill was not ignored")
	}
	if ignoreUnrelatedDanglingClaudeSkill(errClaudeSkillSymlinkDangling, active, active) {
		t.Fatal("active-path dangling skill was ignored")
	}
}

func inspectPrepared(root, active, name string, owned bool) (clients.RegistryFinding, error) {
	return (*Adapter)(nil).InspectPreparedRegistry(domain.DeliveryPlan{
		ClientID:     domain.ClientClaude,
		DeclaredName: name,
		TargetRoot:   root,
		ActivePath:   active,
	}, name, owned)
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string, body []byte) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}
