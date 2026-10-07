//go:build linux

package claude

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

func TestLinuxPreparedRegistrySpecialNodesFailClosed(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		seed    func(*testing.T, string, string)
		want    clients.RegistryFinding
		errText string
	}{
		{
			name: "fifo-skills-root",
			seed: func(t *testing.T, root, _ string) {
				if err := syscall.Mkfifo(root, 0o600); err != nil {
					t.Fatal(err)
				}
			},
			errText: "is not a directory",
		},
		{
			name: "file-skills-root",
			seed: func(t *testing.T, root, _ string) {
				mustWrite(t, root, []byte("not a skills directory"))
			},
			errText: "is not a directory",
		},
		{
			name: "socket-skills-root",
			seed: func(t *testing.T, root, _ string) {
				listener, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", root)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = listener.Close() })
			},
			errText: "is not a directory",
		},
		{
			name: "skills-root-symlink-to-file",
			seed: func(t *testing.T, root, _ string) {
				file := filepath.Join(t.TempDir(), "not-skills")
				mustWrite(t, file, []byte("file"))
				mustSymlink(t, file, root)
			},
			errText: "does not resolve to a directory",
		},
		{
			name: "active-path-regular-file",
			seed: func(t *testing.T, root, active string) {
				mustMkdir(t, root)
				mustWrite(t, active, []byte("not a package"))
			},
			errText: "is not a directory",
		},
		{
			name: "active-path-fifo",
			seed: func(t *testing.T, root, active string) {
				mustMkdir(t, root)
				if err := syscall.Mkfifo(active, 0o600); err != nil {
					t.Fatal(err)
				}
			},
			errText: "is not a directory",
		},
		{
			name: "unreadable-child-directory",
			seed: func(t *testing.T, root, _ string) {
				mustMkdir(t, root)
				child := filepath.Join(root, "locked-skill")
				mustMkdir(t, child)
				t.Cleanup(func() { _ = os.Chmod(child, 0o700) })
				if err := os.Chmod(child, 0); err != nil {
					t.Fatal(err)
				}
			},
			errText: "permission denied",
		},
		{
			name: "unreadable-skills-root",
			seed: func(t *testing.T, root, _ string) {
				mustMkdir(t, root)
				t.Cleanup(func() { _ = os.Chmod(root, 0o700) })
				if err := os.Chmod(root, 0); err != nil {
					t.Fatal(err)
				}
			},
			errText: "permission denied",
		},
		{
			name: "three-node-symlink-cycle",
			seed: func(t *testing.T, root, _ string) {
				mustMkdir(t, root)
				mustSymlink(t, "b", filepath.Join(root, "a"))
				mustSymlink(t, "c", filepath.Join(root, "b"))
				mustSymlink(t, "a", filepath.Join(root, "c"))
			},
			errText: "too many levels of symbolic links",
		},
		{
			name: "linked-foreign-plugin-is-collision",
			seed: func(t *testing.T, root, _ string) {
				mustMkdir(t, root)
				foreign := filepath.Join(t.TempDir(), "foreign-plugin")
				mustWrite(t, filepath.Join(foreign, ".claude-plugin", "plugin.json"), []byte(`{"name":"demo"}`))
				mustSymlink(t, foreign, filepath.Join(root, "foreign-demo"))
			},
			want: clients.RegistryCollision,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), "skills")
			active := filepath.Join(root, "managed-demo")
			tc.seed(t, root, active)
			finding, err := inspectPrepared(t, root, active, "demo", false)
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
