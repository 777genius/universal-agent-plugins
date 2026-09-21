//go:build linux

package shared

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

func TestLinuxUnqualifiedPluginRootRealLayouts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		seed     func(*testing.T, string, string)
		owned    bool
		want     clients.RegistryFinding
		errText  string
		dangling bool
	}{
		{
			name: "missing-root-is-clear",
			seed: func(*testing.T, string, string) {},
			want: clients.RegistryClear,
		},
		{
			name: "ds-store-is-ignored",
			seed: func(t *testing.T, root, active string) {
				if err := os.MkdirAll(active, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(active, "plugin.json"), []byte(`{"name":"demo"}`), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte("finder"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			owned: true,
			want:  clients.RegistryExpected,
		},
		{
			name: "child-symlink-fails-closed",
			seed: func(t *testing.T, root, active string) {
				if err := os.MkdirAll(root, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(active, filepath.Join(root, "suspicious-link")); err != nil {
					t.Fatal(err)
				}
			},
			want: clients.RegistryIndeterminate,
		},
		{
			name: "dangling-root-fails-closed",
			seed: func(t *testing.T, root, _ string) {
				if err := os.Symlink(filepath.Join(t.TempDir(), "removed-root"), root); err != nil {
					t.Fatal(err)
				}
			},
			dangling: true,
		},
		{
			name: "fifo-root-fails-closed",
			seed: func(t *testing.T, root, _ string) {
				if err := syscall.Mkfifo(root, 0o600); err != nil {
					t.Fatal(err)
				}
			},
			errText: "is not a directory",
		},
		{
			name: "file-root-fails-closed",
			seed: func(t *testing.T, root, _ string) {
				if err := os.WriteFile(root, []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			errText: "is not a directory",
		},
		{
			name: "active-path-file-fails-closed",
			seed: func(t *testing.T, root, active string) {
				if err := os.MkdirAll(root, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(active, []byte("not a package"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			errText: "is not a directory",
		},
		{
			name: "foreign-plugin-is-collision",
			seed: func(t *testing.T, root, _ string) {
				if err := os.MkdirAll(filepath.Join(root, "foreign", ".cursor-plugin"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "foreign", ".cursor-plugin", "plugin.json"), []byte(`{"name":"demo"}`), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			want: clients.RegistryCollision,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), "plugins")
			active := filepath.Join(root, "managed-demo")
			tc.seed(t, root, active)
			finding, err := inspectUnqualifiedWithin(t, root, "demo", active, tc.owned)
			if tc.dangling {
				if err == nil || finding != clients.RegistryIndeterminate || !errors.Is(err, errPluginRootDangling) {
					t.Fatalf("finding=%v err=%v, want dangling root", finding, err)
				}
				return
			}
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

func inspectUnqualifiedWithin(t *testing.T, root, name, active string, owned bool) (clients.RegistryFinding, error) {
	t.Helper()
	type outcome struct {
		finding clients.RegistryFinding
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		finding, err := InspectUnqualifiedPluginRoot(root, name, active, owned)
		done <- outcome{finding, err}
	}()
	select {
	case result := <-done:
		return result.finding, result.err
	case <-time.After(3 * time.Second):
		t.Fatal("InspectUnqualifiedPluginRoot hung")
		return clients.RegistryIndeterminate, nil
	}
}
