//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func (c *cursorMCPProjectCycle) approvalRecipe(r *cursorCycleRecipe) (string, []byte, []string, error) {
	config := cursorCycleJSON(map[string]any{"mcpServers": map[string]any{"uapcursora": c.server(r)}})
	path, one, err := cursorApprovalRecipe(c.f.base.Home, r.Project, config)
	var current []string
	if err != nil || json.Unmarshal(one, &current) != nil || len(current) != 1 {
		return "", nil, nil, fmt.Errorf("Cursor expected approval fingerprint differs")
	}
	ids := append([]string{}, c.approvals[r.Scope]...)
	for _, id := range ids {
		if id == current[0] {
			return "", nil, nil, fmt.Errorf("Cursor approval must add one fresh expected revision")
		}
	}
	ids = append(ids, current[0])
	body, err := json.MarshalIndent(ids, "", "  ")
	return path, body, ids, err
}

func (c *cursorMCPProjectCycle) openApproval(t *testing.T, r *cursorCycleRecipe) {
	t.Helper()
	c.check(t)
	path, _, ids, err := c.approvalRecipe(r)
	wantCount := 1
	if r.Version == "1.1.0" {
		wantCount = 2
	}
	if err != nil || len(ids) != wantCount {
		t.Fatal("Cursor native approval phase/order differs", err)
	}
	if len(c.approvals[r.Scope]) == 0 {
		found := false
		for _, absent := range c.f.absent {
			found = found || absent == path
		}
		if _, err := os.Lstat(path); !found || !os.IsNotExist(err) {
			t.Fatal("Cursor initial approval absence differs")
		}
	} else {
		previous, _ := json.MarshalIndent(c.approvals[r.Scope], "", "  ")
		raw, err := projectNativeRuntimeRead(path, 4096)
		if err != nil || !bytes.Equal(raw, previous) || c.f.held[path].info == nil {
			t.Fatal("Cursor old approval bytes/authority differ", err)
		}
		c.priorApproval[r.Scope] = c.f.held[path]
		delete(c.f.held, path)
	}
}

func (c *cursorMCPProjectCycle) acceptApproval(t *testing.T, r *cursorCycleRecipe, stage string) {
	t.Helper()
	path, expected, ids, err := c.approvalRecipe(r)
	body, readErr := projectNativeRuntimeRead(path, 4096)
	info, statErr := os.Lstat(path)
	if err != nil || readErr != nil || statErr != nil || info.Mode() != 0644 || !bytes.Equal(body, expected) {
		t.Fatal("Cursor native approval exact bytes/mode differ")
	}
	if before := c.priorApproval[r.Scope]; before.info != nil && !cursorCycleDirectorySame(before.info, info) {
		t.Fatal("Cursor native approval replaced original file authority")
	}
	if err := c.acceptFile(path, expected, 0644); err != nil {
		t.Fatal(err)
	}
	c.removeAbsent(path)
	for parent := filepath.Dir(path); parent != c.f.base.Home; parent = filepath.Dir(parent) {
		c.acceptApprovalParent(t, parent)
	}
	c.approvals[r.Scope] = ids
	delete(c.priorApproval, r.Scope)
	c.proof(t, stage, map[string]any{"scope": r.Scope, "path": path, "body": c.stream(t, body), "fingerprints": ids, "probe_bytes": len(c.probe), "probe_sha256": clineAttemptSHA(c.probe)})
}

func (c *cursorMCPProjectCycle) acceptApprovalParent(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() {
		t.Fatal("Cursor approval parent is not ordinary")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) || stat.Gid != uint32(os.Getgid()) {
		t.Fatal("Cursor approval parent owner differs")
	}
	if old, exists := c.f.approvalParents[path]; exists {
		if !cursorCycleDirectorySame(old, info) {
			t.Fatal("Cursor existing approval parent changed")
		}
	} else {
		if info.Mode() != os.ModeDir|0755 {
			t.Fatal("Cursor new native approval parent mode differs")
		}
		c.f.approvalParents[path] = info
	}
}
