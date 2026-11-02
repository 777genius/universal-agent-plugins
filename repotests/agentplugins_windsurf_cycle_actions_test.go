//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
)

func windCycleWrite(path string, want windProfileRecord, body []byte, create bool) (err error) {
	edges, err := windOpenDirectory(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer func() {
		if windCloseEdges(edges) != nil {
			err = fmt.Errorf("Windsurf fixture parent close")
		}
	}()
	parent := edges[len(edges)-1].file
	flags := unix.O_WRONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC
	if create {
		flags |= unix.O_CREAT | unix.O_EXCL
	}
	if err := windCheckEdges(edges); err != nil {
		return err
	}
	fd, err := unix.Openat(int(parent.Fd()), filepath.Base(path), flags, 0600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), "held-fixture-action")
	defer func() {
		if file.Close() != nil {
			err = fmt.Errorf("Windsurf fixture file close")
		}
	}()
	stat, err := windFileStat(file)
	if err != nil || !windOwned(stat) || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Nlink != 1 || !create && windRecord(stat, "file", want.SHA256) != want {
		return fmt.Errorf("Windsurf fixture held file authority")
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Write(body); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	after, err := windFileStat(file)
	if err != nil || !windDirectoryIdentity(stat, after) {
		return fmt.Errorf("Windsurf fixture file replaced")
	}
	var linked unix.Stat_t
	if unix.Fstatat(int(parent.Fd()), filepath.Base(path), &linked, unix.AT_SYMLINK_NOFOLLOW) != nil || !windSameStat(after, linked) {
		return fmt.Errorf("Windsurf fixture edge replaced")
	}
	return nil
}

func (c *windProjectCycle) marker(t *testing.T, r *windComponentRecipe, frames map[string]map[string]windProfileRecord, start time.Time) {
	t.Helper()
	data, err := windOwnedTree(c.w.f.ctx, r.Data)
	if err != nil || len(data) != 2 || data["."].Mode != 040700 || data[".agentplugins-data-owner.json"].Kind != "file" {
		t.Fatal("Windsurf first marker requires exact proven owned data")
	}
	body := []byte("WINDSURF_CALLER_" + r.Scope + "_" + c.w.f.base.Nonce + "\n")
	name := filepath.Join(r.Data, "caller-owned.txt")
	if err := windCycleWrite(name, windProfileRecord{}, body, true); err != nil {
		t.Fatal(err)
	}
	end := time.Now()
	after, err := c.w.proofFrames(r)
	if err != nil {
		t.Fatal(err)
	}
	for root, before := range frames {
		marker, relerr := filepath.Rel(root, name)
		if relerr != nil {
			t.Fatal(relerr)
		}
		contains := marker != ".." && !strings.HasPrefix(marker, "../")
		for key, old := range before {
			now, ok := after[root][key]
			if ok && old == now {
				continue
			}
			if contains && key == filepath.Dir(marker) && windRuntimeAuthority(old, now) && old.SHA256 == "" && now.SHA256 == "" && now.Size >= 0 && now.Size <= 1<<20 && windRuntimeCommandTimes(now, start, end) {
				continue
			}
			t.Fatalf("Windsurf caller marker changed unrelated frame: parent=%t present=%t authority=%t times=%t mtime_start_ns=%d ctime_start_ns=%d", contains && key == filepath.Dir(marker), ok, windRuntimeAuthority(old, now), windRuntimeCommandTimes(now, start, end), time.Unix(now.Modified[0], now.Modified[1]).Sub(start).Nanoseconds(), time.Unix(now.Changed[0], now.Changed[1]).Sub(start).Nanoseconds())
		}
		added := 0
		for key, record := range after[root] {
			if _, existed := before[key]; existed {
				continue
			}
			added++
			if !contains || key != marker || record.Kind != "file" || record.Mode != 0100600 || record.Links != 1 || record.UID != uint32(os.Getuid()) || record.GID != uint32(os.Getgid()) || record.SHA256 != clineAttemptSHA(body) || record.Size != int64(len(body)) || !windRuntimeCommandTimes(record, start, end) {
				t.Fatal("Windsurf caller marker exact addition")
			}
		}
		if contains && added != 1 || !contains && added != 0 {
			t.Fatal("Windsurf caller marker membership")
		}
		// The complete comparison above validates the only fixture-authored delta.
		frames[root] = after[root]
	}
	if err := windFrozenTrees(c.w.f.ctx, frames); err != nil {
		t.Fatal(err)
	}
	c.markers[r.Scope] = body
	c.proof(t, "caller-marker-"+r.Scope, map[string]any{"scope": r.Scope, "path": name, "started_at": start.UTC().Format(time.RFC3339Nano), "ended_at": end.UTC().Format(time.RFC3339Nano), "marker": windCycleStream(body), "before": data, "after": mustWindCycleData(t, c, r)})
}

func windCycleDigest(ctx context.Context, authored map[string][]byte) (string, error) {
	entries := make([]packagedigest.CapturedEntry, 0, len(authored)*3)
	dirs := map[string]bool{}
	for name, body := range authored {
		entries = append(entries, packagedigest.CapturedEntry{Path: name, Kind: "file", Executable: name == "bin/probe", Content: body})
		for dir := filepath.ToSlash(filepath.Dir(name)); dir != "."; dir = filepath.ToSlash(filepath.Dir(dir)) {
			dirs[dir] = true
		}
	}
	for dir := range dirs {
		entries = append(entries, packagedigest.CapturedEntry{Path: dir, Kind: "directory"})
	}
	return packagedigest.DigestCaptured(ctx, entries)
}

func (c *windProjectCycle) revise(t *testing.T, start time.Time) {
	t.Helper()
	w, r := c.w, c.w.recipes["a"]
	w.check(t)
	before := w.trees[r.Source]
	authored := map[string][]byte{}
	for name, body := range r.Authored {
		authored[name] = bytes.Clone(body)
	}
	var manifest map[string]any
	if json.Unmarshal(authored["plugin.json"], &manifest) != nil {
		t.Fatal("Windsurf authored manifest")
	}
	manifest["version"] = "1.1.0"
	authored["plugin.json"], _ = json.Marshal(manifest)
	nonce := clineAttemptSHA([]byte(w.f.base.Nonce + "a-two"))[:32]
	authored["skills/"+windComponentName+"/SKILL.md"] = []byte("---\nname: " + windComponentName + "\ndescription: a-two\n---\nUAP_WINDSURF_a-two_" + nonce + "\n")
	var mcp map[string]any
	if json.Unmarshal(authored["mcp.json"], &mcp) != nil {
		t.Fatal("Windsurf authored MCP")
	}
	server := mcp["mcpServers"].(map[string]any)[windComponentName].(map[string]any)
	server["args"].([]any)[0] = "Owned Windsurf a-two"
	env := server["env"].(map[string]any)
	env["UAP_TEST_NONCE"], env["UAP_TEST_REVISION"] = nonce, "a-two"
	authored["mcp.json"], _ = json.Marshal(mcp)
	for _, name := range []string{"plugin.json", "mcp.json", "skills/" + windComponentName + "/SKILL.md"} {
		if err := windCycleWrite(filepath.Join(r.Source, name), before[name], authored[name], false); err != nil {
			t.Fatal(err)
		}
	}
	end := time.Now()
	after, err := windOwnedTree(w.f.ctx, r.Source)
	if err != nil || len(after) != len(before) {
		t.Fatal("Windsurf exact source revision frame", err)
	}
	for name, old := range before {
		now := after[name]
		body := authored[name]
		if bytes.Equal(body, r.Authored[name]) || old.Kind == "directory" {
			if now != old {
				t.Fatal("Windsurf revision changed unselected source record")
			}
			continue
		}
		if !windRuntimeAuthority(old, now) || now.SHA256 != clineAttemptSHA(body) || now.Size != int64(len(body)) || !windRuntimeCommandTimes(now, start, end) {
			t.Fatalf("Windsurf exact revised source authority: authority=%t hash=%t size=%t times=%t mtime_start_ns=%d ctime_start_ns=%d", windRuntimeAuthority(old, now), now.SHA256 == clineAttemptSHA(body), now.Size == int64(len(body)), windRuntimeCommandTimes(now, start, end), time.Unix(now.Modified[0], now.Modified[1]).Sub(start).Nanoseconds(), time.Unix(now.Changed[0], now.Changed[1]).Sub(start).Nanoseconds())
		}
	}
	r.Authored, r.Revision, r.Nonce = authored, "a-two", nonce
	r.Digest, err = windCycleDigest(w.f.ctx, authored)
	if err != nil {
		t.Fatal(err)
	}
	if err := windFrozenTrees(w.f.ctx, map[string]map[string]windProfileRecord{r.Source: after}); err != nil {
		t.Fatal(err)
	}
	w.trees[r.Source] = after
	c.proof(t, "source-a-two", map[string]any{"source": r.Source, "revision": r.Revision, "nonce": r.Nonce, "source_digest": r.Digest, "manifest": windCycleStream(authored["plugin.json"]), "mcp": windCycleStream(authored["mcp.json"]), "skill": windCycleStream(authored["skills/"+windComponentName+"/SKILL.md"])})
	w.check(t)
}

func (c *windProjectCycle) damage(t *testing.T) {
	t.Helper()
	w, r := c.w, c.w.recipes["a"]
	start := time.Now()
	w.check(t)
	directory := filepath.Join(r.Profile, "skills", windComponentName)
	tree, err := windOwnedTree(w.f.ctx, directory)
	if err != nil || len(tree) != 2 || tree["SKILL.md"].SHA256 != clineAttemptSHA(r.Authored["skills/"+windComponentName+"/SKILL.md"]) {
		t.Fatal("Windsurf exact owned skill fault authority")
	}
	if err := os.Remove(filepath.Join(directory, "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(directory); err != nil {
		t.Fatal(err)
	}
	end := time.Now()
	updated := map[string]map[string]windProfileRecord{}
	for root, before := range w.trees {
		rel, _ := filepath.Rel(root, directory)
		if rel == ".." || strings.HasPrefix(rel, "../") {
			updated[root] = before
			continue
		}
		after, err := windOwnedTree(w.f.ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if len(after) != len(before)-2 {
			t.Fatal("Windsurf fault exact membership")
		}
		for name, old := range before {
			now, has := after[name]
			if name == rel || name == rel+"/SKILL.md" {
				if has {
					t.Fatal("Windsurf fault leaf remains")
				}
				continue
			}
			if !has || old != now && (name != filepath.Dir(rel) || !windCycleParent(old, now, start, end) || old.Links != now.Links+1) {
				t.Fatal("Windsurf fault changed unrelated entry")
			}
		}
		updated[root] = after
	}
	if err := windFrozenTrees(w.f.ctx, updated); err != nil {
		t.Fatal(err)
	}
	w.trees = updated
	c.proof(t, "missing-owned-a-skill", map[string]any{"scope": "a", "path": directory, "skill_sha256": tree["SKILL.md"].SHA256, "started_at": start.UTC().Format(time.RFC3339Nano), "ended_at": end.UTC().Format(time.RFC3339Nano)})
	w.check(t)
}

func mustWindCycleData(t *testing.T, c *windProjectCycle, r *windComponentRecipe) map[string]windProfileRecord {
	t.Helper()
	frame, err := windOwnedTree(c.w.f.ctx, r.Data)
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func windCycleProbe(t *testing.T, c *windProjectCycle) windProfileRecord {
	t.Helper()
	record, err := windCycleProbeRecord(c)
	if err != nil {
		t.Fatal(err)
	}
	return record
}
func windCycleProbeRecord(c *windProjectCycle) (record windProfileRecord, err error) {
	path := c.w.f.base.Events
	edges, err := windOpenDirectory(filepath.Dir(path))
	if err != nil {
		return record, err
	}
	defer func() {
		if windCloseEdges(edges) != nil {
			err = fmt.Errorf("Windsurf probe parent close")
		}
	}()
	walker := &windFrameWalk{ctx: c.w.f.ctx, records: map[string]windProfileRecord{}}
	if err = walker.child(edges[len(edges)-1].file, filepath.Base(path), "event", 0); err != nil {
		return record, err
	}
	record = walker.records["event"]
	if record.Mode != 0100600 || record.Size != 0 || record.Links != 1 || record.SHA256 != clineAttemptSHA(nil) {
		return record, fmt.Errorf("Windsurf empty event authority")
	}
	return record, nil
}
