//go:build linux

package pluginkitairepo_test

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func qoderDeltaPriorityLess(a, b qoderDeltaEntry, otherFirst bool) bool {
	if otherFirst && (a.Area == "other") != (b.Area == "other") {
		return a.Area == "other"
	}
	if otherFirst && a.Area == "other" && b.Area == "other" {
		aFile := a.Before.Kind == "file" || a.After.Kind == "file"
		bFile := b.Before.Kind == "file" || b.After.Kind == "file"
		if aFile != bFile {
			return aFile
		}
	}
	return a.rank < b.rank || (a.rank == b.rank && a.KeySHA256 < b.KeySHA256)
}

func qoderDeltaPriorityFixture(t *testing.T, count int) (string, map[string]string, map[string]string) {
	t.Helper()
	root := qoderDeltaFixture(t)
	before := qoderDeltaSnapshot(t, root)
	for index := range count {
		projectNativeRuntimeWrite(t, filepath.Join(root, fmt.Sprintf("PRIVATE-KEY-%d", index)), []byte(fmt.Sprintf("PRIVATE-BODY-%d\n", index)), 0640)
	}
	for index := range 3 {
		projectNativeRuntimeWrite(t, filepath.Join(root, "logs", "run-private-target", fmt.Sprintf("PRIVATE-KEY-log-%d", index)), []byte("PRIVATE-BODY-log\n"), 0600)
	}
	qoderDeltaMutate(t, root, "same-target-rotation")
	return root, before, qoderDeltaSnapshot(t, root)
}

func qoderDeltaPriorityExpected(before, after map[string]string, count int) []qoderDeltaEntry {
	size, logSize := int64(15), int64(17)
	added := func(key, digest string) qoderDeltaEntry {
		return qoderDeltaEntry{KeySHA256: key, Area: "other", Operation: "added", Before: qoderDeltaRecord{Kind: "absent"}, After: qoderDeltaRecord{Kind: "file", Mode: "640", Bytes: &size, ContentSHA256: digest}}
	}
	if count == 7 {
		return []qoderDeltaEntry{
			added("1cf52e636502d4fbfad917f2d94f3671b785a2d8b4d4177a7c8d23f0b2f60bad", "0bf9b4479f31c38fe151c1ee8166988124d602088321bb0568bad105c279342d"),
			added("4dabb223a442282b5ae1a0cf90be99ae89080e27501c03a2626d4f2c2f85d512", "6543378f10b203e977ea839d673fe1440461fd5c6184fb792dbfa4986988a883"),
			added("53e6490e39a55ba83e28aee3b11eb97ad9cd8b2c37899f1429ada97ac70d293d", "0f31bcd41780292ffd34d27412592d3b411a84a4cf63ff8be35c01fb6b4641dc"),
			added("6797b1822823f37cf3df09000080f3eff8364b6a2d4e7a908a36c610528b144e", "2c22054a7009d10e53452da0dee842749730504c2b53c45e7a5cb6992724e831"),
		}
	}
	link := qoderDeltaEntry{KeySHA256: "22abff24b603cb9288fe82d7f92a98a22a4e3b648b8b0e07db5bc03da0901228", Area: "log-latest", Operation: "changed",
		Before: qoderDeltaRecord{Kind: "symlink", RecordSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(before["logs/latest"])))},
		After:  qoderDeltaRecord{Kind: "symlink", RecordSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(after["logs/latest"])))}}
	log := qoderDeltaEntry{KeySHA256: "85b711de57b6a2434a38cf3a2cb2b55ed13d60ca7ee98be2052a3022bf92d3cc", Area: "logs", Operation: "added", Before: qoderDeltaRecord{Kind: "absent"}, After: qoderDeltaRecord{Kind: "file", Mode: "600", Bytes: &logSize, ContentSHA256: "e2b9c8e8c303354e03149e46c6dafcd17303e3998b2026fb7aee9c6960136a4f"}}
	secondLog := log
	secondLog.KeySHA256 = "a19f11735bceecab78bb9f5261fca65713cc6f5219a632be852509e26f519a4b"
	if count == 1 {
		return []qoderDeltaEntry{added("809103e20c841c150f13982def42e437f1b3c2d60e7c11550d9ec2aeacd02155", "83e787826d4b24b98e62a63c91474209e44436757410c905e885fc36c1aad374"), link, log, secondLog}
	}
	thirdLog := log
	thirdLog.KeySHA256 = "b20a0545710e3fe8e085d613863a77d78dcafcdc3681b1dba27b4f29c3714661"
	return []qoderDeltaEntry{link, log, secondLog, thirdLog}
}

func qoderDeltaReverseInsertion(source map[string]string) map[string]string {
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	result := make(map[string]string, len(source))
	for _, key := range keys {
		result[key] = source[key]
	}
	return result
}

func TestQoderProfileDeltaPrioritizesActualOtherEntriesWithoutChangingCountsOrGuards(t *testing.T) {
	for _, count := range []int{0, 1, 7} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			root, before, after := qoderDeltaPriorityFixture(t, count)
			unchanged := qoderDeltaUnchanged(t, root)
			delta := qoderDeltaContract(t, before, after)
			areas := map[string]int{"log-latest": 1, "logs": 3, "settings": 0, "plugin-cache": 0, "plugins": 0, "cache": 0, "other": count}
			status := "entry-limit"
			if count == 0 {
				status = "captured"
			}
			if delta.Status != status || delta.BeforeEntries != len(before) || delta.AfterEntries != len(before)+count+3 || delta.Added != count+3 || delta.Changed != 1 || delta.Removed != 0 || delta.OmittedCount != count || !reflect.DeepEqual(delta.Areas, areas) || len(delta.Entries) != 4 {
				t.Fatal("priority changed exact whole-profile counts, four-detail bound or omissions")
			}
			for index, want := range qoderDeltaPriorityExpected(before, after, count) {
				if delta.Entries[index].KeySHA256 != want.KeySHA256 {
					t.Fatal("actual selected hashed-name order differs from independently stated expectation")
				}
				qoderDeltaRequireDetail(t, delta, want)
			}
			again := qoderDeltaContract(t, qoderDeltaReverseInsertion(before), qoderDeltaReverseInsertion(after))
			if !reflect.DeepEqual(delta, again) {
				t.Fatal("map insertion order changed priority or retained known-area ordering")
			}
			unchanged()
			if !reflect.DeepEqual(after, qoderDeltaSnapshot(t, root)) || reflect.DeepEqual(before, after) {
				t.Fatal("projection mutated the profile or adopted the unequal original maps")
			}
			// Mutating the returned projection cannot renew either held input map.
			again.Areas["other"] = 999
			again.Entries[0].After.Kind = "changed-returned-kind"
			if again.Entries[0].After.Bytes != nil {
				*again.Entries[0].After.Bytes = 999
			}
			if !reflect.DeepEqual(delta, qoderDeltaContract(t, before, after)) {
				t.Fatal("returned projection aliases a held input or later diagnostic")
			}
		})
	}
}
