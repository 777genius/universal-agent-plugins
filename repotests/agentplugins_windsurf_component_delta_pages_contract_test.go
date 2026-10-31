//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func windPageContractMaps(count int) (map[string]windProfileRecord, map[string]windProfileRecord, map[string]bool) {
	before, after, changed := map[string]windProfileRecord{}, map[string]windProfileRecord{}, map[string]bool{}
	for _, root := range []string{"home", "tmp", "xdg-config", "xdg-cache", "xdg-data", "xdg-state"} {
		value := windProfileRecord{Kind: "directory", Mode: unix.S_IFDIR | 0700}
		before[root+"\x00."], after[root+"\x00."] = value, value
	}
	for i := range count {
		root := []string{"xdg-config", "home", "xdg-data", "xdg-state", "xdg-cache", "tmp"}[i%6]
		key := root + "\x00" + fmt.Sprintf("PRIVATE_PATH_%02d", i)
		if i == 0 {
			key = "xdg-config\x00devin/config.json"
		}
		file := windProfileRecord{Kind: "file", Mode: unix.S_IFREG | 0600, Size: 1, SHA256: windsurfConfigDigest([]byte("a"))}
		switch i % 3 {
		case 0:
			file = windProfileRecord{Kind: "directory", Mode: unix.S_IFDIR | 0700}
			before[key] = file
			file.Inode = 123
			after[key] = file
		case 1:
			after[key] = file
		case 2:
			before[key] = file
		}
		changed[windsurfConfigDigest([]byte(key))] = true
	}
	return before, after, changed
}

func TestWindsurfFixedDeltaPagesCoverOnlyEighteenAndPreserveOriginal(t *testing.T) {
	for _, count := range []int{0, 1, 3, 4, 14, 15, 16, 18, 19} {
		before, after, expected := windPageContractMaps(count)
		old := windProfileDifference(before, after)
		pages := windProfileDifferencePages(before, after)
		if !reflect.DeepEqual(old, pages[0]) {
			t.Fatal("original zero-offset projection changed")
		}
		seen, ranks := map[string]bool{}, []string{}
		for index, page := range pages {
			raw, err := json.Marshal(page)
			if err != nil || len(raw) > 2<<10 || strings.Contains(string(raw), "PRIVATE") || page.Before != old.Before || page.After != old.After || page.Total != old.Total || !reflect.DeepEqual(page.Roots, old.Roots) || page.Omitted != count-len(page.Details) {
				t.Fatal("fixed page lost original counts/bounds/privacy", index, count, err)
			}
			want := min(3, max(0, count-index*3))
			if len(page.Details) != want {
				t.Fatal("fixed page window differs", index, count)
			}
			for _, detail := range page.Details {
				if seen[detail.KeySHA256] || !expected[detail.KeySHA256] {
					t.Fatal("duplicate or unobserved delta key")
				}
				seen[detail.KeySHA256] = true
				rank := strings.Index("xdg-config home xdg-data xdg-state xdg-cache tmp", detail.Root) + 1
				if detail.KeySHA256 == windsurfConfigDigest([]byte("xdg-config\x00devin/config.json")) {
					rank = 0
				}
				ranks = append(ranks, fmt.Sprintf("%03d:%s", rank, detail.KeySHA256))
			}
		}
		if len(seen) != min(count, 18) || count-len(seen) != max(0, count-18) || !sort.StringsAreSorted(ranks) {
			t.Fatal("fixed full coverage/overflow/global ordering differs", count)
		}
		backwards := map[string]windProfileRecord{}
		keys := make([]string, 0, len(after))
		for key := range after {
			keys = append(keys, key)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(keys)))
		for _, key := range keys {
			backwards[key] = after[key]
		}
		if !reflect.DeepEqual(pages, windProfileDifferencePages(before, backwards)) {
			t.Fatal("map insertion order changed fixed pages")
		}
	}
}

func TestWindsurfFixedDeltaPagesNeverAliasInputOrHideRefusedRecords(t *testing.T) {
	before, after, _ := windPageContractMaps(15)
	beforeRaw, _ := json.Marshal(before)
	afterRaw, _ := json.Marshal(after)
	pages := windProfileDifferencePages(before, after)
	pages[1].Roots["home"] = windDeltaCount{Added: 123}
	pages[1].Details[0].Before.Kind = "PRIVATE"
	checkBefore, _ := json.Marshal(before)
	checkAfter, _ := json.Marshal(after)
	if string(beforeRaw) != string(checkBefore) || string(afterRaw) != string(checkAfter) || reflect.DeepEqual(pages[0].Roots, pages[1].Roots) {
		t.Fatal("paged output renewed or aliased original authority")
	}
	for _, invalid := range []map[string]windProfileRecord{nil, {"private\x00PRIVATE": {Kind: "symlink"}}, {"home\x00PRIVATE": {Kind: "file", Size: (64 << 20) + 1}}} {
		for _, page := range windProfileDifferencePages(before, invalid) {
			raw, err := json.Marshal(page)
			if (page.Status != "inventory-bound" && page.Status != "record-refused") || len(page.Details) != 0 || err != nil || len(raw) > 2<<10 || strings.Contains(string(raw), "PRIVATE") {
				t.Fatal("refused inventory became accepted or exposed private input")
			}
		}
	}
}

func TestWindsurfFixedDeltaPagesKeepExistingSerializationFallback(t *testing.T) {
	before, after, _ := windPageContractMaps(0)
	for i := range 3 {
		key := fmt.Sprintf("home\x00PRIVATE_PATH_%d", i)
		file := windProfileRecord{Kind: "file", Mode: unix.S_IFREG | 0600, Size: 64 << 20, SHA256: windsurfConfigDigest([]byte("a"))}
		before[key] = file
		file.SHA256 = windsurfConfigDigest([]byte("b"))
		after[key] = file
	}
	pages := windProfileDifferencePages(before, after)
	if pages[0].Status != "details-bound" || !reflect.DeepEqual(pages[0], windProfileDifference(before, after)) {
		t.Fatal("original serialization fallback changed")
	}
	for _, page := range pages {
		raw, err := json.Marshal(page)
		if err != nil || len(raw) > 2<<10 || len(page.Details) != 0 || page.Omitted != 3 || page.Total != (windDeltaCount{Changed: 3}) || strings.Contains(string(raw), "PRIVATE") {
			t.Fatal("serialization fallback hid counts or exceeded existing bound")
		}
	}
}
