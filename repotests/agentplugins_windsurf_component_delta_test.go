//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

type windDeltaCount struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Changed int `json:"changed"`
}
type windDeltaValue struct {
	Kind          string `json:"kind"`
	Mode          uint32 `json:"mode,omitempty"`
	Bytes         *int64 `json:"bytes,omitempty"`
	ContentSHA256 string `json:"content_sha256,omitempty"`
	RecordSHA256  string `json:"record_sha256,omitempty"`
}
type windDeltaDetail struct {
	Root      string         `json:"root"`
	KeySHA256 string         `json:"key_sha256"`
	Operation string         `json:"operation"`
	Before    windDeltaValue `json:"before"`
	After     windDeltaValue `json:"after"`
}
type windProfileDelta struct {
	Status  string                    `json:"status"`
	Before  int                       `json:"before"`
	After   int                       `json:"after"`
	Total   windDeltaCount            `json:"total"`
	Roots   map[string]windDeltaCount `json:"roots"`
	Omitted int                       `json:"omitted_count"`
	Details []windDeltaDetail         `json:"details"`
}

func windDeltaKey(key string) (string, bool) {
	root, rel, ok := strings.Cut(key, "\x00")
	known := false
	for _, role := range windProfileRoles {
		known = known || root == role
	}
	return root, ok && known && rel != "" && !filepath.IsAbs(rel) && filepath.Clean(rel) == rel && rel != ".." && !strings.HasPrefix(rel, "../") && !strings.ContainsRune(rel, 0)
}
func windProjectRecord(record windProfileRecord, present bool) (windDeltaValue, bool) {
	if !present {
		return windDeltaValue{Kind: "absent"}, true
	}
	v := windDeltaValue{Kind: record.Kind, Mode: record.Mode}
	if record.Kind != "directory" && record.Kind != "file" {
		return windDeltaValue{}, false
	}
	if record.Kind == "file" {
		if record.Size < 0 || record.Size > 64<<20 || !windsurfConfigDigestValid(record.SHA256) {
			return windDeltaValue{}, false
		}
		size := record.Size
		v.Bytes = &size
		v.ContentSHA256 = record.SHA256
	} else if record.SHA256 != "" {
		return windDeltaValue{}, false
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return windDeltaValue{}, false
	}
	v.RecordSHA256 = windsurfConfigDigest(raw)
	return v, true
}
func windProfileDifference(before, after map[string]windProfileRecord) windProfileDelta {
	d := windProfileDelta{Status: "captured", Before: len(before), After: len(after), Roots: map[string]windDeltaCount{}, Details: []windDeltaDetail{}}
	for _, role := range windProfileRoles {
		d.Roots[role] = windDeltaCount{}
	}
	if before == nil || after == nil || len(before) > 4096 || len(after) > 4096 {
		d.Status = "inventory-bound"
		return d
	}
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	changed := []string{}
	for key := range keys {
		root, ok := windDeltaKey(key)
		a, hasA := before[key]
		b, hasB := after[key]
		_, validA := windProjectRecord(a, hasA)
		_, validB := windProjectRecord(b, hasB)
		if !ok || !validA || !validB {
			d.Status = "record-refused"
			d.Details = []windDeltaDetail{}
			return d
		}
		if hasA && hasB && a == b {
			continue
		}
		changed = append(changed, key)
		count := d.Roots[root]
		switch {
		case !hasA:
			count.Added++
			d.Total.Added++
		case !hasB:
			count.Removed++
			d.Total.Removed++
		default:
			count.Changed++
			d.Total.Changed++
		}
		d.Roots[root] = count
	}
	sort.Slice(changed, func(i, j int) bool {
		a, b := windDeltaRank(changed[i]), windDeltaRank(changed[j])
		if a != b {
			return a < b
		}
		return windsurfConfigDigest([]byte(changed[i])) < windsurfConfigDigest([]byte(changed[j]))
	})
	for _, key := range changed {
		if len(d.Details) == 3 {
			break
		}
		root, _ := windDeltaKey(key)
		a, hasA := before[key]
		b, hasB := after[key]
		va, _ := windProjectRecord(a, hasA)
		vb, _ := windProjectRecord(b, hasB)
		op := "changed"
		if !hasA {
			op = "added"
		}
		if !hasB {
			op = "removed"
		}
		d.Details = append(d.Details, windDeltaDetail{Root: root, KeySHA256: windsurfConfigDigest([]byte(key)), Operation: op, Before: va, After: vb})
	}
	d.Omitted = len(changed) - len(d.Details)
	if reflect.DeepEqual(before, after) {
		d.Status = "equal"
	}
	raw, err := json.Marshal(d)
	if err != nil || len(raw) > 2<<10 {
		d.Status = "details-bound"
		d.Details = []windDeltaDetail{}
		d.Omitted = len(changed)
	}
	return d
}
func windDeltaRank(key string) int {
	if key == "xdg-config\x00devin/config.json" {
		return 0
	}
	root, _ := windDeltaKey(key)
	for i, role := range []string{"xdg-config", "home", "xdg-data", "xdg-state", "xdg-cache", "tmp"} {
		if root == role {
			return i + 1
		}
	}
	return 7
}
