//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"
)

func (c *qoderSkillsCycle) suspend(r *qoderCycleRecipe) {
	delete(c.f.protected, c.statePath(r))
	if r.Active != "" {
		delete(c.trees, r.Active)
	}
}

func (c *qoderSkillsCycle) apply(t *testing.T, label, key string, mutated bool) {
	t.Helper()
	r := c.recipes[key]
	c.check(t)
	var before []byte
	if !mutated {
		before = mustQoderCycleRead(t, c.statePath(r))
	}
	c.suspend(r)
	c.mutable = key
	if label == "missing-repair-a" {
		r.Missing = false
	}
	result := c.publicCommand(t, label)
	if label != "repeat-remove-a" {
		verb := strings.Split(label, "-")[0]
		switch label {
		case "repeat-add-a":
			verb = "add"
		case "unchanged-update-a":
			verb = "update"
		case "intact-repair-a", "missing-repair-a":
			verb = "repair"
		}
		if err := qoderCyclePublicJSON(result.Stdout, verb, mutated); err != nil {
			t.Fatal(err)
		}
		if string(result.Stderr) != qoderCycleProgress(label) {
			t.Fatal("Qoder public progress differs from exact source contract")
		}
	}
	if !mutated && !bytes.Equal(before, mustQoderCycleRead(t, c.statePath(r))) {
		t.Fatal("Qoder repeated/no-op public operation changed state bytes")
	}
	if strings.HasPrefix(label, "remove-") {
		r.Removed = true
	}
	c.proof(t, r, label)
	if label != "repeat-remove-a" {
		_, args := c.publicArgs(label)
		group, err := qoderCyclePublicProof(result.Stdout, label, args[1], mutated, r)
		if err != nil || label != "repeat-add-a" && c.groups[group] {
			t.Fatal("Qoder public result differs from exact current proof or repeats a group", err)
		}
		if group != "" {
			c.groups[group] = true
		}
	}
	c.mutable = ""
	c.check(t)
}

func qoderCycleProgress(label string) string {
	switch label {
	case "add-a", "add-b":
		return "Resolving and validating one Agent Plugin package for every selected target...\nInstalling " + qoderCycleName + " for 1 selected clients...\n"
	case "repeat-add-a":
		return "Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"
	case "unchanged-update-a", "update-a":
		return "Resolving and validating one updated Agent Plugin package for every selected target...\n"
	case "intact-repair-a", "missing-repair-a":
		return "Resolving and validating each unique exact installed package revision once...\n"
	default:
		return ""
	}
}

func (c *qoderSkillsCycle) reviseA(t *testing.T) {
	t.Helper()
	r := c.recipes["a"]
	c.proof(t, r, "before-source-revision-a")
	delete(c.trees, r.Source)
	r.Revision = "a-two"
	c.author(t, r, true)
	c.check(t)
}

func (c *qoderSkillsCycle) missingA(t *testing.T) {
	t.Helper()
	r := c.recipes["a"]
	c.proof(t, r, "before-missing-a")
	c.check(t)
	c.removeOwnedA(t)
}

func (c *qoderSkillsCycle) removeOwnedA(t *testing.T) {
	t.Helper()
	r := c.recipes["a"]
	if r.Revision != "a-two" || r.Removed || r.Missing || len(r.Binding.NativeObjects) != 1 || r.Binding.NativeObjects[0].Path != r.Active {
		t.Fatal("Qoder controlled fault requires exact whole owned frozen A-two package")
	}
	before, err := clineAttemptTree(r.Active)
	if err != nil || !reflect.DeepEqual(before, c.trees[r.Active]) {
		t.Fatal("Qoder fault differs from the last independently owned payload", err)
	}
	held := map[string]os.FileInfo{}
	paths := make([]string, 0, len(before))
	for relative := range before {
		path := filepath.Join(r.Active, relative)
		if err := projectNativeRuntimeParents(path); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
			t.Fatal("Qoder fault requires ordinary exact owned membership")
		}
		stat := info.Sys().(*syscall.Stat_t)
		if stat.Uid != uint32(os.Getuid()) || stat.Gid != uint32(os.Getgid()) || info.Mode().IsRegular() && stat.Nlink != 1 {
			t.Fatal("Qoder fault ownership/link differs")
		}
		if info.Mode().IsRegular() {
			body := mustQoderCycleRead(t, path)
			current, err := os.Lstat(path)
			if err != nil || !projectNativeRuntimeStableStat(info.Sys(), current.Sys()) || before[relative] != fmt.Sprintf("file:%o:%s", info.Mode(), qoderDeltaHash(string(body))) {
				t.Fatal("Qoder fault file bytes/identity changed before authorization")
			}
		}
		held[relative] = info
		paths = append(paths, relative)
	}
	sort.Slice(paths, func(i, j int) bool {
		return len(strings.Split(paths[i], string(filepath.Separator))) > len(strings.Split(paths[j], string(filepath.Separator))) || len(strings.Split(paths[i], string(filepath.Separator))) == len(strings.Split(paths[j], string(filepath.Separator))) && paths[i] > paths[j]
	})
	// The top '.' directory is removed only after every known child; no recursive
	// remover can consume an unexpected foreign leaf introduced during the fault.
	delete(c.trees, r.Active)
	for _, relative := range paths {
		if relative == "." {
			continue
		}
		path := filepath.Join(r.Active, relative)
		now, err := os.Lstat(path)
		if err != nil || !os.SameFile(held[relative], now) || now.Mode() != held[relative].Mode() || now.Mode().IsRegular() && !projectNativeRuntimeStableStat(held[relative].Sys(), now.Sys()) {
			t.Fatal("Qoder held fault identity changed")
		}
		if err := projectNativeRuntimeParents(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	current, err := os.Lstat(r.Active)
	if err != nil || !os.SameFile(held["."], current) || !current.IsDir() {
		t.Fatal("Qoder whole owned root changed before final nonrecursive removal")
	}
	if err := os.Remove(r.Active); err != nil {
		t.Fatal(err)
	}
	if err := qoderCycleAbsent(r.Active); err != nil {
		t.Fatal(err)
	}
	r.Missing = true
	c.append(t, qoderCycleAggregates[2], "missing-a", map[string]any{"path": r.Active, "source_digest": r.Digest, "object": r.Binding.NativeObjects[0], "before": before, "absent": true})
	c.check(t)
}

func TestAgentpluginsQoderNativeProjectCompleteSkillsCycle(t *testing.T) {
	spec, err := projectNativeCLISpecification("qoder")
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv(spec.optin) != "1" {
		t.Skip("opt-in genuine hosted Qoder supported skills catalog cycle")
	}
	f := newProjectNativeCLIAttempt(t, spec)
	c := newQoderSkillsCycle(f)
	defer c.finish(t)
	c.prepare(t)
	for _, label := range []string{"git-a", "git-b", "git-outside"} {
		result := c.publicCommand(t, label)
		if len(result.Stdout) != 0 || len(result.Stderr) != 0 {
			t.Fatal("Qoder real Git setup diagnostics")
		}
	}
	result := c.publicCommand(t, "version")
	if string(result.Stdout) != "1.1.78\n" || len(result.Stderr) != 0 {
		t.Fatal("Qoder exact pinned native version")
	}
	for _, stage := range []string{"empty-outside", "empty-a", "empty-b"} {
		c.catalog(t, stage)
	}
	c.apply(t, "add-a", "a", true)
	c.catalog(t, "installed-a")
	c.apply(t, "add-b", "b", true)
	c.catalog(t, "installed-b")
	c.catalog(t, "installed-outside")
	c.apply(t, "repeat-add-a", "a", false)
	c.apply(t, "unchanged-update-a", "a", false)
	c.apply(t, "intact-repair-a", "a", true)
	c.catalog(t, "stable-a")
	c.reviseA(t)
	c.apply(t, "update-a", "a", true)
	for _, stage := range []string{"updated-a", "updated-b", "updated-outside"} {
		c.catalog(t, stage)
	}
	c.missingA(t)
	c.catalog(t, "damaged-a")
	c.apply(t, "missing-repair-a", "a", true)
	c.catalog(t, "repaired-a")
	c.apply(t, "remove-a", "a", true)
	for _, stage := range []string{"removed-a", "remaining-b", "removed-outside"} {
		c.catalog(t, stage)
	}
	c.apply(t, "repeat-remove-a", "a", false)
	c.catalog(t, "repeat-removed-a")
	c.apply(t, "remove-b", "b", true)
	for _, stage := range []string{"removed-b", "final-a", "final-b", "final-outside"} {
		c.catalog(t, stage)
	}
	if !reflect.DeepEqual(c.public, qoderCyclePublic) || !reflect.DeepEqual(c.catalogs, qoderCycleCatalog) || len(c.logs.runs) != 21 {
		t.Fatal("Qoder full public/catalog/log-run closure")
	}
	c.check(t)
	c.append(t, qoderCycleAggregates[2], "log-ledger", map[string]any{"original_sha256": qoderCycleInventoryHash(c.logs.original), "expected_sha256": qoderCycleInventoryHash(c.logs.expected), "runs": append([]string{}, c.logs.runs...)})
	c.complete = true
	c.f.evidence["cycle_stage"] = "complete-cycle"
}

func (c *qoderSkillsCycle) mutableAuthority() error {
	for key, r := range c.recipes {
		if key == c.mutable {
			continue
		}
		if r.Installation.InstallationID == "" {
			if err := qoderCycleAbsent(c.statePath(r)); err != nil {
				return fmt.Errorf("Qoder uninstalled other project state appeared")
			}
			continue
		}
		if r.Removed || r.Missing {
			if err := qoderCycleAbsent(r.Active); err != nil {
				return err
			}
		}
		if !r.Removed {
			physical := qoderCycleName + "-" + qoderDeltaHash(r.Installation.InstallationID)[:12]
			if err := qoderCycleAbsent(filepath.Join(r.Project, ".uap", "plugin-data", physical)); err != nil {
				return fmt.Errorf("Qoder installed skills-only data appeared")
			}
		}
	}
	return qoderCycleAbsent(c.f.base.Events)
}
