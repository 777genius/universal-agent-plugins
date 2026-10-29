//go:build linux

package pluginkitairepo_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func qoderFourDetailFixture(t *testing.T) (string, map[string]string) {
	t.Helper()
	root := qoderDeltaFixture(t)
	before := qoderDeltaSnapshot(t, root)
	directory := filepath.Join(root, "logs", "PRIVATE-KEY-new-run")
	if err := os.Mkdir(directory, 0755); err != nil {
		t.Fatal(err)
	}
	projectNativeRuntimeWrite(t, filepath.Join(directory, "qodercli.log"), []byte("PRIVATE-BODY-log\n"), 0644)
	projectNativeRuntimeWrite(t, filepath.Join(directory, "manifest.json"), []byte("{\"PRIVATE-KEY\":\"PRIVATE-BODY\"}\n"), 0644)
	link := filepath.Join(root, "logs", "latest")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("PRIVATE-KEY-new-run", link); err != nil {
		t.Fatal(err)
	}
	return root, before
}

func qoderFourDetailExpected(before, after map[string]string) []qoderDeltaEntry {
	manifestSize, logSize := int64(31), int64(17)
	return []qoderDeltaEntry{
		{KeySHA256: "22abff24b603cb9288fe82d7f92a98a22a4e3b648b8b0e07db5bc03da0901228", Area: "log-latest", Operation: "changed",
			Before: qoderDeltaRecord{Kind: "symlink", RecordSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(before["logs/latest"])))},
			After:  qoderDeltaRecord{Kind: "symlink", RecordSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(after["logs/latest"])))}},
		{KeySHA256: "1b370a214d3c23056984b0a326e20ea1bb58f6cc8403d87cf06e41eb31db1cc8", Area: "logs", Operation: "added", Before: qoderDeltaRecord{Kind: "absent"},
			After: qoderDeltaRecord{Kind: "file", Mode: "644", Bytes: &manifestSize, ContentSHA256: "3ffa6538f18eb86fa0e9250ca108864f5393c1816851e9a3d758bf387104a751"}},
		{KeySHA256: "5e8c44b0532d3994adc6f7e8a3c67a68c9d8397812a0ab64583b728c58da4277", Area: "logs", Operation: "added", Before: qoderDeltaRecord{Kind: "absent"},
			After: qoderDeltaRecord{Kind: "directory", Mode: "20000000755"}},
		{KeySHA256: "ee5ac080ef6087d938a7938b7c86dd310a8313384bee0e41a46cabb3f9fc0c3b", Area: "logs", Operation: "added", Before: qoderDeltaRecord{Kind: "absent"},
			After: qoderDeltaRecord{Kind: "file", Mode: "644", Bytes: &logSize, ContentSHA256: "e2b9c8e8c303354e03149e46c6dafcd17303e3998b2026fb7aee9c6960136a4f"}},
	}
}

func TestQoderProfileDeltaRetainsFourActualDetailsAndTruthfulFifthOmission(t *testing.T) {
	for _, extra := range []int{0, 1} {
		t.Run(fmt.Sprint(extra), func(t *testing.T) {
			root, before := qoderFourDetailFixture(t)
			if extra == 1 {
				qoderDeltaMutate(t, root, "settings")
			}
			after := qoderDeltaSnapshot(t, root)
			unchanged := qoderDeltaUnchanged(t, root)
			delta := qoderDeltaContract(t, before, after)
			status := "captured"
			if extra == 1 {
				status = "entry-limit"
			}
			areas := map[string]int{"log-latest": 1, "logs": 3, "settings": extra, "plugin-cache": 0, "plugins": 0, "cache": 0, "other": 0}
			if delta.Status != status || delta.BeforeEntries != len(before) || delta.AfterEntries != len(before)+3 || delta.Added != 3 || delta.Removed != 0 || delta.Changed != 1+extra || delta.OmittedCount != extra || len(delta.Entries) != 4 || !reflect.DeepEqual(delta.Areas, areas) {
				t.Fatal("four-detail diagnostic lost exact counts, bounds or omission")
			}
			for index, want := range qoderFourDetailExpected(before, after) {
				if delta.Entries[index].KeySHA256 != want.KeySHA256 {
					t.Fatal("four-detail diagnostic changed the independently expected prefix order")
				}
				qoderDeltaRequireDetail(t, delta, want)
			}
			qoderFilePriorityRequireUnchanged(t, root, before, after, delta, unchanged)
			if reflect.DeepEqual(before, after) {
				t.Fatal("four-detail diagnostic adopted the unequal original profile")
			}
		})
	}
}
