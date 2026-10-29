//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var grokCyclePublic = []string{"git-a", "git-b", "git-outside", "version", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b"}
var grokCycleInspections = []string{"empty-outside", "empty-a", "empty-b", "installed-a", "installed-b", "installed-outside", "stable-a", "updated-a", "updated-b", "updated-outside", "damaged-a", "repaired-a", "removed-a", "remaining-b", "removed-outside", "repeat-removed-a", "removed-b", "final-a", "final-b", "final-outside"}
var grokCycleProofs = []string{"source-a-a-one", "source-b-b-one", "fixture", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "before-source-revision-a", "source-a-a-two", "update-a", "before-missing-a", "damaged-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b", "final"}
var grokCycleMembers = []string{"grok-cycle-public.jsonl", "grok-cycle-inspect.jsonl", "grok-cycle-proof.jsonl"}

type grokCycleRecipe struct {
	Scope, Project, Source, Name, Revision, Digest string
	Authored                                       map[string][]byte
	Installation                                   domain.Installation
	Binding                                        domain.ClientBinding
	Active, Data                                   string
	Removed                                        bool
}

type grokCycleFile struct {
	Kind     string           `json:"kind"`
	Mode     uint32           `json:"mode"`
	UID      uint32           `json:"uid"`
	GID      uint32           `json:"gid"`
	Device   uint64           `json:"device"`
	Inode    uint64           `json:"inode"`
	Modified int64            `json:"modified_ns"`
	Changed  int64            `json:"changed_ns"`
	Content  *clineCycleBytes `json:"content,omitempty"`
}

type grokProjectCycle struct {
	f                   *projectNativeCLIAttempt
	recipes             map[string]*grokCycleRecipe
	aggregates          map[string][]byte
	guards, fixed       map[string]map[string]grokCycleFile
	absent              map[string]bool
	public, inspections []string
	proofs              []string
	complete            bool
}

func grokCycleStream(raw []byte) (clineCycleBytes, error) {
	if grokProjectPrivateCredential(raw) {
		return clineCycleBytes{}, fmt.Errorf("Grok cycle private raw stream refused before encoding")
	}
	return clineCycleBytes{Bytes: len(raw), SHA256: clineAttemptSHA(raw), Body: append([]byte{}, raw...)}, nil
}

func (c *grokProjectCycle) stream(t *testing.T, raw []byte) clineCycleBytes {
	t.Helper()
	value, err := grokCycleStream(raw)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func grokCycleSnapshot(root string) (map[string]grokCycleFile, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || projectNativeRuntimeParents(root) != nil {
		return nil, fmt.Errorf("Grok cycle snapshot root authority differs")
	}
	result, total := map[string]grokCycleFile{}, 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || len(result) >= 128 {
			return fmt.Errorf("Grok cycle finite snapshot bound")
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uint32(os.Getuid()) || info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("Grok cycle held path is not ordinary originally owned")
		}
		value := grokCycleFile{Kind: "directory", Mode: uint32(info.Mode().Perm()), UID: stat.Uid, GID: stat.Gid, Device: stat.Dev, Inode: stat.Ino}
		if entry.IsDir() {
			if err := projectNativeRuntimeParents(filepath.Join(path, "leaf")); err != nil {
				return err
			}
		} else {
			body, err := projectNativeRuntimeRead(path, 1<<20)
			if err != nil {
				return err
			}
			total += len(body)
			after, err := os.Lstat(path)
			if err != nil || total > 1<<20 || !os.SameFile(info, after) || !projectNativeRuntimeStableStat(info.Sys(), after.Sys()) {
				return fmt.Errorf("Grok cycle held file changed during bounded capture")
			}
			stream, err := grokCycleStream(body)
			if err != nil {
				return err
			}
			value.Kind, value.Content = "file", &stream
			value.Modified, value.Changed = stat.Mtim.Nano(), stat.Ctim.Nano()
		}
		result[filepath.ToSlash(relative)] = value
		return nil
	})
	return result, err
}

func (c *grokProjectCycle) freeze(t *testing.T, path string, fixed bool) {
	t.Helper()
	value, err := grokCycleSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	c.guards[path] = value
	if fixed {
		c.fixed[path] = value
	}
}

func (c *grokProjectCycle) check(t *testing.T) {
	t.Helper()
	c.f.check(t)
	for path, expected := range c.guards {
		actual, err := grokCycleSnapshot(path)
		if err != nil || !reflect.DeepEqual(actual, expected) {
			t.Fatalf("Grok cycle original held authority changed: %s (%v)", path, err)
		}
	}
	for path := range c.absent {
		if err := projectNativeRuntimeParents(path); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("Grok cycle independently absent path appeared")
		}
	}
}

func (c *grokProjectCycle) append(t *testing.T, member, stage string, value any) {
	t.Helper()
	known := false
	for _, name := range grokCycleMembers {
		known = known || name == member
	}
	if !known {
		t.Fatal("unregistered Grok aggregate")
	}
	raw, err := json.Marshal(map[string]any{"stage": stage, "value": value})
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	total := len(raw)
	for _, body := range c.aggregates {
		total += len(body)
	}
	if len(c.aggregates[member])+len(raw) > 1<<20 || total > 16<<20 {
		t.Fatal("Grok cycle live aggregate bound exceeded")
	}
	c.aggregates[member] = append(c.aggregates[member], raw...)
}

func (c *grokProjectCycle) author(t *testing.T, r *grokCycleRecipe, replace bool) {
	t.Helper()
	r.Authored = grokCycleAuthored(r.Name, r.Revision, c.f.base.Nonce)
	for relative, body := range r.Authored {
		path := filepath.Join(r.Source, filepath.FromSlash(relative))
		if replace {
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
		} else {
			projectNativeRuntimeWrite(t, path, body, 0600)
		}
	}
	entries, err := clineAttemptPayload(r.Source)
	if err != nil {
		t.Fatal(err)
	}
	r.Digest, err = packagedigest.DigestCaptured(c.f.ctx, entries)
	if err != nil {
		t.Fatal(err)
	}
	c.freeze(t, r.Source, false)
	c.append(t, "grok-cycle-proof.jsonl", "source-"+r.Scope+"-"+r.Revision, map[string]any{"scope": r.Scope, "source": r.Source, "revision": r.Revision, "digest": r.Digest, "files": c.guards[r.Source]})
	c.proofs = append(c.proofs, "source-"+r.Scope+"-"+r.Revision)
}

func grokCycleAuthored(name, revision, nonce string) map[string][]byte {
	manifest, _ := json.Marshal(map[string]any{"$schema": domain.PluginSchemaV1, "name": name, "version": "1.0.0", "description": "Disposable Grok skills cycle"})
	skill := []byte("---\nname: " + name + "\ndescription: " + revision + " " + nonce + "\n---\nUAP_GROK_" + revision + "_" + nonce + "\n")
	return map[string][]byte{"plugin.json": manifest, "skills/" + name + "/SKILL.md": skill}
}

func (c *grokProjectCycle) prepare(t *testing.T) {
	t.Helper()
	f := c.f
	if f.spec.client != "grok" || f.base.Project != filepath.Join(f.base.Root, "project") || f.other != filepath.Join(f.base.Root, "project-b") || f.outside != filepath.Join(f.base.Root, "outside") || f.base.Home != filepath.Join(f.base.Root, "home") {
		t.Fatal("Grok cycle original sibling roots differ")
	}
	for _, path := range []string{f.base.Root, f.base.Project, f.other, f.outside, f.base.Home} {
		if err := projectNativeRuntimeParents(filepath.Join(path, "leaf")); err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Sys().(*syscall.Stat_t).Uid != uint32(os.Getuid()) {
			t.Fatal("Grok cycle roots must be original ordinary owned directories")
		}
	}
	for _, scope := range []string{"a", "b"} {
		project := f.base.Project
		if scope == "b" {
			project = f.other
		}
		r := &grokCycleRecipe{Scope: scope, Project: project, Source: filepath.Join(f.base.Root, "cycle-source-"+scope), Name: "uap-grok-cycle-" + scope, Revision: scope + "-one"}
		c.recipes[scope] = r
		c.author(t, r, false)
		name := "uap-grok-foreign-" + scope
		path := filepath.Join(project, ".grok", "plugins", name)
		c.seedPlugin(t, path, name, "Foreign inert marker", "FOREIGN_"+scope+"_"+f.base.Nonce)
		caller := filepath.Join(project, ".uap", "plugin-data", "caller-owned")
		projectNativeRuntimeWrite(t, filepath.Join(caller, "keep.txt"), []byte("CALLER_"+scope+"_"+f.base.Nonce+"\n"), 0600)
		c.freeze(t, caller, true)
	}
	name := "uap-grok-original-user"
	c.seedPlugin(t, filepath.Join(f.base.Home, ".grok", "plugins", name), name, "Original user "+f.base.Nonce, "USER_"+f.base.Nonce)
	var trust bytes.Buffer
	for _, root := range []string{f.base.Project, f.other} {
		quoted, _ := json.Marshal(root)
		trust.WriteString("[folders." + string(quoted) + "]\ntrusted = true\n")
	}
	config := []byte("[marketplace]\ndefault_skills_installs_purged = true\n[plugins]\nenabled = [\"uap-grok-cycle-a\", \"uap-grok-cycle-b\", \"uap-grok-original-user\"]\ndisabled = [\"uap-grok-foreign-a\", \"uap-grok-foreign-b\"]\n[features]\nturn_summary = false\n")
	for name, body := range map[string][]byte{"config.toml": config, "trusted_folders.toml": trust.Bytes()} {
		path := filepath.Join(f.base.Home, ".grok", name)
		projectNativeRuntimeWrite(t, path, body, 0600)
		f.protected[path] = nativeSHA(t, path)
		c.freeze(t, path, true)
	}
	c.append(t, "grok-cycle-proof.jsonl", "fixture", map[string]any{"root": f.base.Root, "home": f.base.Home, "project_a": f.base.Project, "project_b": f.other, "outside": f.outside, "nonce": f.base.Nonce, "fixed": c.fixed})
	c.proofs = append(c.proofs, "fixture")
	c.check(t)
}

func (c *grokProjectCycle) seedPlugin(t *testing.T, path, name, description, body string) {
	t.Helper()
	manifest, _ := json.Marshal(map[string]any{"name": name, "version": "0.9.0", "skills": "./skills/"})
	projectNativeRuntimeWrite(t, filepath.Join(path, ".grok-plugin", "plugin.json"), manifest, 0600)
	projectNativeRuntimeWrite(t, filepath.Join(path, "skills", name, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: "+description+"\n---\n"+body+"\n"), 0600)
	c.freeze(t, path, true)
}
