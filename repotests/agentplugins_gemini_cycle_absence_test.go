//go:build linux

package pluginkitairepo_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func geminiCycleOwnedSkill(r *geminiCycleRecipe) domain.NativeObjectOwnership {
	body := r.Authored["skills/"+r.Name+"/SKILL.md"]
	return domain.NativeObjectOwnership{ObjectID: "gemini-skill:" + r.Name, Kind: "gemini_global_skill_directory", LogicalName: r.Name, Path: filepath.Dir(r.SkillPath), SourceRelative: "skills/" + r.Name, ManagedDigest: "sha256:" + geminiCycleSHA(append(append([]byte("f\x00SKILL.md\x00-\x00"), body...), 0)), ProtectionClass: "managed"}
}
func geminiCycleRemoveOwnedSkill(ctx context.Context, r *geminiCycleRecipe) (err error) {
	if r.Scope != "a" || r.Revision != "A-two" || r.Removed || r.Name != "uapgeminia" || r.SkillPath != filepath.Join(r.Profile, "skills", r.Name, "SKILL.md") {
		return fmt.Errorf("Gemini exact project-a owned absence fault")
	}
	want := geminiCycleOwnedSkill(r)
	count := 0
	for _, object := range r.Binding.NativeObjects {
		if object.Kind == want.Kind {
			count++
			if !reflect.DeepEqual(object, want) {
				return fmt.Errorf("Gemini fault native ownership differs")
			}
		}
	}
	if count != 1 {
		return fmt.Errorf("Gemini fault requires exactly one owned skill")
	}
	edges, err := geminiCycleOpen(filepath.Join(r.Profile, "skills"))
	if err != nil {
		return err
	}
	defer func() {
		if e := geminiCycleClose(edges); e != nil {
			err = fmt.Errorf("Gemini fault parent checked close")
		}
	}()
	parent := edges[len(edges)-1].file
	fd, err := unix.Openat(int(parent.Fd()), r.Name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("Gemini fault owned directory open")
	}
	dir := os.NewFile(uintptr(fd), "owned-skill")
	defer func() {
		if e := dir.Close(); e != nil {
			err = fmt.Errorf("Gemini fault directory checked close")
		}
	}()
	dirStat, err := geminiCycleStat(dir)
	if err != nil || dirStat.Mode != 0040000|0755 || dirStat.UID != uint32(os.Getuid()) || dirStat.GID != uint32(os.Getgid()) {
		return fmt.Errorf("Gemini fault exact directory owner/mode")
	}
	var linked unix.Stat_t
	if unix.Fstatat(int(dir.Fd()), "SKILL.md", &linked, unix.AT_SYMLINK_NOFOLLOW) != nil || !geminiCycleFaultLeaf(geminiCycleRecordOf(linked)) {
		return fmt.Errorf("Gemini fault linked ordinary bounded skill required")
	}
	initialLinked := geminiCycleRecordOf(linked)
	leafFD, err := unix.Openat(int(dir.Fd()), "SKILL.md", unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("Gemini fault skill open")
	}
	leaf := os.NewFile(uintptr(leafFD), "owned-skill-leaf")
	defer func() {
		if e := leaf.Close(); e != nil {
			err = fmt.Errorf("Gemini fault skill checked close")
		}
	}()
	before, e := geminiCycleStat(leaf)
	if e != nil || before != initialLinked || !geminiCycleFaultLeaf(before) {
		return fmt.Errorf("Gemini fault held ordinary bounded skill required")
	}
	body, readErr := io.ReadAll(io.LimitReader(leaf, (1<<20)+1))
	after, statErr := geminiCycleStat(leaf)
	linkErr := unix.Fstatat(int(dir.Fd()), "SKILL.md", &linked, unix.AT_SYMLINK_NOFOLLOW)
	if readErr != nil || statErr != nil || linkErr != nil || before != after || before != geminiCycleRecordOf(linked) || int64(len(body)) != before.Size || geminiCycleSHA(body) != geminiCycleSHA(r.Authored["skills/"+r.Name+"/SKILL.md"]) {
		return fmt.Errorf("Gemini fault independent held skill bytes/identity")
	}
	names, e := dir.Readdirnames(2)
	if e != nil && !errors.Is(e, io.EOF) || len(names) != 1 || names[0] != "SKILL.md" || ctx.Err() != nil || geminiCycleEdges(edges) != nil {
		return fmt.Errorf("Gemini fault exact skill directory membership")
	}
	if unix.Unlinkat(int(dir.Fd()), "SKILL.md", 0) != nil {
		return fmt.Errorf("Gemini checked owned leaf removal")
	}
	current, e := geminiCycleStat(dir)
	linkErr = unix.Fstatat(int(parent.Fd()), r.Name, &linked, unix.AT_SYMLINK_NOFOLLOW)
	if e != nil || linkErr != nil || !geminiCycleSameDirectory(dirStat, current) || current != geminiCycleRecordOf(linked) {
		return fmt.Errorf("Gemini fault owned directory replaced")
	}
	if unix.Unlinkat(int(parent.Fd()), r.Name, unix.AT_REMOVEDIR) != nil {
		return fmt.Errorf("Gemini nonrecursive owned directory removal")
	}
	if e = unix.Fstatat(int(parent.Fd()), r.Name, &linked, unix.AT_SYMLINK_NOFOLLOW); !errors.Is(e, unix.ENOENT) || ctx.Err() != nil {
		return fmt.Errorf("Gemini exact owned final absence")
	}
	parentNow, e := geminiCycleStat(parent)
	if e != nil || !geminiCycleSameDirectory(edges[len(edges)-1].initial, parentNow) {
		return fmt.Errorf("Gemini absence parent authority changed")
	}
	edges[len(edges)-1].initial = parentNow
	return geminiCycleEdges(edges)
}
func geminiCycleFaultLeaf(record geminiCycleRecord) bool {
	return record.Mode == 0100000|0644 && record.Links == 1 && record.UID == uint32(os.Getuid()) && record.GID == uint32(os.Getgid()) && record.Size >= 0 && record.Size <= 1<<20
}
func geminiCycleMissingDelta(before, after map[string]geminiCycleRecord, prefix string) error {
	parents := map[string]bool{}
	for p := filepath.Dir(prefix); ; p = filepath.Dir(p) {
		parents[p] = true
		if p == "." {
			break
		}
	}
	for name := range geminiCycleKeys(before, after) {
		a, had := before[name]
		b, has := after[name]
		if had && has && a == b {
			continue
		}
		if (name == prefix || name == filepath.Join(prefix, "SKILL.md")) && had && !has {
			continue
		}
		if parents[name] && had && has && geminiCycleSameDirectory(a, b) {
			continue
		}
		return fmt.Errorf("Gemini absence fault changed unrelated authority")
	}
	return nil
}
func (c *geminiProjectCycle) missingA(t *testing.T) {
	t.Helper()
	c.stage = "missing-owned-skill-a"
	r := c.recipes["a"]
	c.check(t)
	c.proof(t, r, "before-missing-a")
	if err := geminiCycleRemoveOwnedSkill(c.f.ctx, r); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{r.Project, filepath.Join(r.Profile, "skills")} {
		after, err := geminiCycleTree(c.f.ctx, path)
		prefix, _ := filepath.Rel(path, filepath.Dir(r.SkillPath))
		if err != nil || geminiCycleMissingDelta(c.frozen[path], after, prefix) != nil {
			t.Fatal("Gemini exact controlled absence delta")
		}
		c.frozen[path] = after
	}
	c.absent[filepath.Dir(r.SkillPath)] = true
	c.absent[r.SkillPath] = true
	c.check(t)
}
