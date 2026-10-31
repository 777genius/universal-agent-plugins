//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func windCycleInert(t *testing.T) *windProjectCycle {
	t.Helper()
	w := windComponentInert(t)
	return &windProjectCycle{w: w, aggregates: map[string][]byte{}, removed: map[string]bool{}, markers: map[string][]byte{}, cacheKeys: map[string]bool{}, completed: []string{}, proofs: []string{}}
}

func TestWindsurfCycleKeepsLiteralFortySevenCallsAndOriginalPrefix(t *testing.T) {
	c := windCycleInert(t)
	want := strings.Fields("git-a git-b version empty-user-skills empty-user-mcp empty-a-skills empty-a-mcp empty-b-skills empty-b-mcp add-user user-skills user-mcp user-get add-a a-skills a-mcp a-get b-before-skills b-before-mcp add-b b-skills b-mcp b-get repeat-add-a unchanged-update-a intact-repair-a stable-a-skills stable-b-skills update-a updated-a-skills updated-a-get updated-b-skills damaged-a-skills missing-repair-a repaired-a-skills repaired-a-get remove-a removed-a-skills removed-a-mcp remaining-b-skills remaining-b-mcp repeat-remove-a remove-b final-a-skills final-a-mcp final-b-skills final-b-mcp")
	if len(want) != 47 || !reflect.DeepEqual(windCycleLabels, want) || !reflect.DeepEqual(want[:23], windComponentLabels) {
		t.Fatal("Windsurf literal stage recipe")
	}
	c.w.recipes["a"].Installation.InstallationID = "held-a"
	c.w.recipes["b"].Installation.InstallationID = "held-b"
	public, native := 0, 0
	for _, label := range want {
		command, err := c.selectCommand(label)
		if err != nil {
			t.Fatal(label, err)
		}
		if command.Native {
			native++
			if command.Timeout != 10*time.Second && label != "version" {
				t.Fatal("native deadline")
			}
		} else if windCyclePublic(label) {
			public++
			if command.Timeout != 30*time.Second {
				t.Fatal("public deadline")
			}
		}
		if label == "update-a" && !reflect.DeepEqual(command.Argv, []string{c.w.f.installer, "update", "held-a", "--scope", "project", "--target", "windsurf", "--format", "json", "--project-root", c.w.recipes["a"].Project}) {
			t.Fatal("exact held UUID update")
		}
	}
	if public != 11 || native != 34 {
		t.Fatal("exact public/native/Git cardinality", public, native)
	}
	if _, err := c.selectCommand("approve-all"); err == nil {
		t.Fatal("unregistered action")
	}
}

func TestWindsurfCycleEmptyStreamsAndFailedQualificationStayClosed(t *testing.T) {
	c := windCycleInert(t)
	c.append(t, windCycleMembers[2], "git-a", map[string]any{"events": windCycleStream(nil)})
	raw := c.aggregates[windCycleMembers[2]]
	if !bytes.Contains(raw, []byte(`"body":""`)) || bytes.Contains(raw, []byte(`null`)) {
		t.Fatal("nil byte slice became null wire")
	}
	evidence := map[string]any{"capture_mode": windCycleMode, "test_failed": true, "runtime_attempt_complete": true, "qualification": map[string]bool{"project-lifecycle": true}}
	windCycleFinalizeEvidence(evidence)
	if evidence["runtime_attempt_complete"] != false {
		t.Fatal("late failure retained completion")
	}
	for _, value := range evidence["qualification"].(map[string]bool) {
		if value {
			t.Fatal("late failure qualified")
		}
	}
}

func TestWindsurfCycleActualOwnedMarkerAndLateReplacementRefuse(t *testing.T) {
	c := windCycleInert(t)
	w := c.w
	r := w.recipes["a"]
	raw := windComponentStageActual(t, w, "a")
	if _, err := w.validateProof(r, raw); err != nil {
		t.Fatal(err)
	}
	frames := r.proven
	actionStart := time.Now()
	// The inert files are tiny; keep their writes inside the real action
	// interval despite local sub-millisecond filesystem timestamp granularity.
	time.Sleep(20 * time.Millisecond)
	c.marker(t, r, frames, actionStart)
	if err := w.proofStable(r, frames, raw); err != nil {
		t.Fatal(err)
	}
	w.freezeProof(t, r)
	path := filepath.Join(r.Data, "caller-owned.txt")
	if len(c.proofs) != 1 || c.proofs[0] != "caller-marker-a" {
		t.Fatal("closed marker authoring record")
	}
	before := w.trees[r.StateRoot]
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, c.markers["a"], 0600); err != nil {
		t.Fatal(err)
	}
	if windFrozenTrees(w.f.ctx, map[string]map[string]windProfileRecord{r.StateRoot: before}) == nil {
		t.Fatal("same-byte later marker replacement adopted")
	}
}

func TestWindsurfCycleSelectedParentPreservesUnchangedMtimeButRequiresWindowedCtime(t *testing.T) {
	start := time.Unix(1900000000, 0)
	end := start.Add(time.Second)
	old := windProfileRecord{
		Kind: "directory", Mode: 040700, Device: 2, Inode: 123, Links: 2,
		UID: uint32(os.Getuid()), GID: uint32(os.Getgid()), Size: 4096,
		Modified: [2]int64{start.Unix() - 20, 0},
		Changed: [2]int64{start.Unix() - 20, 0},
	}
	now := old
	now.Changed = [2]int64{start.Unix(), 200000000}
	if !windCycleSelectedParent(old, now, start, end) || windCycleParent(old, now, start, end) {
		t.Fatal("selected chmod must keep old mtime without relaxing native parent proof")
	}
	for _, fault := range []struct {
		name string
		edit func(*windProfileRecord)
	}{
		{"unchanged", func(r *windProfileRecord) { r.Changed = old.Changed }},
		{"stale ctime", func(r *windProfileRecord) { r.Changed = [2]int64{start.Unix() - 1, 0} }},
		{"future ctime", func(r *windProfileRecord) { r.Changed = [2]int64{end.Unix() + 1, 0} }},
		{"invalid nanoseconds", func(r *windProfileRecord) { r.Changed[1] = 1000000000 }},
		{"foreign inode", func(r *windProfileRecord) { r.Inode++ }},
		{"foreign mode", func(r *windProfileRecord) { r.Mode = 040755 }},
		{"old mtime rewritten", func(r *windProfileRecord) { r.Modified = [2]int64{start.Unix() - 1, 0} }},
	} {
		bad := now
		fault.edit(&bad)
		if windCycleSelectedParent(old, bad, start, end) {
			t.Fatal("selected parent accepted", fault.name)
		}
	}
}

func TestWindsurfCycleSelectedMutationLockRequiresActualWindowAndAuthority(t *testing.T) {
	c := windCycleInert(t)
	r := c.w.recipes["a"]
	path := filepath.Join(r.StateRoot, "mutation.lock")
	start := time.Now()
	time.Sleep(20 * time.Millisecond)
	body, _ := json.Marshal(map[string]any{"pid": 4321, "acquired_at": time.Now().UTC().Format(time.RFC3339Nano)})
	// The source writer has a fixed struct order, followed by LF.
	var value map[string]any
	if json.Unmarshal(body, &value) != nil {
		t.Fatal("lock input")
	}
	body = []byte(`{"pid":4321,"acquired_at":"` + value["acquired_at"].(string) + `"}` + "\n")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	end := time.Now()
	tree, err := windOwnedTree(c.w.f.ctx, r.StateRoot)
	if err != nil {
		t.Fatal(err)
	}
	now := tree["mutation.lock"]
	if err := windCycleLock(path, windProfileRecord{}, now, start, end); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"mode", "inode", "links", "owner", "time", "size", "digest"} {
		bad := now
		switch fault {
		case "mode":
			bad.Mode = 0100644
		case "inode":
			bad.Inode = 0
		case "links":
			bad.Links = 2
		case "owner":
			bad.UID++
		case "time":
			bad.Modified = [2]int64{start.Unix() - 1, 0}
		case "size":
			bad.Size++
		case "digest":
			bad.SHA256 = strings.Repeat("0", 64)
		}
		if windCycleLock(path, windProfileRecord{}, bad, start, end) == nil {
			t.Fatal("lock hostile accepted", fault)
		}
	}
}

func TestWindsurfCycleSourceRevisionChangesOnlyThreeHeldFiles(t *testing.T) {
	c := windCycleInert(t)
	r := c.w.recipes["a"]
	probe := r.Authored["bin/probe"]
	before := r.Digest
	actionStart := time.Now()
	time.Sleep(20 * time.Millisecond)
	c.revise(t, actionStart)
	if r.Revision != "a-two" || windCycleVersion(r) != "1.1.0" || before == r.Digest || !bytes.Equal(probe, r.Authored["bin/probe"]) {
		t.Fatal("source revision authority")
	}
	if err := windFrozenTrees(c.w.f.ctx, c.w.trees); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(r.Authored["mcp.json"], []byte("Owned Windsurf a-two")) {
		t.Fatal("revision configuration witness")
	}
}
