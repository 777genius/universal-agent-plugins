//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// auditNativeCache records evidence for an otherwise refused native cache
// mutation. It never updates a frozen frame or grants a runtime exception.
func (c *windProjectCycle) auditNativeCache(label string, start, end time.Time) error {
	root := filepath.Join(c.w.f.base.Root, "xdg-cache", "devin")
	held, ok := c.w.trees[root]
	if !ok || end.Before(start) {
		return fmt.Errorf("Windsurf native cache audit missing held authority")
	}
	observed, err := windOwnedTree(c.w.f.ctx, root)
	if err != nil {
		return fmt.Errorf("Windsurf native cache audit inventory refused")
	}
	all := map[string]bool{}
	for name := range held {
		all[name] = true
	}
	for name := range observed {
		all[name] = true
	}
	changed := []string{}
	for name := range all {
		old, before := held[name]
		now, after := observed[name]
		if before != after || old != now {
			changed = append(changed, name)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	sort.Strings(changed)
	audit := map[string]any{
		"diagnostic_only":      true,
		"accepted":             false,
		"stage":                label,
		"root_sha256":          windsurfConfigDigest([]byte(root)),
		"native_binary_sha256": c.w.f.immutable[c.w.f.client],
		"started_at":           start.UTC().Format(time.RFC3339Nano),
		"ended_at":             end.UTC().Format(time.RFC3339Nano),
		"changed_records":      len(changed),
	}
	// Never disclose arbitrary file names, even inside disposable native cache.
	// A large/unreadable delta remains a refusal, not an output-amplification path.
	if len(changed) > 16 {
		audit["bounded_out"] = true
		c.w.f.evidence["native_cache_delta"] = audit
		return fmt.Errorf("Windsurf native cache audit exceeded record bound")
	}
	rows := make([]map[string]any, 0, len(changed))
	for _, name := range changed {
		old, before := held[name]
		now, after := observed[name]
		operation := "changed"
		if !before {
			operation = "added"
		} else if !after {
			operation = "missing"
		}
		rows = append(rows, map[string]any{
			"key_sha256":     windsurfConfigDigest([]byte(name)),
			"parent_sha256":  windsurfConfigDigest([]byte(filepath.ToSlash(filepath.Dir(name)))),
			"operation":      operation,
			"before":         before,
			"after":          after,
			"previous":       old,
			"current":        now,
			"within_command": after && windRuntimeCommandTimes(now, start, end),
		})
	}
	audit["records"] = rows
	c.w.f.evidence["native_cache_delta"] = audit
	return nil
}

func TestWindsurfCycleCacheAuditRemainsFailClosedAndOpaque(t *testing.T) {
	c := windCycleInert(t)
	root := filepath.Join(c.w.f.base.Root, "xdg-cache", "devin")
	name := "private\nDO_NOT_DISCLOSE"
	path := filepath.Join(root, "cli", name)
	start := time.Now()
	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(path, []byte("PRIVATE_CONTENT"), 0600); err != nil {
		t.Fatal(err)
	}
	end := time.Now()
	if err := c.auditNativeCache("b-mcp", start, end); err != nil {
		t.Fatal(err)
	}
	if windFrozenTrees(c.w.f.ctx, c.w.trees) == nil {
		t.Fatal("audit authorized unknown foreign cache file")
	}
	raw, err := json.Marshal(c.w.f.evidence["native_cache_delta"])
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{name, "PRIVATE", "DO_NOT_DISCLOSE", path, root} {
		if strings.Contains(string(raw), leak) {
			t.Fatal("cache audit disclosed private input")
		}
	}
	var audit map[string]any
	if err := json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	rows, ok := audit["records"].([]any)
	if !ok || len(rows) < 2 || audit["diagnostic_only"] != true || audit["accepted"] != false {
		t.Fatal("cache audit incomplete")
	}
	found := false
	for _, row := range rows {
		record := row.(map[string]any)
		if record["key_sha256"] == windsurfConfigDigest([]byte("cli/"+name)) {
			if record["operation"] != "added" || record["within_command"] != true {
				t.Fatal("cache file origin/time not witnessed")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("cache audit omitted exact hashed file")
	}
}

func TestWindsurfCycleCacheAuditRefusesUnknownSymlink(t *testing.T) {
	c := windCycleInert(t)
	root := filepath.Join(c.w.f.base.Root, "xdg-cache", "devin")
	if err := os.Symlink("telemetry_state.json", filepath.Join(root, "cli", "foreign-link")); err != nil {
		t.Fatal(err)
	}
	if c.auditNativeCache("b-mcp", time.Now().Add(-time.Second), time.Now()) == nil {
		t.Fatal("cache symlink became auditable authority")
	}
	if windFrozenTrees(c.w.f.ctx, c.w.trees) == nil {
		t.Fatal("cache symlink escaped closed tree")
	}
}
