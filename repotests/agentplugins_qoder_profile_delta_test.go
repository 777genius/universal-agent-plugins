//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type qoderDeltaRecord struct {
	Kind          string `json:"kind"`
	Mode          string `json:"mode,omitempty"`
	Bytes         *int64 `json:"bytes,omitempty"`
	ContentSHA256 string `json:"content_sha256,omitempty"`
	RecordSHA256  string `json:"record_sha256,omitempty"`
}
type qoderDeltaEntry struct {
	KeySHA256 string           `json:"key_sha256"`
	Area      string           `json:"area"`
	Operation string           `json:"operation"`
	Before    qoderDeltaRecord `json:"before"`
	After     qoderDeltaRecord `json:"after"`
	rank      int
}
type qoderProfileDelta struct {
	Status        string            `json:"status"`
	BeforeEntries int               `json:"before_entries"`
	AfterEntries  int               `json:"after_entries"`
	Added         int               `json:"added"`
	Removed       int               `json:"removed"`
	Changed       int               `json:"changed"`
	Areas         map[string]int    `json:"areas"`
	OmittedCount  int               `json:"omitted_count"`
	Entries       []qoderDeltaEntry `json:"entries"`
}

func qoderOriginalProfileDelta(before, after map[string]string) qoderProfileDelta {
	delta := qoderProfileDelta{Status: "inventory-bound"}
	if len(before) > 4096 || len(after) > 4096 {
		return delta
	}
	delta.BeforeEntries, delta.AfterEntries = len(before), len(after)
	delta.Areas = map[string]int{"log-latest": 0, "logs": 0, "settings": 0, "plugin-cache": 0, "plugins": 0, "cache": 0, "other": 0}
	delta.Entries = []qoderDeltaEntry{}
	keys := make(map[string]bool, len(before)+len(after))
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	for key := range keys {
		old, had := before[key]
		current, has := after[key]
		if had && has && old == current {
			continue
		}
		entry, ok := qoderDeltaDetail(key, old, current, had, has)
		if !ok {
			return qoderProfileDelta{Status: "record-refused"}
		}
		switch entry.Operation {
		case "added":
			delta.Added++
		case "removed":
			delta.Removed++
		case "changed":
			delta.Changed++
		}
		delta.Areas[entry.Area]++
		delta.Entries = append(delta.Entries, entry)
	}
	return qoderDeltaBound(delta)
}

func qoderDeltaBound(delta qoderProfileDelta) qoderProfileDelta {
	sort.Slice(delta.Entries, func(i, j int) bool {
		a, b := delta.Entries[i], delta.Entries[j]
		return qoderDeltaPriorityLess(a, b, delta.Areas["other"] > 0)
	})
	delta.Status = "captured"
	if len(delta.Entries) > 3 {
		delta.OmittedCount, delta.Status = len(delta.Entries)-3, "entry-limit"
		delta.Entries = delta.Entries[:3]
	}
	if raw, err := json.Marshal(delta); err != nil || len(raw) > 2<<10 {
		return qoderProfileDelta{Status: "diagnostic-bound"}
	}
	return delta
}

func qoderDeltaDetail(key, old, current string, had, has bool) (qoderDeltaEntry, bool) {
	area, rank := qoderDeltaArea(key)
	entry := qoderDeltaEntry{KeySHA256: qoderDeltaHash(key), Area: area, rank: rank, Operation: "changed"}
	var beforeOK, afterOK bool
	entry.Before, beforeOK = qoderDeltaValue(old, had, area == "log-latest")
	entry.After, afterOK = qoderDeltaValue(current, has, area == "log-latest")
	if !had {
		entry.Operation = "added"
	}
	if !has {
		entry.Operation = "removed"
	}
	return entry, beforeOK && afterOK
}

func qoderDeltaArea(key string) (string, int) {
	switch {
	case key == "logs/latest":
		return "log-latest", 0
	case key == "logs" || strings.HasPrefix(key, "logs/"):
		return "logs", 1
	case key == "settings.json":
		return "settings", 2
	case key == "plugins/cache" || strings.HasPrefix(key, "plugins/cache/"):
		return "plugin-cache", 3
	case key == "plugins" || strings.HasPrefix(key, "plugins/"):
		return "plugins", 4
	case key == "cache" || strings.HasPrefix(key, "cache/"):
		return "cache", 5
	default:
		return "other", 6
	}
}

func qoderDeltaValue(record string, present, link bool) (qoderDeltaRecord, bool) {
	value := qoderDeltaRecord{Kind: "absent"}
	if !present {
		return value, true
	}
	if len(record) > 1024 {
		return value, false
	}
	parts := strings.Split(record, ":")
	if len(parts) < 2 {
		return value, false
	}
	if parts[0] == "symlink" {
		return qoderDeltaRecord{Kind: "symlink", RecordSHA256: qoderDeltaHash(record)}, link && qoderDeltaLinkValid(parts)
	}
	mode, err := strconv.ParseUint(parts[1], 8, 32)
	if err != nil || strconv.FormatUint(mode, 8) != parts[1] {
		return value, false
	}
	value.Kind, value.Mode = parts[0], parts[1]
	switch {
	case value.Kind == "directory" && len(parts) == 2:
		return value, os.FileMode(mode)&os.ModeType == os.ModeDir
	case value.Kind == "file" && len(parts) == 4:
		size, err := strconv.ParseInt(parts[2], 10, 64)
		value.Bytes, value.ContentSHA256 = &size, parts[3]
		return value, err == nil && size >= 0 && size <= 64<<20 && strconv.FormatInt(size, 10) == parts[2] && os.FileMode(mode).IsRegular() && qoderDeltaSHAValid(parts[3])
	default:
		return value, false
	}
}

func qoderDeltaLinkValid(parts []string) bool {
	if len(parts) != 26 || parts[12] != "target" || parts[3] != "120777" || parts[4] != "1" || !qoderDeltaSHAValid(parts[25]) {
		return false
	}
	for index := 1; index < 25; index++ {
		if index == 12 {
			continue
		}
		base := 10
		if index == 3 || index == 15 {
			base = 8
		}
		if index == 8 || index == 10 || index == 20 || index == 22 {
			if _, err := strconv.ParseInt(parts[index], base, 64); err != nil {
				return false
			}
		} else if _, err := strconv.ParseUint(parts[index], base, 64); err != nil {
			return false
		}
	}
	size, err := strconv.ParseUint(parts[24], 10, 64)
	return err == nil && size > 0 && size <= 4096 && parts[7] == parts[24]
}

func qoderDeltaHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func qoderDeltaSHAValid(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func qoderDeltaFixture(t *testing.T) string {
	t.Helper()
	root, _ := qoderLogFixture(t, "run-private-target")
	for _, path := range []string{"settings.json", "plugins/cache/private-entry", "cache/PRIVATE-KEY\n\x1b"} {
		projectNativeRuntimeWrite(t, filepath.Join(root, path), []byte("PRIVATE-BODY\n"), 0600)
	}
	return root
}

func qoderDeltaSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result, err := projectNativeCLIQoderProfile(root)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func qoderDeltaContract(t *testing.T, before, after map[string]string) qoderProfileDelta {
	t.Helper()
	old, _ := json.Marshal(before)
	current, _ := json.Marshal(after)
	delta := qoderOriginalProfileDelta(before, after)
	raw, err := json.Marshal(delta)
	if err != nil || len(raw) > 2<<10 || bytes.ContainsAny(raw, "\r\n") {
		t.Fatal("Qoder delta serialization bound")
	}
	for _, private := range []string{"PRIVATE-KEY", "PRIVATE-BODY", "run-private-target", "private-entry", "logs/latest", "settings.json"} {
		if bytes.Contains(raw, []byte(private)) {
			t.Fatal("Qoder delta exposed a private path, target or body")
		}
	}
	unchanged, _ := json.Marshal(before)
	stable, _ := json.Marshal(after)
	if !bytes.Equal(old, unchanged) || !bytes.Equal(current, stable) {
		t.Fatal("Qoder delta adopted or mutated either held inventory")
	}
	return delta
}

func TestQoderProfileDeltaMeasuresActualFileAndLinkChangesWithoutExemptingThem(t *testing.T) {
	for _, tc := range []struct {
		kind                    string
		added, removed, changed int
		areas                   map[string]int
	}{
		{"unchanged", 0, 0, 0, map[string]int{}}, {"bytes", 0, 0, 1, map[string]int{"logs": 1}}, {"mode", 0, 0, 1, map[string]int{"logs": 1}},
		{"new-log", 1, 0, 0, map[string]int{"logs": 1}}, {"empty-directory", 1, 0, 0, map[string]int{"logs": 1}},
		{"rotation", 1, 0, 1, map[string]int{"logs": 1, "log-latest": 1}}, {"same-target-rotation", 0, 0, 1, map[string]int{"log-latest": 1}},
		{"settings", 0, 0, 1, map[string]int{"settings": 1}}, {"plugin-cache", 0, 0, 1, map[string]int{"plugin-cache": 1}},
		{"private-name", 0, 0, 1, map[string]int{"cache": 1}}, {"removed-log", 0, 1, 1, map[string]int{"logs": 1, "log-latest": 1}},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			root := qoderDeltaFixture(t)
			before := qoderDeltaSnapshot(t, root)
			qoderDeltaMutate(t, root, tc.kind)
			after := qoderDeltaSnapshot(t, root)
			unchanged := qoderDeltaUnchanged(t, root)
			delta := qoderDeltaContract(t, before, after)
			if delta.Status != "captured" || delta.OmittedCount != 0 || delta.Added != tc.added || delta.Removed != tc.removed || delta.Changed != tc.changed || delta.BeforeEntries != len(before) || delta.AfterEntries != len(after) {
				t.Fatal("Qoder exact actual mutation counts differ")
			}
			for area, count := range delta.Areas {
				if count != tc.areas[area] {
					t.Fatal("Qoder exact static mutation area differs")
				}
			}
			qoderDeltaExactDetails(t, tc.kind, delta, before, after)
			unchanged()
			if !reflect.DeepEqual(after, qoderDeltaSnapshot(t, root)) || (tc.kind != "unchanged" && reflect.DeepEqual(before, after)) {
				t.Fatal("diagnostic changed a file or exempted the original unequal maps")
			}
		})
	}
}

func qoderDeltaExactDetails(t *testing.T, kind string, delta qoderProfileDelta, before, after map[string]string) {
	t.Helper()
	size, changedSize, newSize := int64(21), int64(25), int64(16)
	original := qoderDeltaRecord{Kind: "file", Mode: "600", Bytes: &size, ContentSHA256: "b049c3e0881bb9591f621e0e6faf7e5c3943177301a7d6c93ff8985b83e9bee5"}
	file := qoderDeltaEntry{KeySHA256: "cd593b10c09608bd0d708f4ef261332f353942a53cbae4b3dede51b71339c2ab", Area: "logs", Operation: "changed", Before: original, After: original}
	switch kind {
	case "bytes":
		file.After.Bytes, file.After.ContentSHA256 = &changedSize, "80635f06d1c96d49da1437c56a012c34aa38c378246fc206e10ececde076ecd5"
		qoderDeltaRequireDetail(t, delta, file)
	case "mode":
		file.After.Mode = "640"
		qoderDeltaRequireDetail(t, delta, file)
	case "new-log":
		qoderDeltaRequireDetail(t, delta, qoderDeltaEntry{KeySHA256: "22c92e7171f6f51a9ad9c4e8985ffeaccd63319a7d9d2f724560828ff9335de9", Area: "logs", Operation: "added", Before: qoderDeltaRecord{Kind: "absent"}, After: qoderDeltaRecord{Kind: "file", Mode: "600", Bytes: &newSize, ContentSHA256: "567f723eb74f37a466e683a7e7605a76fb818db9fa929e93c611751dd3ff45a3"}})
	case "removed-log":
		file.Operation, file.After = "removed", qoderDeltaRecord{Kind: "absent"}
		qoderDeltaRequireDetail(t, delta, file)
	}
	if kind == "rotation" || kind == "same-target-rotation" || kind == "removed-log" {
		old, current := sha256.Sum256([]byte(before["logs/latest"])), sha256.Sum256([]byte(after["logs/latest"]))
		qoderDeltaRequireDetail(t, delta, qoderDeltaEntry{KeySHA256: "22abff24b603cb9288fe82d7f92a98a22a4e3b648b8b0e07db5bc03da0901228", Area: "log-latest", Operation: "changed", Before: qoderDeltaRecord{Kind: "symlink", RecordSHA256: hex.EncodeToString(old[:])}, After: qoderDeltaRecord{Kind: "symlink", RecordSHA256: hex.EncodeToString(current[:])}})
	}
}

func qoderDeltaRequireDetail(t *testing.T, delta qoderProfileDelta, expected qoderDeltaEntry) {
	t.Helper()
	for _, entry := range delta.Entries {
		if entry.KeySHA256 == expected.KeySHA256 {
			entry.rank = 0
			if !reflect.DeepEqual(entry, expected) {
				t.Fatal("Qoder exact independent operation/kind/mode/size/digest detail differs")
			}
			return
		}
	}
	t.Fatal("Qoder exact independently expected detail missing")
}

func qoderDeltaMutate(t *testing.T, root, kind string) {
	t.Helper()
	var err error
	switch kind {
	case "unchanged":
		return
	case "settings", "plugin-cache", "private-name":
		path := map[string]string{"settings": "settings.json", "plugin-cache": "plugins/cache/private-entry", "private-name": "cache/PRIVATE-KEY\n\x1b"}[kind]
		err = os.WriteFile(filepath.Join(root, path), []byte("CHANGED-PRIVATE-BODY\n"), 0600)
	case "removed-log":
		err = os.Remove(filepath.Join(root, "logs/run-private-target/log"))
	default:
		qoderLogMutateFixture(t, root, filepath.Join(root, "logs/latest"), kind)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func qoderDeltaUnchanged(t *testing.T, root string) func() {
	t.Helper()
	held := map[string]os.FileInfo{}
	if err := filepath.WalkDir(root, func(path string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(path)
		held[path] = info
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		for path, info := range held {
			after, err := os.Lstat(path)
			if err != nil || !projectNativeRuntimeStableStat(info.Sys(), after.Sys()) {
				t.Fatal("Qoder diagnostic changed held profile metadata")
			}
		}
	}
}

func TestQoderProfileDeltaBoundsDetailsAndRefusesMalformedHeldRecords(t *testing.T) {
	before, after := map[string]string{}, map[string]string{}
	mode := strconv.FormatUint(uint64(os.ModeSetuid|os.ModeSetgid|os.ModeSticky|0777), 8)
	for _, name := range []string{"PRIVATE-KEY-one", "PRIVATE-KEY-two", "PRIVATE-KEY-three", "PRIVATE-KEY-four"} {
		before["cache/"+name] = "file:" + mode + ":67108864:" + strings.Repeat("a", 64)
		after["cache/"+name] = "file:" + mode + ":67108864:" + strings.Repeat("b", 64)
	}
	delta := qoderDeltaContract(t, before, after)
	if delta.Status != "entry-limit" || delta.Changed != 4 || delta.Areas["cache"] != 4 || delta.OmittedCount != 1 || len(delta.Entries) != 3 {
		t.Fatal("larger delta was silently truncated")
	}
	if again := qoderOriginalProfileDelta(before, after); !reflect.DeepEqual(delta, again) {
		t.Fatal("hashed bounded selection is not deterministic")
	}
	wantDigests := []string{"53dc54f1a425947b2f6b8175aee8b2cc6cd4abe4a5c5a57162bf0f834419e346", "62b14fde114d9eb0efda38990c4afb861568275b5ccb6d929f123f9e0ee2c140", "8018caba2e34bf352146720b5528dfa3b229b0899a268f4e4035b3107f24c9c5"}
	size := int64(64 << 20)
	for index, digest := range wantDigests {
		qoderDeltaRequireDetail(t, delta, qoderDeltaEntry{KeySHA256: digest, Area: "cache", Operation: "changed", Before: qoderDeltaRecord{Kind: "file", Mode: mode, Bytes: &size, ContentSHA256: strings.Repeat("a", 64)}, After: qoderDeltaRecord{Kind: "file", Mode: mode, Bytes: &size, ContentSHA256: strings.Repeat("b", 64)}})
		if delta.Entries[index].KeySHA256 != digest {
			t.Fatal("Qoder deterministic selected digest order differs")
		}
	}
	delta.Areas["cache"] = -1
	delta.Entries[0].KeySHA256 = "PRIVATE-KEY"
	if len(before) != 4 || len(after) != 4 || after["cache/PRIVATE-KEY-one"] != "file:"+mode+":67108864:"+strings.Repeat("b", 64) {
		t.Fatal("returned diagnostic aliases an input inventory")
	}
	for _, record := range []string{"PRIVATE-BODY", "file:600:1:PRIVATE-BODY", "directory:600", "file:600:-1:" + strings.Repeat("a", 64), "file:600:67108865:" + strings.Repeat("a", 64), "symlink:PRIVATE-BODY"} {
		if got := qoderDeltaContract(t, nil, map[string]string{"PRIVATE-KEY": record}); got.Status != "record-refused" || len(got.Entries) != 0 {
			t.Fatal("malformed private held record escaped refusal")
		}
	}
	oversized := map[string]string{}
	for index := range 4097 {
		oversized[strconv.Itoa(index)] = "directory:20000000700"
	}
	if got := qoderOriginalProfileDelta(oversized, nil); got.Status != "inventory-bound" {
		t.Fatal("oversized held inventory escaped original count cap")
	}
}
