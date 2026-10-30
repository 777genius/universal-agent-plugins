//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func (c *cursorMCPProjectCycle) check(t *testing.T) {
	t.Helper()
	if err := c.checkError(); err != nil {
		t.Fatal(err)
	}
}

func (c *cursorMCPProjectCycle) checkError() error {
	if c.f.ctx.Err() != nil {
		return fmt.Errorf("Cursor original cycle deadline exhausted")
	}
	if err := c.f.check(); err != nil {
		return err
	}
	for path, before := range c.dirs {
		after, err := os.Lstat(path)
		if err != nil || !cursorCycleDirectorySame(before, after) {
			return fmt.Errorf("Cursor held directory authority changed")
		}
	}
	for root, files := range c.trees {
		if err := c.treeSet(root, files); err != nil {
			return err
		}
	}
	return nil
}

func (c *cursorMCPProjectCycle) treeSet(root string, files map[string][]byte) error {
	dirs, count := cursorCycleDirectories(files), 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || projectNativeRuntimeParents(path) != nil {
			return fmt.Errorf("Cursor held tree inventory failed")
		}
		name, err := filepath.Rel(root, path)
		name = filepath.ToSlash(name)
		if err != nil || count >= len(files)+len(dirs) {
			return fmt.Errorf("Cursor held tree membership changed")
		}
		count++
		if dirs[name] {
			if !entry.IsDir() {
				return fmt.Errorf("Cursor held tree directory changed")
			}
			return nil
		}
		body, ok := files[name]
		if !ok || !entry.Type().IsRegular() || c.f.held[path].digest != clineAttemptSHA(body) {
			return fmt.Errorf("Cursor held tree file membership changed")
		}
		return nil
	})
	if err != nil || count != len(files)+len(dirs) {
		return fmt.Errorf("Cursor held tree membership changed")
	}
	return nil
}

func (c *cursorMCPProjectCycle) releasePackage(r *cursorCycleRecipe) {
	if r.Active == "" {
		return
	}
	delete(c.trees, r.Active)
	for path := range c.f.held {
		if strings.HasPrefix(path, r.Active+string(filepath.Separator)) {
			delete(c.f.held, path)
		}
	}
	for path := range c.dirs {
		if path == r.Active || strings.HasPrefix(path, r.Active+string(filepath.Separator)) {
			delete(c.dirs, path)
		}
	}
}

func (c *cursorMCPProjectCycle) openSelected(t *testing.T, r *cursorCycleRecipe) {
	t.Helper()
	c.check(t)
	c.forget(c.statePath(r))
	c.forget(r.Config)
	delete(c.trees, filepath.Dir(r.Config))
	c.releasePackage(r)
}

func cursorCycleDocument(raw []byte, expected any) error {
	value, err := projectNativeRuntimeJSON(raw)
	known, marshalErr := json.Marshal(expected)
	if err != nil || marshalErr != nil {
		return fmt.Errorf("Cursor closed document decode")
	}
	want, err := projectNativeRuntimeJSON(known)
	if err != nil || !reflect.DeepEqual(value, want) {
		return fmt.Errorf("Cursor closed document differs from independent recipe")
	}
	return nil
}

func (c *cursorMCPProjectCycle) profile(t *testing.T, r *cursorCycleRecipe) []byte {
	t.Helper()
	raw, err := projectNativeRuntimeRead(r.Config, 1<<20)
	if err != nil {
		t.Fatal("Cursor selected native config missing", err)
	}
	servers := map[string]any{}
	if !r.Removed {
		servers["uapcursora"] = c.server(r)
	}
	if err := cursorCycleDocument(raw, map[string]any{"mcpServers": servers}); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"mcp.json": raw, "foreign-owned.txt": []byte("caller-owned " + r.Scope + "\n")}
	c.acceptTree(t, filepath.Dir(r.Config), files, false)
	return raw
}

func (c *cursorMCPProjectCycle) dataProof(t *testing.T, r *cursorCycleRecipe, first bool) map[string]any {
	t.Helper()
	owner := filepath.Join(r.Data.Locator, ".agentplugins-data-owner.json")
	raw, err := projectNativeRuntimeRead(owner, 1<<20)
	if err != nil || cursorCycleDocument(raw, r.Data) != nil {
		t.Fatal("Cursor owned data marker differs", err)
	}
	marker := []byte("caller-data-" + r.Scope + "-" + c.f.base.Nonce + "\n")
	if first {
		projectNativeRuntimeWrite(t, filepath.Join(r.Data.Locator, "caller-owned.txt"), marker, 0600)
	}
	c.acceptTree(t, r.Data.Locator, map[string][]byte{".agentplugins-data-owner.json": raw, "caller-owned.txt": marker}, true)
	return map[string]any{"receipt": r.Data, "owner": c.stream(t, raw), "caller": c.stream(t, marker)}
}

func (c *cursorMCPProjectCycle) missingA(t *testing.T) {
	t.Helper()
	c.check(t)
	r := c.recipes["a"]
	raw, err := projectNativeRuntimeRead(r.Config, 1<<20)
	if err != nil || cursorCycleDocument(raw, map[string]any{"mcpServers": map[string]any{"uapcursora": c.server(r)}}) != nil {
		t.Fatal("Cursor sole owned native-config absence fault refused", err)
	}
	before, err := os.Lstat(r.Config)
	if held := c.f.held[r.Config]; err != nil || !os.SameFile(before, held.info) || !projectNativeRuntimeStableStat(before.Sys(), held.info.Sys()) {
		t.Fatal("Cursor fault file authority changed")
	}
	if err := os.Remove(r.Config); err != nil {
		t.Fatal(err)
	}
	c.forget(r.Config)
	c.f.absent = append(c.f.absent, r.Config)
	c.trees[filepath.Dir(r.Config)] = map[string][]byte{"foreign-owned.txt": []byte("caller-owned a\n")}
	c.proof(t, "missing-native-a", map[string]any{"scope": "a", "path": r.Config, "before": c.stream(t, raw), "managed_package_unchanged": true})
	c.check(t)
}

func (c *cursorMCPProjectCycle) proofPayload(t *testing.T, r *cursorCycleRecipe) map[string]any {
	t.Helper()
	files := c.projected(r)
	c.acceptTree(t, r.Active, files, false)
	result := map[string]any{}
	for name, body := range files {
		result[name] = map[string]any{"bytes": len(body), "sha256": clineAttemptSHA(body), "executable": name == "bin/probe" || strings.HasSuffix(name, "/managed-stdio-v1/agentplugins")}
	}
	return map[string]any{"root": r.Active, "files": result, "digest": c.payloadDigest(t, files)}
}
