//go:build linux

package pluginkitairepo_test

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

func cursorCycleDefaultWriterFrame(t *testing.T) (*cursorMCPProjectCycle, string, map[string][]byte) {
	t.Helper()
	root := t.TempDir()
	config := filepath.Join(root, "mcp.json")
	projectNativeRuntimeWrite(t, filepath.Join(root, "foreign-owned.txt"), []byte("caller-owned a\n"), 0600)
	_, err := nativeconfig.New().Apply(nativeconfig.Request{Paths: nativeconfig.Paths{JSON: config}, Codec: nativeconfig.CodecMCPServers, Action: nativeconfig.ActionAdd, Name: "uapcursora", Server: nativeconfig.Server{Type: "remote", URL: "http://127.0.0.1:9/mcp"}})
	if err != nil {
		t.Fatal(err)
	}
	body, err := projectNativeRuntimeRead(config, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := cursorCycleWriterLock(config)
	if err != nil || lock != filepath.Join(root, "mcp.json.agentplugins.lock") {
		t.Fatal("default writer path differs", err)
	}
	info, err := os.Lstat(lock)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() != 0 {
		t.Fatal("default writer did not create the exact empty ordinary 0600 lock", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) || stat.Gid != uint32(os.Getgid()) || stat.Nlink != 1 {
		t.Fatal("default writer lock authority differs")
	}
	f := &cursorProjectAttempt{ctx: context.Background(), held: map[string]cursorProjectHeld{}}
	c := &cursorMCPProjectCycle{f: f, dirs: map[string]os.FileInfo{}, trees: map[string]map[string][]byte{}}
	old := map[string][]byte{"mcp.json": body, "foreign-owned.txt": []byte("caller-owned a\n")}
	if c.acceptTreeError(root, old, false) == nil {
		t.Fatal("original two-file frame accepted the default writer's persistent lock")
	}
	files := map[string][]byte{"mcp.json": body, "foreign-owned.txt": []byte("caller-owned a\n"), "mcp.json.agentplugins.lock": {}}
	if err := c.acceptTreeError(root, files, false); err != nil || c.checkError() != nil {
		t.Fatal("exact source-defined writer frame refused", err)
	}
	return c, root, files
}

func TestCursorCycleWriterFrameMatchesDefaultKernel(t *testing.T) {
	cursorCycleDefaultWriterFrame(t)
}

func TestCursorCycleWriterFrameRetainsExactAuthority(t *testing.T) {
	for _, fault := range []string{"unknown-file", "unknown-directory", "body", "mode", "replacement", "alias", "hardlink", "missing", "late-timestamp"} {
		t.Run(fault, func(t *testing.T) {
			c, root, files := cursorCycleDefaultWriterFrame(t)
			lock := filepath.Join(root, "mcp.json.agentplugins.lock")
			var err error
			switch fault {
			case "unknown-file":
				err = os.WriteFile(filepath.Join(root, "unknown.lock"), nil, 0600)
			case "unknown-directory":
				err = os.Mkdir(filepath.Join(root, "unknown"), 0700)
			case "body":
				err = os.WriteFile(lock, []byte("foreign"), 0600)
			case "mode":
				err = os.Chmod(lock, 0400)
			case "replacement", "alias":
				other := filepath.Join(t.TempDir(), "replacement")
				projectNativeRuntimeWrite(t, other, nil, 0600)
				if fault == "replacement" {
					err = os.Rename(other, lock)
				} else if err = os.Remove(lock); err == nil {
					err = os.Symlink(other, lock)
				}
			case "hardlink":
				err = os.Link(lock, filepath.Join(t.TempDir(), "second-link"))
			case "missing":
				err = os.Remove(lock)
			case "late-timestamp":
				stamp := time.Unix(100, 0)
				err = os.Chtimes(lock, stamp, stamp)
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.acceptTreeError(root, files, false) == nil || c.checkError() == nil {
				t.Fatal("writer frame adopted a late membership or authority change")
			}
		})
	}
}
