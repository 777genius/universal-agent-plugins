//go:build linux

package pluginkitairepo_test

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

func windPublicScratchFrames(t *testing.T) (map[string]windProfileRecord, map[string]windProfileRecord, time.Time, time.Time) {
	t.Helper()
	w := windComponentInert(t)
	scratch := filepath.Join(w.f.base.Root, "tmp")
	for _, name := range []string{"TMPDIR", "TEMP", "TMP"} {
		t.Setenv(name, scratch)
	}
	before := w.profile(t)
	started := time.Now()
	// Keep the actual filesystem operation inside the measured interval even
	// on filesystems whose timestamps lag the wall clock at sub-ms precision.
	time.Sleep(20 * time.Millisecond)
	snapshot, err := (packagesnapshot.Builder{}).Build(w.f.ctx, w.recipes["user"].Source)
	if err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	ended := time.Now()
	after := w.profile(t)
	old, now := before["tmp\x00."], after["tmp\x00."]
	if old.Kind != "directory" || !windRuntimeAuthority(old, now) || old.Size != now.Size || old.SHA256 != now.SHA256 || (old.Modified == now.Modified && old.Changed == now.Changed) {
		t.Fatal("actual default snapshot scratch effect differs")
	}
	unchanged := maps.Clone(after)
	unchanged["tmp\x00."] = old
	if !reflect.DeepEqual(before, unchanged) {
		t.Fatal("default snapshot changed another complete profile record")
	}
	entries, err := os.ReadDir(scratch)
	if err != nil || len(entries) != 0 {
		t.Fatal("default snapshot left scratch entries", err)
	}
	t.Logf("actual snapshot scratch: size_equal=%t modified_equal=%t changed_equal=%t", old.Size == now.Size, old.Modified == now.Modified, old.Changed == now.Changed)
	return before, after, started, ended
}

func TestWindsurfPublicProfileAcceptsOnlyEmptySnapshotScratchTimes(t *testing.T) {
	before, after, start, end := windPublicScratchFrames(t)
	for _, scope := range []string{"user", "a", "b"} {
		if err := windPublicProfileClosure(before, after, scope, start, end); err != nil {
			t.Fatal("ordinary default snapshot refused", scope, err)
		}
	}
}

func TestWindsurfPublicProfileRefusesOtherScratchChanges(t *testing.T) {
	before, after, start, end := windPublicScratchFrames(t)
	for _, fault := range []string{"missing-before", "missing-after", "kind", "mode", "special-mode", "device", "inode", "zero-inode", "uid", "gid", "foreign-owner", "links", "zero-links", "size", "hash", "invalid-ns", "mtime-before", "ctime-after", "reversed-time", "new-child", "original-child", "unknown-scope", "other-role"} {
		t.Run(fault, func(t *testing.T) {
			oldFrame, newFrame := maps.Clone(before), maps.Clone(after)
			old, now := oldFrame["tmp\x00."], newFrame["tmp\x00."]
			scope, first, last := "user", start, end
			switch fault {
			case "missing-before":
				delete(oldFrame, "tmp\x00.")
			case "missing-after":
				delete(newFrame, "tmp\x00.")
			case "kind":
				now.Kind = "file"
			case "mode":
				now.Mode ^= 0001
			case "special-mode":
				old.Mode |= 01000
				now.Mode = old.Mode
			case "device":
				now.Device++
			case "inode":
				now.Inode++
			case "zero-inode":
				old.Inode, now.Inode = 0, 0
			case "uid":
				now.UID++
			case "gid":
				now.GID++
			case "foreign-owner":
				old.UID++
				now.UID = old.UID
			case "links":
				now.Links++
			case "zero-links":
				old.Links, now.Links = 0, 0
			case "size":
				now.Size++
			case "hash":
				now.SHA256 = "unexpected"
			case "invalid-ns":
				now.Modified[1] = int64(time.Second)
			case "mtime-before":
				now.Modified = [2]int64{start.Add(-time.Second).Unix(), int64(start.Nanosecond())}
			case "ctime-after":
				now.Changed = [2]int64{end.Add(time.Second).Unix(), int64(end.Nanosecond())}
			case "reversed-time":
				first, last = end, start
			case "new-child":
				newFrame["tmp\x00leftover"] = windProfileRecord{Kind: "file"}
			case "original-child":
				oldFrame["tmp\x00original"] = windProfileRecord{Kind: "file"}
				newFrame["tmp\x00original"] = oldFrame["tmp\x00original"]
			case "unknown-scope":
				scope = "unknown"
			case "other-role":
				record := newFrame["home\x00."]
				record.Modified = now.Modified
				record.Changed = now.Changed
				newFrame["home\x00."] = record
			}
			if fault != "missing-before" {
				oldFrame["tmp\x00."] = old
			}
			if fault != "missing-after" {
				newFrame["tmp\x00."] = now
			}
			if err := windPublicProfileClosure(oldFrame, newFrame, scope, first, last); err == nil {
				t.Fatal("unrelated scratch/profile change accepted", fault)
			}
		})
	}
}
