//go:build linux

package pluginkitairepo_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func (c *copilotProjectCycle) prepare(t *testing.T) {
	t.Helper()
	f := c.f
	f.evidence["nonce"] = f.base.Nonce
	f.evidence["layout"] = map[string]string{"root": f.base.Root, "home": f.base.Home, "a": f.base.Project, "b": f.other, "outside": f.outside,
		"installer": f.installer, "client": f.client, "git": f.git, "probe": f.base.Probe}
	immutable := map[string]any{}
	for role, path := range map[string]string{"installer": f.installer, "probe": f.base.Probe} {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 64<<20 || f.immutable[path] == "" {
			t.Fatal("Copilot held build artifact identity/size", err)
		}
		immutable[role] = map[string]any{"path": path, "bytes": info.Size(), "sha256": f.immutable[path]}
	}
	c.append(t, "cycle-proof.jsonl", "fixture", map[string]any{"nonce": f.base.Nonce, "layout": f.evidence["layout"], "immutable": immutable})
	for _, key := range []string{"user", "a", "b"} {
		project := f.outside
		switch key {
		case "a":
			project = f.base.Project
		case "b":
			project = f.other
		}
		r := &copilotCycleRecipe{Key: key, Project: project, Source: filepath.Join(f.base.Root, "cycle-source-"+key), Skill: "uapcopilot" + key,
			Revision: key + "-one", Version: "1.0.0", Nonce: copilotProjectConfigHash([]byte(f.base.Nonce + key))[:32]}
		c.recipes[key] = r
		c.author(t, r, false)
		c.callerData(t, r)
	}
	c.originalSkill(t, "user", filepath.Join(f.base.Home, ".copilot", "skills"), "Original user skill must remain unchanged", "personal-copilot")
	c.originalSkill(t, "b", filepath.Join(f.other, ".github", "skills"), "Original project B skill must remain unchanged", "project")
	if err := f.authorOriginalConfig(t); err != nil {
		t.Fatal(err)
	}
	c.check(t)
}

func (c *copilotProjectCycle) callerData(t *testing.T, r *copilotCycleRecipe) {
	t.Helper()
	root := filepath.Join(r.Project, ".uap")
	if r.Key == "user" {
		root = filepath.Join(c.f.base.Root, "installer-state")
	}
	path := filepath.Join(root, "plugin-data", "caller-owned", "keep.txt")
	body := []byte("CALLER_" + r.Key + "_" + c.f.base.Nonce + "\n")
	projectNativeRuntimeWrite(t, path, body, 0600)
	c.holdOriginal(t, path)
	c.holdOriginal(t, filepath.Dir(path))
	tree, err := clineAttemptTree(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	c.trees[filepath.Dir(path)] = tree
	c.f.protected[path] = copilotProjectConfigHash(body)
	c.append(t, "cycle-proof.jsonl", "caller-"+r.Key, map[string]any{"path": path, "body": copilotCycleStream(t, body), "files": tree})
}

func (c *copilotProjectCycle) originalSkill(t *testing.T, key, parent, description, source string) {
	t.Helper()
	name := "uap-foreign-" + key
	body := []byte(fmt.Sprintf("---\nname: %s\ndescription: %s\nuser-invocable: true\n---\nUAP_FOREIGN_%s_%s\n", name, description, strings.ToUpper(key), c.f.base.Nonce))
	path := filepath.Join(parent, name, "SKILL.md")
	projectNativeRuntimeWrite(t, path, body, 0600)
	for _, held := range []string{path, filepath.Dir(path), parent} {
		c.holdOriginal(t, held)
	}
	c.f.protected[path] = copilotProjectConfigHash(body)
	tree, err := clineAttemptTree(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	c.trees[filepath.Dir(path)] = tree
	c.append(t, "cycle-proof.jsonl", "foreign-"+key, map[string]any{"name": name, "description": description, "source": source, "path": path,
		"body": copilotCycleStream(t, body), "files": tree})
}

func (c *copilotProjectCycle) holdOriginal(t *testing.T, path string) {
	t.Helper()
	if err := projectNativeRuntimeParents(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
		t.Fatal("Copilot original ordinary identity required")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) || info.Mode().IsRegular() && stat.Nlink != 1 {
		t.Fatal("Copilot original ownership/link identity differs")
	}
	c.held[path] = info
}
