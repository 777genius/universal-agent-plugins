//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func windContractFIFO(t *testing.T, path string) {
	t.Helper()
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestWindsurfComponentDeltaBindsExactHeldMetadataAndOrdering(t *testing.T) {
	w := windComponentInert(t)
	root := w.f.base.Root
	before, err := windProfileSnapshot(w.f.ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "xdg-config", "devin", "config.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(original, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := windProfileSnapshot(w.f.ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	d := windProfileDifference(before, after)
	if d.Status != "captured" || len(d.Details) != 1 || d.Total != (windDeltaCount{Changed: 1}) {
		t.Fatal("exact single file delta", d)
	}
	detail := d.Details[0]
	record := after["xdg-config\x00devin/config.json"]
	raw, _ := json.Marshal(record)
	if detail.Root != "xdg-config" || detail.KeySHA256 != windsurfConfigDigest([]byte("xdg-config\x00devin/config.json")) || detail.Operation != "changed" || detail.After.Kind != "file" || detail.After.Mode != unix.S_IFREG|0600 || detail.After.Bytes == nil || *detail.After.Bytes != int64(len(original)+1) || detail.After.ContentSHA256 != windsurfConfigDigest(append(original, '\n')) || detail.After.RecordSHA256 != windsurfConfigDigest(raw) {
		t.Fatal("finite metadata not bound to exact held record")
	}
	copyBefore := map[string]windProfileRecord{}
	for key, value := range before {
		copyBefore[key] = value
	}
	copyAfter := map[string]windProfileRecord{}
	for key, value := range after {
		copyAfter[key] = value
	}
	d.Details[0].Root = "mutated-output"
	if !reflect.DeepEqual(before, copyBefore) || !reflect.DeepEqual(after, copyAfter) {
		t.Fatal("projection aliased input")
	}
	for _, kind := range []string{"mode", "inode", "parent"} {
		t.Run(kind, func(t *testing.T) {
			start, err := windProfileSnapshot(w.f.ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "mode":
				if err = os.Chmod(path, 0640); err != nil {
					t.Fatal(err)
				}
			case "inode":
				if err = os.Remove(path); err != nil {
					t.Fatal(err)
				}
				projectNativeRuntimeWrite(t, path, original, 0600)
			case "parent":
				if err = os.Chmod(filepath.Dir(path), 0750); err != nil {
					t.Fatal(err)
				}
			}
			end, err := windProfileSnapshot(w.f.ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			if reflect.DeepEqual(start, end) || windProfileDifference(start, end).Total.Changed == 0 {
				t.Fatal("identity mutation hidden")
			}
		})
	}
}
func TestWindsurfComponentProfileHeldEdgesRejectReplacementAndClosedHandles(t *testing.T) {
	w := windComponentInert(t)
	path := filepath.Join(w.f.base.Root, "tmp")
	edges, err := windOpenDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(path, path+"-old"); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if windCheckEdges(edges) == nil {
		t.Fatal("replacement linked edge accepted")
	}
	if windCloseEdges(edges) != nil {
		t.Fatal("checked ordinary closes")
	}
	if windCheckEdges(edges) == nil || windCloseEdges(edges) == nil {
		t.Fatal("closed descriptor/close error hidden")
	}
}
func TestWindsurfComponentFrameReadAndGlobalBudgetRefusalsRemainClosed(t *testing.T) {
	w := windComponentInert(t)
	path := filepath.Join(w.f.base.Root, "tmp", "held")
	projectNativeRuntimeWrite(t, path, []byte("private"), 0600)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	before, err := windFileStat(file)
	if err != nil {
		t.Fatal(err)
	}
	walker := &windFrameWalk{ctx: w.f.ctx, records: map[string]windProfileRecord{}, bytes: 256 << 20}
	if walker.regular(file, "tmp\x00held", 1, before) == nil || len(walker.records) != 0 {
		t.Fatal("global budget silently omitted body")
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	walker.bytes = 0
	if walker.regular(file, "tmp\x00held", 1, before) == nil || len(walker.records) != 0 {
		t.Fatal("closed read became successful frame")
	}
}
func TestWindsurfComponentDeltaRefusesImpossibleRecordsAndRetainsBoundedExactCounts(t *testing.T) {
	record := windProfileRecord{Kind: "file", Mode: unix.S_IFREG | 0600, Size: 1, SHA256: windsurfConfigDigest([]byte("a"))}
	before, after := map[string]windProfileRecord{}, map[string]windProfileRecord{}
	for _, name := range []string{"first", "second", "third", "fourth"} {
		key := "home\x00" + name
		before[key] = record
		changed := record
		changed.SHA256 = windsurfConfigDigest([]byte("b"))
		after[key] = changed
	}
	d := windProfileDifference(before, after)
	raw, err := json.Marshal(d)
	if err != nil || len(raw) > 2<<10 || d.Total != (windDeltaCount{Changed: 4}) || d.Omitted != 4-len(d.Details) {
		t.Fatal("bounded counts lost", d)
	}
	reversed := map[string]windProfileRecord{}
	for _, name := range []string{"fourth", "third", "second", "first"} {
		reversed["home\x00"+name] = after["home\x00"+name]
	}
	if !reflect.DeepEqual(d, windProfileDifference(before, reversed)) {
		t.Fatal("map iteration changed selected details")
	}
	for _, kind := range []string{"type", "hash", "size", "key", "entries"} {
		candidate := map[string]windProfileRecord{"home\x00leaf": record}
		switch kind {
		case "type":
			bad := record
			bad.Kind = "symlink"
			candidate["home\x00leaf"] = bad
		case "hash":
			bad := record
			bad.SHA256 = "PRIVATE"
			candidate["home\x00leaf"] = bad
		case "size":
			bad := record
			bad.Size = (64 << 20) + 1
			candidate["home\x00leaf"] = bad
		case "key":
			candidate = map[string]windProfileRecord{"unknown\x00PRIVATE": record}
		case "entries":
			for i := range 4097 {
				candidate[string(rune(i))+"PRIVATE"] = record
			}
		}
		got := windProfileDifference(before, candidate)
		if got.Status != "record-refused" && got.Status != "inventory-bound" {
			t.Fatal("impossible projected record admitted", kind)
		}
		encoded, err := json.Marshal(got)
		if err != nil || len(encoded) > 2<<10 || strings.Contains(string(encoded), "PRIVATE") {
			t.Fatal("refusal privacy")
		}
	}
}
func TestWindsurfComponentCWDClosureFreezesUnknownSiblingsAndGitAuthority(t *testing.T) {
	w := windComponentInert(t)
	r := w.recipes["a"]
	before := w.trees[r.Project]
	projectNativeRuntimeWrite(t, filepath.Join(r.Project, ".git", "config"), []byte("inert Git fixture; not executed"), 0600)
	after, err := windOwnedTree(w.f.ctx, r.Project)
	if err != nil {
		t.Fatal(err)
	}
	if windCWDMutation(before, after, "git") != nil {
		t.Fatal("exact Git-only setup refused")
	}
	if windCWDMutation(before, after, "project-public") == nil || windCWDMutation(before, after, "user-public") == nil || windCWDMutation(after, after, "git") == nil {
		t.Fatal("unselected or preexisting Git authority adopted")
	}
	projectNativeRuntimeWrite(t, filepath.Join(r.Project, "unknown-native.log"), []byte("PRIVATE"), 0600)
	changed, err := windOwnedTree(w.f.ctx, r.Project)
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"git", "project-public", "user-public"} {
		if windCWDMutation(before, changed, operation) == nil {
			t.Fatal("unexpected cwd sibling admitted", operation)
		}
	}
	if w.gitFreeze("git-a") == nil {
		t.Fatal("Git freeze accepted unknown cwd sibling")
	}
	if _, exists := w.trees[filepath.Join(r.Project, ".git")]; exists {
		t.Fatal("refused Git mutation registered partial baseline")
	}
	if !reflect.DeepEqual(before, w.trees[r.Project]) {
		t.Fatal("refused cwd mutation adopted a baseline")
	}
}
func TestWindsurfComponentNativeCWDGuardRejectsUnknownFilesInAllThreeRoots(t *testing.T) {
	for _, scope := range []string{"user", "a", "b"} {
		t.Run(scope, func(t *testing.T) {
			w := windComponentInert(t)
			r := w.recipes[scope]
			if err := windFrozenTrees(w.f.ctx, w.trees); err != nil {
				t.Fatal(err)
			}
			projectNativeRuntimeWrite(t, filepath.Join(r.Project, "unregistered-session.json"), []byte("PRIVATE"), 0600)
			if windFrozenTrees(w.f.ctx, w.trees) == nil {
				t.Fatal("native cwd addition escaped complete original cwd guard")
			}
		})
	}
}
