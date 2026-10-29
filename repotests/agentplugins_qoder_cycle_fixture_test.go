//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const qoderCycleMode = "qoder-complete-skills-cycle"
const qoderCycleName = "uap-qoder-cycle"
const qoderCycleDescription = "Disposable Qoder project cycle"

var qoderCyclePublic = []string{"git-a", "git-b", "git-outside", "version", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b"}
var qoderCycleCatalog = []string{"empty-outside", "empty-a", "empty-b", "installed-a", "installed-b", "installed-outside", "stable-a", "updated-a", "updated-b", "updated-outside", "damaged-a", "repaired-a", "removed-a", "remaining-b", "removed-outside", "repeat-removed-a", "removed-b", "final-a", "final-b", "final-outside"}
var qoderCycleAggregates = []string{"qoder-cycle-public.jsonl", "qoder-cycle-catalog.jsonl", "qoder-cycle-proof.jsonl"}

type qoderCycleBytes struct {
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
	Body   []byte `json:"body"`
}

type qoderCycleRecipe struct {
	Key, Project, Source, Revision, Nonce, Digest string
	Authored                                      map[string][]byte
	Installation                                  domain.Installation
	Binding                                       domain.ClientBinding
	Active, View                                  string
	Removed, Missing                              bool
}

type qoderSkillsCycle struct {
	f                *projectNativeCLIAttempt
	recipes          map[string]*qoderCycleRecipe
	aggregates       map[string][]byte
	original         map[string]os.FileInfo
	trees            map[string]map[string]string
	profileRecipe    map[string]string
	groups           map[string]bool
	logs             *qoderCycleLogs
	public, catalogs []string
	complete         bool
	mutable          string
}

func newQoderSkillsCycle(f *projectNativeCLIAttempt) *qoderSkillsCycle {
	f.name = qoderCycleName
	f.evidence["capture_mode"] = qoderCycleMode
	f.evidence["cycle_stage"] = "prepare"
	return &qoderSkillsCycle{f: f, recipes: map[string]*qoderCycleRecipe{}, aggregates: map[string][]byte{}, original: map[string]os.FileInfo{}, trees: map[string]map[string]string{}, profileRecipe: map[string]string{".": "directory:20000000700"}, groups: map[string]bool{}, public: []string{}, catalogs: []string{}}
}

func qoderCyclePrivate(raw []byte) bool {
	marker := []byte("uap-qoder-private-")
	if len(raw) > 1<<20 || !utf8.Valid(raw) || bytes.Contains(raw, marker) {
		return true
	}
	budget := clineCyclePrivacyBudget{text: 4 * len(raw), nodes: 16384}
	return clineCycleCredentialJSON(raw, marker, 0, &budget)
}

func qoderCycleStream(t *testing.T, raw []byte) qoderCycleBytes {
	t.Helper()
	if qoderCyclePrivate(raw) {
		t.Fatal("Qoder cycle private/unbounded stream refused before capture")
	}
	return qoderCycleBytes{Bytes: len(raw), SHA256: qoderDeltaHash(string(raw)), Body: append([]byte{}, raw...)}
}

func (c *qoderSkillsCycle) append(t *testing.T, member, stage string, value any) {
	t.Helper()
	known := false
	for _, name := range qoderCycleAggregates {
		if member == name {
			known = true
		}
	}
	if !known || stage == "" {
		t.Fatal("Qoder cycle unregistered aggregate/stage")
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
		t.Fatal("Qoder cycle aggregate bound")
	}
	c.aggregates[member] = append(c.aggregates[member], raw...)
}

func (c *qoderSkillsCycle) statePath(r *qoderCycleRecipe) string {
	return filepath.Join(r.Project, ".uap", "state-v2.json")
}

func (c *qoderSkillsCycle) hold(t *testing.T, path string) {
	t.Helper()
	if err := projectNativeRuntimeParents(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
		t.Fatal("Qoder original ordinary identity required")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) || stat.Gid != uint32(os.Getgid()) || info.Mode().IsRegular() && stat.Nlink != 1 {
		t.Fatal("Qoder original owner/link identity")
	}
	c.original[path] = info
}

func (c *qoderSkillsCycle) check(t *testing.T) {
	t.Helper()
	if c.f.ctx.Err() != nil {
		t.Fatal("Qoder original work deadline exhausted")
	}
	if err := c.authority(); err != nil {
		t.Fatal(err)
	}
}

func (c *qoderSkillsCycle) authority() error {
	if c.f.qoderMetadata != nil {
		if err := c.f.qoderMetadata.check(); err != nil {
			return err
		}
	}
	if c.f.qoderTelemetry != nil {
		if err := c.f.qoderTelemetry.check(); err != nil {
			return err
		}
	}
	for _, inventory := range []map[string]string{c.f.immutable, c.f.protected} {
		for path, want := range inventory {
			got, err := projectNativeRuntimeDigest(path)
			if err != nil || got != want {
				return fmt.Errorf("Qoder final immutable/original protected identity changed")
			}
		}
	}
	if c.f.bin != "" {
		path := filepath.Join(c.f.bin, "qoder")
		info, err := os.Lstat(path)
		target, linkErr := os.Readlink(path)
		if err != nil || linkErr != nil || info.Mode()&os.ModeSymlink == 0 || target != c.f.client {
			return fmt.Errorf("Qoder admitted executable alias changed")
		}
	}
	if c.f.base != nil {
		if err := c.mutableAuthority(); err != nil {
			return err
		}
	}
	if c.logs != nil {
		if err := c.logs.check(); err != nil {
			return err
		}
	}
	for path, before := range c.original {
		if err := projectNativeRuntimeParents(path); err != nil {
			return err
		}
		after, err := os.Lstat(path)
		if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
			return fmt.Errorf("Qoder original identity/mode changed")
		}
		a, b := before.Sys().(*syscall.Stat_t), after.Sys().(*syscall.Stat_t)
		if a.Uid != b.Uid || a.Gid != b.Gid || before.Mode().IsRegular() && b.Nlink != 1 {
			return fmt.Errorf("Qoder original owner/link changed")
		}
	}
	for root, want := range c.trees {
		actual, err := clineAttemptTree(root)
		if err != nil || !reflect.DeepEqual(actual, want) {
			return fmt.Errorf("Qoder original/unaffected tree changed")
		}
	}
	return nil
}

func (c *qoderSkillsCycle) prepare(t *testing.T) {
	t.Helper()
	f := c.f
	layout := map[string]string{"root": f.base.Root, "home": f.base.Home, "a": f.base.Project, "b": f.other, "outside": f.outside, "installer": f.installer, "client": f.client, "git": f.git}
	f.evidence["nonce"], f.evidence["layout"] = f.base.Nonce, layout
	c.append(t, "qoder-cycle-proof.jsonl", "fixture", map[string]any{"nonce": f.base.Nonce, "layout": layout, "owner_uid": os.Getuid(), "owner_gid": os.Getgid()})
	for _, key := range []string{"a", "b"} {
		project := f.base.Project
		if key == "b" {
			project = f.other
		}
		r := &qoderCycleRecipe{Key: key, Project: project, Source: filepath.Join(f.base.Root, "cycle-source-"+key), Revision: key + "-one", Nonce: qoderDeltaHash(f.base.Nonce + key)[:32]}
		c.recipes[key] = r
		c.author(t, r, false)
		path := filepath.Join(project, ".uap", "plugin-data", "caller-owned", "keep.txt")
		body := []byte("CALLER_" + key + "_" + f.base.Nonce + "\n")
		projectNativeRuntimeWrite(t, path, body, 0600)
		c.hold(t, path)
		c.hold(t, filepath.Dir(path))
		f.protected[path] = qoderDeltaHash(string(body))
		inventory, err := clineAttemptTree(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		c.trees[filepath.Dir(path)] = inventory
		c.append(t, "qoder-cycle-proof.jsonl", "caller-"+key, map[string]any{"path": path, "body": qoderCycleStream(t, body), "files": inventory})
	}
	for i, item := range qoderMetadataRecipe {
		c.profileRecipe[item.name] = fmt.Sprintf("file:%o:%d:%s", item.mode, item.size, f.qoderMetadata.files[i].digest)
	}
	for _, path := range []string{".auth", "tmp", "tmp/telemetry", "tmp/telemetry/active-runs"} {
		c.profileRecipe[path] = "directory:20000000700"
	}
	c.controls(t)
	logs, err := newQoderCycleLogs(f.ctx, filepath.Join(f.base.Home, ".qoder"), c.profileRecipe)
	if err != nil {
		t.Fatal(err)
	}
	c.logs = logs
	c.append(t, "qoder-cycle-proof.jsonl", "original-profile", map[string]any{"logs_absent": true, "inventory": logs.original})
	for relative := range logs.original {
		c.hold(t, filepath.Join(logs.root, relative))
	}
	c.check(t)
}

func (c *qoderSkillsCycle) author(t *testing.T, r *qoderCycleRecipe, replace bool) {
	t.Helper()
	authored := qoderCycleAuthored(r.Key, r.Revision, r.Nonce)
	for relative, body := range authored {
		path := filepath.Join(r.Source, filepath.FromSlash(relative))
		if replace {
			if _, err := projectNativeRuntimeRead(path, 1<<20); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
		} else {
			projectNativeRuntimeWrite(t, path, body, 0600)
		}
	}
	r.Authored = authored
	entries, err := clineAttemptPayload(r.Source)
	if err != nil {
		t.Fatal(err)
	}
	r.Digest, err = packagedigest.DigestCaptured(c.f.ctx, entries)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := clineAttemptTree(r.Source)
	if err != nil {
		t.Fatal(err)
	}
	c.trees[r.Source] = inventory
	c.append(t, "qoder-cycle-proof.jsonl", "source-"+r.Key+"-"+r.Revision, map[string]any{"source": r.Source, "digest": r.Digest, "files": inventory})
}

func qoderCycleAuthored(key, revision, nonce string) map[string][]byte {
	manifest, _ := json.Marshal(map[string]any{"$schema": domain.PluginSchemaV1, "name": qoderCycleName, "version": "1.0.0", "description": qoderCycleDescription})
	skill := []byte(fmt.Sprintf("---\nname: %s\ndescription: Disposable Qoder %s %s %s\n---\nUAP_QODER_%s_%s\n", qoderCycleName, key, revision, nonce, revision, nonce))
	return map[string][]byte{"plugin.json": manifest, "skills/" + qoderCycleName + "/SKILL.md": skill}
}

func qoderCycleControlName(key string) string {
	if key == "user" {
		return "uap-qoder-original-user"
	}
	return "uap-qoder-foreign-" + key
}

func (c *qoderSkillsCycle) controls(t *testing.T) {
	t.Helper()
	f := c.f
	registry := map[string]any{}
	enabled := map[string]bool{}
	for _, key := range []string{"user", "a", "b"} {
		name := qoderCycleControlName(key)
		parent := f.base.Home
		switch key {
		case "a":
			parent = f.base.Project
		case "b":
			parent = f.other
		}
		root := filepath.Join(parent, ".qoder", "plugins", name)
		description := "Original Qoder " + key + " " + f.base.Nonce
		manifest, _ := json.Marshal(map[string]any{"name": name, "version": "0.9.0", "description": description, "skills": "./skills/"})
		skill := []byte(fmt.Sprintf("---\nname: %s\ndescription: %s\n---\nUAP_QODER_ORIGINAL_%s_%s\n", name, description, strings.ToUpper(key), f.base.Nonce))
		for relative, body := range map[string][]byte{".qoder-plugin/plugin.json": manifest, "skills/" + name + "/SKILL.md": skill} {
			path := filepath.Join(root, relative)
			projectNativeRuntimeWrite(t, path, body, 0600)
			if key == "user" {
				c.profileFile(t, path, body)
			}
		}
		c.hold(t, filepath.Dir(root))
		c.hold(t, filepath.Dir(filepath.Dir(root)))
		if err := filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			c.hold(t, path)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		inventory, err := clineAttemptTree(root)
		if err != nil {
			t.Fatal(err)
		}
		c.trees[root] = inventory
		entry := map[string]any{"scope": "user", "installPath": root, "version": "0.9.0"}
		if key != "user" {
			entry["scope"], entry["projectPath"] = "project", parent
		}
		registry[name+"@local"] = []any{entry}
		enabled[name+"@local"] = false
		c.append(t, "qoder-cycle-proof.jsonl", "original-"+key, map[string]any{"key": key, "path": root, "manifest": qoderCycleStream(t, manifest), "skill": qoderCycleStream(t, skill), "files": inventory, "entry": entry})
	}
	profile := filepath.Join(f.base.Home, ".qoder")
	settings := filepath.Join(profile, "settings.json")
	f.check(t)
	body, _ := json.Marshal(map[string]any{"general": map[string]any{"enableAutoUpdate": false}, "enabledPlugins": enabled})
	if err := os.WriteFile(settings, body, 0600); err != nil {
		t.Fatal(err)
	}
	f.protected[settings] = qoderDeltaHash(string(body))
	c.profileFile(t, settings, body)
	body, _ = json.Marshal(map[string]any{"version": 2, "plugins": registry})
	path := filepath.Join(profile, "plugins", "installed_plugins_v2.json")
	projectNativeRuntimeWrite(t, path, body, 0600)
	f.protected[path] = qoderDeltaHash(string(body))
	c.profileFile(t, path, body)
	c.append(t, "qoder-cycle-proof.jsonl", "original-registry", map[string]any{"path": path, "registry": qoderCycleStream(t, body), "settings_path": settings, "settings": qoderCycleStream(t, mustQoderCycleRead(t, settings))})
}

func mustQoderCycleRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := projectNativeRuntimeRead(path, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (c *qoderSkillsCycle) profileFile(t *testing.T, path string, body []byte) {
	t.Helper()
	relative, err := filepath.Rel(filepath.Join(c.f.base.Home, ".qoder"), path)
	if err != nil || relative == "." || strings.HasPrefix(relative, "..") {
		t.Fatal("Qoder authored profile recipe escaped")
	}
	c.profileRecipe[relative] = fmt.Sprintf("file:600:%d:%s", len(body), qoderDeltaHash(string(body)))
	for parent := filepath.Dir(relative); parent != "."; parent = filepath.Dir(parent) {
		c.profileRecipe[parent] = "directory:20000000700"
	}
}
