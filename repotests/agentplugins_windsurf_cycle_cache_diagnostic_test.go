//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
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
	// Arbitrary names remain opaque. Only a bounded new cli leaf can carry
	// digest/size and aggregate-type facts; none of these facts authorize its mutation.
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
		row := map[string]any{
			"key_sha256":     windsurfConfigDigest([]byte(name)),
			"parent_sha256":  windsurfConfigDigest([]byte(filepath.ToSlash(filepath.Dir(name)))),
			"operation":      operation,
			"before":         before,
			"after":          after,
			"previous":       old,
			"current":        now,
			"within_command": after && windRuntimeCommandTimes(now, start, end),
		}
		if !before && after {
			if fact := windCacheFileObservation(root, name, now); fact != nil {
				row["file_observation"] = fact
			}
		}
		rows = append(rows, row)
	}
	audit["records"] = rows
	c.w.f.evidence["native_cache_delta"] = audit
	return nil
}

var windCacheLeafName = regexp.MustCompile(`^cli/[A-Za-z0-9][A-Za-z0-9._-]{0,95}$`)

// This optional diagnostic reads a held, bounded file but exports no values.
// A failed read leaves the original opaque refusal evidence unchanged.
func windCacheFileObservation(root, name string, want windProfileRecord) (fact map[string]any) {
	if !windCacheLeafName.MatchString(name) || want.Kind != "file" || want.Size < 0 || want.Size > 1<<20 {
		return nil
	}
	path := filepath.Join(root, filepath.FromSlash(name))
	edges, err := windOpenDirectory(filepath.Dir(path))
	if err != nil {
		return nil
	}
	defer func() {
		if windCloseEdges(edges) != nil {
			fact = nil
		}
	}()
	var before, after unix.Stat_t
	if unix.Lstat(path, &before) != nil || windRecord(before, "file", want.SHA256) != want {
		return nil
	}
	body, err := projectNativeRuntimeRead(path, 1<<20)
	if err != nil || int64(len(body)) != want.Size || windsurfConfigDigest(body) != want.SHA256 {
		return nil
	}
	if unix.Lstat(path, &after) != nil || !windSameStat(before, after) || windCheckEdges(edges) != nil {
		return nil
	}
	fact = map[string]any{"bytes": want.Size, "sha256": want.SHA256, "format": "opaque"}
	value, err := projectNativeRuntimeJSON(body)
	if err != nil {
		return fact
	}
	kind := windCacheJSONType(value)
	fact["format"] = "json-scalar"
	if kind == "array" || kind == "object" {
		fact["format"] = "json-" + kind
	}
	if object, ok := value.(map[string]any); ok {
		if counts := windCacheJSONTypes(object); counts != nil {
			fact["top_level_type_counts"] = counts
		}
	}
	return fact
}

func windCacheJSONTypes(object map[string]any) map[string]int {
	if len(object) > 32 {
		return nil
	}
	counts := make(map[string]int)
	for _, value := range object {
		counts[windCacheJSONType(value)]++
	}
	return counts
}

func windCacheJSONType(value any) string {
	switch value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case json.Number:
		return "number"
	case bool:
		return "boolean"
	default:
		return "null"
	}
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

func TestWindsurfCycleCacheFileObservationIsBoundedSchemaOnly(t *testing.T) {
	for _, scenario := range []struct {
		name, body, format string
		counts             map[string]int
	}{
		{"cache.json", `{"access_token":"OWNED_CACHE_VALUE_MUST_NOT_APPEAR","servers":{"nested":"OWNED_CACHE_VALUE_MUST_NOT_APPEAR"},"items":[],"count":1,"enabled":true,"empty":null}`, "json-object", map[string]int{"string": 1, "object": 1, "array": 1, "number": 1, "boolean": 1, "null": 1}},
		{"duplicate.json", `{"key":1,"key":2}`, "opaque", nil},
		{"malformed.json", `{"token":"OWNED_CACHE_VALUE_MUST_NOT_APPEAR"`, "opaque", nil},
		{"unsafe-schema.json", `{"/private/path":"OWNED_CACHE_VALUE_MUST_NOT_APPEAR"}`, "json-object", map[string]int{"string": 1}},
		{"SECRET_FILENAME_DO_NOT_DISCLOSE.json", `{"SECRET_KEY_DO_NOT_DISCLOSE":"OWNED_CACHE_VALUE_MUST_NOT_APPEAR"}`, "json-object", map[string]int{"string": 1}},
		{"array.json", `["OWNED_CACHE_VALUE_MUST_NOT_APPEAR"]`, "json-array", nil},
		{"scalar.json", `"OWNED_CACHE_VALUE_MUST_NOT_APPEAR"`, "json-scalar", nil},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			c := windCycleInert(t)
			root := filepath.Join(c.w.f.base.Root, "xdg-cache", "devin")
			name := "cli/" + scenario.name
			if err := os.WriteFile(filepath.Join(root, name), []byte(scenario.body), 0600); err != nil {
				t.Fatal(err)
			}
			if err := c.auditNativeCache("a-mcp", time.Now().Add(-time.Second), time.Now()); err != nil {
				t.Fatal(err)
			}
			if windFrozenTrees(c.w.f.ctx, c.w.trees) == nil {
				t.Fatal("schema diagnostic authorized new cache")
			}
			audit := c.w.f.evidence["native_cache_delta"].(map[string]any)
			var fact map[string]any
			for _, row := range audit["records"].([]map[string]any) {
				if row["key_sha256"] == windsurfConfigDigest([]byte(name)) {
					fact, _ = row["file_observation"].(map[string]any)
				}
			}
			if fact == nil || fact["bytes"] != int64(len(scenario.body)) || fact["sha256"] != windsurfConfigDigest([]byte(scenario.body)) || fact["format"] != scenario.format {
				t.Fatal("cache diagnostic did not bind exact bounded file facts")
			}
			counts, _ := fact["top_level_type_counts"].(map[string]int)
			if !reflect.DeepEqual(counts, scenario.counts) {
				t.Fatal("cache diagnostic aggregate types differ")
			}
			raw, err := json.Marshal(audit)
			if err != nil || strings.Contains(string(raw), "OWNED_CACHE_VALUE_MUST_NOT_APPEAR") || strings.Contains(string(raw), "SECRET_FILENAME_DO_NOT_DISCLOSE") || strings.Contains(string(raw), "SECRET_KEY_DO_NOT_DISCLOSE") || strings.Contains(string(raw), "/private/path") || strings.Contains(string(raw), root) {
				t.Fatal("cache diagnostic disclosed values or host path")
			}
		})
	}
}

func TestWindsurfCycleCacheFileObservationRefusesUnboundFacts(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "cli"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "cli", "cache.json")
	body := []byte(`{"count":1}`)
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		t.Fatal(err)
	}
	want := windRecord(stat, "file", windsurfConfigDigest(body))
	if windCacheFileObservation(root, "cli/cache.json", want) == nil {
		t.Fatal("exact owned file not observed")
	}
	for _, name := range []string{"cli/../cache.json", "cli/nested/cache.json", "cli/private\nname", "/cli/cache.json", "cli/" + strings.Repeat("x", 97)} {
		if windCacheFileObservation(root, name, want) != nil {
			t.Fatal("unsafe name disclosed")
		}
	}
	wrong := want
	wrong.SHA256 = strings.Repeat("0", 64)
	if windCacheFileObservation(root, "cli/cache.json", wrong) != nil {
		t.Fatal("digest mismatch accepted")
	}
	wrong = want
	wrong.Inode++
	if windCacheFileObservation(root, "cli/cache.json", wrong) != nil {
		t.Fatal("identity mismatch accepted")
	}
	wrong = want
	wrong.Size = (1 << 20) + 1
	if windCacheFileObservation(root, "cli/cache.json", wrong) != nil {
		t.Fatal("oversize file observation accepted")
	}
	large := make(map[string]any, 33)
	for i := range 33 {
		large[fmt.Sprintf("key%d", i)] = "PRIVATE_VALUE"
	}
	if windCacheJSONTypes(large) != nil {
		t.Fatal("unbounded schema accepted")
	}
}
