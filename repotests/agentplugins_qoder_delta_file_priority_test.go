//go:build linux

package pluginkitairepo_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var qoderFilePriorityExpected = []struct{ key, digest string }{
	{"1c789d8c54f84d9a76bb9e9c466fd82edc93600e0ae1096beb1aa2caae60c045", "0bf9b4479f31c38fe151c1ee8166988124d602088321bb0568bad105c279342d"},
	{"5b16aaa9c4be8b1466c2722c7fa908a2b7e41e6ea4c487f9a19cd536f1a655e4", "83e787826d4b24b98e62a63c91474209e44436757410c905e885fc36c1aad374"},
	{"9c39a648892eec9283695b692d95e346f397274fb8d9e284c9a7eaf005d2fea0", "b012dc678157d20bc91a92eb1f4f2087a9d819be3d287674757833b62fd1d7fd"},
}

func qoderFilePriorityAddDirectories(t *testing.T, root string) {
	t.Helper()
	for index := range 2 {
		if err := os.Mkdir(filepath.Join(root, fmt.Sprintf("PRIVATE-KEY-dir-%d", index)), 0700); err != nil {
			t.Fatal(err)
		}
	}
}

func qoderFilePriorityRequireUnchanged(t *testing.T, root string, before, after map[string]string, delta qoderProfileDelta, unchanged func()) {
	t.Helper()
	again := qoderDeltaContract(t, qoderDeltaReverseInsertion(before), qoderDeltaReverseInsertion(after))
	if !reflect.DeepEqual(delta, again) {
		t.Fatal("file priority depends on map insertion order")
	}
	if again.Entries[0].After.Bytes != nil {
		*again.Entries[0].After.Bytes = 999
	}
	if again.Entries[0].Before.Bytes != nil {
		*again.Entries[0].Before.Bytes = 999
	}
	again.Entries[0].After.Kind = "PRIVATE-RETURNED-KIND"
	again.Areas["other"] = 999
	if !reflect.DeepEqual(delta, qoderDeltaContract(t, before, after)) || !reflect.DeepEqual(after, qoderDeltaSnapshot(t, root)) {
		t.Fatal("file priority aliases a held map or changes the original unequal profile")
	}
	unchanged()
}

func TestQoderProfileDeltaPrioritizesHeldOtherFilesOverDirectoryOnlyChanges(t *testing.T) {
	root := qoderDeltaFixture(t)
	before := qoderDeltaSnapshot(t, root)
	qoderFilePriorityAddDirectories(t, root)
	for index := range 5 {
		projectNativeRuntimeWrite(t, filepath.Join(root, fmt.Sprintf("PRIVATE-KEY-file-%d", index)), []byte(fmt.Sprintf("PRIVATE-BODY-%d\n", index)), 0640)
	}
	for index := range 3 {
		projectNativeRuntimeWrite(t, filepath.Join(root, "logs", "run-private-target", fmt.Sprintf("PRIVATE-KEY-log-%d", index)), []byte("PRIVATE-BODY-log\n"), 0600)
	}
	qoderDeltaMutate(t, root, "same-target-rotation")
	after := qoderDeltaSnapshot(t, root)
	unchanged := qoderDeltaUnchanged(t, root)
	delta := qoderDeltaContract(t, before, after)
	areas := map[string]int{"log-latest": 1, "logs": 3, "settings": 0, "plugin-cache": 0, "plugins": 0, "cache": 0, "other": 7}
	if delta.Status != "entry-limit" || delta.BeforeEntries != len(before) || delta.AfterEntries != len(before)+10 || delta.Added != 10 || delta.Removed != 0 || delta.Changed != 1 || delta.OmittedCount != 8 || len(delta.Entries) != 3 || !reflect.DeepEqual(delta.Areas, areas) {
		t.Fatal("file priority changed complete counts, areas or three-detail/eight-omission bounds")
	}
	size := int64(15)
	for index, want := range qoderFilePriorityExpected {
		if delta.Entries[index].KeySHA256 != want.key {
			t.Fatal("directory-only changes displaced the independently expected file hash")
		}
		qoderDeltaRequireDetail(t, delta, qoderDeltaEntry{KeySHA256: want.key, Area: "other", Operation: "added", Before: qoderDeltaRecord{Kind: "absent"}, After: qoderDeltaRecord{Kind: "file", Mode: "640", Bytes: &size, ContentSHA256: want.digest}})
	}
	qoderFilePriorityRequireUnchanged(t, root, before, after, delta, unchanged)
}

func TestQoderProfileDeltaFilePriorityIncludesRemovedAndTypeChangedFiles(t *testing.T) {
	for _, mode := range []string{"removed-file", "file-to-directory", "directory-to-file"} {
		t.Run(mode, func(t *testing.T) {
			root := qoderDeltaFixture(t)
			path := filepath.Join(root, "PRIVATE-KEY-file-target")
			if mode == "directory-to-file" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				projectNativeRuntimeWrite(t, path, []byte("PRIVATE-BODY\n"), 0640)
			}
			before := qoderDeltaSnapshot(t, root)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "file-to-directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "directory-to-file":
				projectNativeRuntimeWrite(t, path, []byte("PRIVATE-BODY\n"), 0640)
			}
			qoderFilePriorityAddDirectories(t, root)
			after := qoderDeltaSnapshot(t, root)
			unchanged := qoderDeltaUnchanged(t, root)
			delta := qoderDeltaContract(t, before, after)
			removed, changed := 0, 1
			if mode == "removed-file" {
				removed, changed = 1, 0
			}
			if delta.Status != "captured" || delta.Added != 2 || delta.Removed != removed || delta.Changed != changed || delta.Areas["other"] != 3 || delta.OmittedCount != 0 || len(delta.Entries) != 3 || delta.Entries[0].KeySHA256 != "ee524f4d1c5fba7f6dcc3e6ab434d1906695e83b698f74078b7bca49caa5e8c9" {
				t.Fatal("removed or type-changed file did not retain priority and exact change counts")
			}
			if delta.Entries[1].KeySHA256 != "7620e3e60c68eb4ed00340bd6c387fb68b29b908d85262bae429fcd3c32b0bb0" || delta.Entries[2].KeySHA256 != "f68f95691ecc72e4f29367a0d7edfe75ad68d46c77e4cac273e67c6a2fd31437" {
				t.Fatal("directory-only changes lost their previous relative hash order")
			}
			qoderFilePriorityRequireUnchanged(t, root, before, after, delta, unchanged)
		})
	}
}

func TestQoderProfileDeltaAllDirectoryOtherChangesKeepPreviousHashOrder(t *testing.T) {
	root := qoderDeltaFixture(t)
	before := qoderDeltaSnapshot(t, root)
	for index := range 5 {
		if err := os.Mkdir(filepath.Join(root, fmt.Sprintf("PRIVATE-KEY-file-%d", index)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	after := qoderDeltaSnapshot(t, root)
	unchanged := qoderDeltaUnchanged(t, root)
	delta := qoderDeltaContract(t, before, after)
	if delta.Status != "entry-limit" || delta.Added != 5 || delta.Removed != 0 || delta.Changed != 0 || delta.Areas["other"] != 5 || delta.OmittedCount != 2 || len(delta.Entries) != 3 {
		t.Fatal("directory-only priority changed previous counts or bounds")
	}
	for index, want := range qoderFilePriorityExpected {
		if delta.Entries[index].KeySHA256 != want.key {
			t.Fatal("directory-only priority changed previous hash order")
		}
		qoderDeltaRequireDetail(t, delta, qoderDeltaEntry{KeySHA256: want.key, Area: "other", Operation: "added", Before: qoderDeltaRecord{Kind: "absent"}, After: qoderDeltaRecord{Kind: "directory", Mode: "20000000700"}})
	}
	qoderFilePriorityRequireUnchanged(t, root, before, after, delta, unchanged)
}
