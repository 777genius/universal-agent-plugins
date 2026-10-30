//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
)

func (c *cursorMCPProjectCycle) server(r *cursorCycleRecipe) map[string]any {
	return map[string]any{"command": filepath.Join(r.Active, "io.github.777genius.agentplugins", "managed-stdio-v1", "agentplugins"), "args": []string{"--internal-stdio-v1", r.Active, r.Data.Locator, r.Active, "plugin", "--argv-base64-v1", "Li9iaW4vcHJvYmU=", "Y3Vyc29yLWNvbXBvbmVudA=="}, "env": map[string]string{"PLUGIN_ROOT": r.Active, "PLUGIN_DATA": r.Data.Locator, "UAP_TEST_ROOT": c.f.base.Root, "UAP_TEST_EVENTS": c.f.base.Events, "UAP_TEST_NONCE": r.Nonce, "UAP_TEST_REVISION": r.Revision}}
}

func cursorCycleJSON(value any) []byte {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic("fixed Cursor cycle JSON recipe")
	}
	return append(body, '\n')
}

func (c *cursorMCPProjectCycle) projected(r *cursorCycleRecipe) map[string][]byte {
	metadata, _ := json.Marshal(struct {
		Protocol int    `json:"protocol"`
		SHA      string `json:"sha256"`
		Version  string `json:"cliVersion"`
	}{1, clineAttemptSHA(c.installerBody), "0.1.0-development"})
	return map[string][]byte{"plugin.json": r.Authored["plugin.json"], "bin/probe": c.probeBody, "mcp.json": cursorCycleJSON(map[string]any{"mcpServers": map[string]any{"uapcursora": c.server(r)}}), ".cursor-plugin/plugin.json": cursorCycleJSON(map[string]any{"name": cursorCycleName, "version": r.Version, "mcpServers": "./mcp.json"}), "io.github.777genius.agentplugins/managed-stdio-v1/agentplugins": c.installerBody, "io.github.777genius.agentplugins/managed-stdio-v1/metadata.json": metadata}
}

func cursorCycleDirectories(files map[string][]byte) map[string]bool {
	result := map[string]bool{".": true}
	for name := range files {
		for dir := filepath.ToSlash(filepath.Dir(name)); dir != "."; dir = filepath.ToSlash(filepath.Dir(dir)) {
			result[dir] = true
		}
	}
	return result
}

func cursorCyclePermitted(info os.FileInfo, declared os.FileMode) bool {
	return info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky|os.ModeSymlink) == 0 && info.Mode().Perm()&^declared == 0 && info.Mode().Perm()&0400 != 0 && (info.Mode().Perm()&0111 != 0) == (declared&0111 != 0)
}

func (c *cursorMCPProjectCycle) acceptFile(path string, expected []byte, declared os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || !cursorCyclePermitted(info, declared) || info.Size() != int64(len(expected)) {
		return fmt.Errorf("Cursor expected ordinary file/mode/size differs")
	}
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != uint32(os.Getuid()) || s.Gid != uint32(os.Getgid()) || s.Nlink != 1 {
		return fmt.Errorf("Cursor expected file owner/link differs")
	}
	if held, exists := c.f.held[path]; exists && (!os.SameFile(info, held.info) || !projectNativeRuntimeStableStat(info.Sys(), held.info.Sys()) || held.digest != clineAttemptSHA(expected)) {
		return fmt.Errorf("Cursor original file authority cannot be renewed")
	}
	if err := c.f.hold(path); err != nil {
		return err
	}
	held := c.f.held[path]
	if held.digest != clineAttemptSHA(expected) || !os.SameFile(info, held.info) || !projectNativeRuntimeStableStat(info.Sys(), held.info.Sys()) {
		return fmt.Errorf("Cursor validated file changed before hold")
	}
	return nil
}

func (c *cursorMCPProjectCycle) acceptTree(t *testing.T, root string, files map[string][]byte, source bool) {
	t.Helper()
	directories, count := cursorCycleDirectories(files), 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || projectNativeRuntimeParents(path) != nil {
			return fmt.Errorf("Cursor expected tree parent differs")
		}
		name, err := filepath.Rel(root, path)
		name = filepath.ToSlash(name)
		if err != nil || count >= len(files)+len(directories) {
			return fmt.Errorf("Cursor expected tree finite membership differs")
		}
		count++
		if directories[name] {
			return c.acceptDirectory(path, name, source)
		}
		body, ok := files[name]
		if !ok || !entry.Type().IsRegular() {
			return fmt.Errorf("Cursor unknown/aliased package member")
		}
		mode := os.FileMode(0644)
		if source {
			mode = 0600
		}
		if name == "bin/probe" {
			mode = 0755
			if source {
				mode = 0700
			}
		}
		if strings.HasSuffix(name, "/managed-stdio-v1/agentplugins") {
			mode = 0700
		}
		if strings.HasSuffix(name, "/managed-stdio-v1/metadata.json") {
			mode = 0600
		}
		return c.acceptFile(path, body, mode)
	})
	if err != nil || count != len(files)+len(directories) {
		t.Fatal("Cursor complete expected tree differs", err)
	}
	c.trees[root] = files
}

func (c *cursorMCPProjectCycle) acceptDirectory(path, name string, source bool) error {
	info, err := os.Lstat(path)
	mode := os.FileMode(0755)
	if source || strings.HasPrefix(name, "io.github.777genius.agentplugins") {
		mode = 0700
	}
	if err != nil || !info.IsDir() || !cursorCyclePermitted(info, mode) {
		return fmt.Errorf("Cursor ordinary expected directory differs")
	}
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok || s.Uid != uint32(os.Getuid()) || s.Gid != uint32(os.Getgid()) {
		return fmt.Errorf("Cursor directory owner differs")
	}
	if before, exists := c.dirs[path]; exists && !cursorCycleDirectorySame(before, info) {
		return fmt.Errorf("Cursor original directory cannot be renewed")
	}
	c.dirs[path] = info
	return nil
}

func cursorCycleDirectorySame(a, b os.FileInfo) bool {
	x, okX := a.Sys().(*syscall.Stat_t)
	y, okY := b.Sys().(*syscall.Stat_t)
	return okX && okY && os.SameFile(a, b) && a.Mode() == b.Mode() && x.Uid == y.Uid && x.Gid == y.Gid
}

func (c *cursorMCPProjectCycle) payloadDigest(t *testing.T, files map[string][]byte) string {
	t.Helper()
	items := []packagedigest.CapturedEntry{}
	for name := range cursorCycleDirectories(files) {
		if name != "." {
			items = append(items, packagedigest.CapturedEntry{Path: name, Kind: "directory"})
		}
	}
	for name, body := range files {
		items = append(items, packagedigest.CapturedEntry{Path: name, Kind: "file", Executable: name == "bin/probe" || strings.HasSuffix(name, "/managed-stdio-v1/agentplugins"), Content: body})
	}
	digest, err := projectNativeRuntimeInstalledDigest(c.f.ctx, items)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
