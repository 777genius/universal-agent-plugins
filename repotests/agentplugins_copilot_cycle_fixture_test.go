//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const copilotCycleMode = "copilot-project-complete-cycle"

var copilotCyclePublic = []string{"git-a", "git-b", "version", "add-user", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b", "repeat-remove-b", "remove-user", "repeat-remove-user"}
var copilotCycleRegistry = []string{"empty-user", "add-user", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b", "repeat-remove-b", "remove-user", "repeat-remove-user"}
var copilotCycleNative = []string{"installed-user", "empty-a", "empty-b", "installed-a", "installed-b", "stable-a", "updated-a", "repaired-a", "removed-a", "remaining-b", "removed-b", "remaining-user", "final-a", "final-b"}
var copilotCycleAggregates = []string{"cycle-native-events.jsonl", "cycle-native-commands.jsonl", "cycle-registry-listings.jsonl", "cycle-provider-requests.jsonl", "cycle-provider-responses.jsonl", "cycle-provider-receipts.jsonl", "cycle-probe-events.jsonl", "cycle-proof.jsonl"}

type copilotCycleRecipe struct {
	Key, Project, Source, Skill, Revision, Version, Nonce, Digest string
	Authored                                                      map[string][]byte
	Installation                                                  domain.Installation
	Binding                                                       domain.ClientBinding
	Active, Data, SkillPath, Config                               string
	Removed, Written                                              bool
}

type copilotCycleBytes struct {
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
	Body   []byte `json:"body"`
}

type copilotProjectCycle struct {
	f                          *copilotProjectAttempt
	recipes                    map[string]*copilotCycleRecipe
	aggregates                 map[string][]byte
	trees                      map[string]map[string]string
	held                       map[string]os.FileInfo
	absent                     map[string]bool
	public, registry, sessions []string
	nativeIDs, probeIDs        map[string]bool
	complete                   bool
}

func newCopilotProjectCycle(f *copilotProjectAttempt) *copilotProjectCycle {
	f.evidence["mode"] = copilotCycleMode
	return &copilotProjectCycle{f: f, recipes: map[string]*copilotCycleRecipe{}, aggregates: map[string][]byte{},
		trees: map[string]map[string]string{}, held: map[string]os.FileInfo{}, absent: map[string]bool{}, nativeIDs: map[string]bool{}, probeIDs: map[string]bool{},
		public: []string{}, registry: []string{}, sessions: []string{}}
}

func copilotCycleStream(t *testing.T, raw []byte) copilotCycleBytes {
	copilotCycleSafe(t, raw)
	return copilotCycleBytes{Bytes: len(raw), SHA256: copilotProjectConfigHash(raw), Body: append([]byte{}, raw...)}
}

func (c *copilotProjectCycle) append(t *testing.T, member, stage string, value any) {
	t.Helper()
	if !slices.Contains(copilotCycleAggregates, member) || stage == "" {
		t.Fatal("unregistered Copilot cycle aggregate/stage")
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
	for _, body := range c.f.files {
		total += len(body)
	}
	if len(c.aggregates[member])+len(raw) > 1<<20 || total > 16<<20 {
		t.Fatal("Copilot cycle live aggregate bound")
	}
	c.aggregates[member] = append(c.aggregates[member], raw...)
}

func (c *copilotProjectCycle) statePath(r *copilotCycleRecipe) string {
	if r.Key == "user" {
		return filepath.Join(c.f.base.Root, "installer-state", "state-v2.json")
	}
	return filepath.Join(r.Project, ".uap", "state-v2.json")
}

func copilotCycleScope(r *copilotCycleRecipe) string {
	if r.Key == "user" {
		return "user"
	}
	return "project"
}

func (c *copilotProjectCycle) check(t *testing.T) {
	t.Helper()
	if c.f.ctx.Err() != nil {
		t.Fatal("Copilot cycle original work deadline exhausted")
	}
	if err := c.authority(); err != nil {
		t.Fatal(err)
	}
}

func (c *copilotProjectCycle) authority() error {
	if err := c.f.readinessAuthority(); err != nil {
		return err
	}
	for path := range c.absent {
		if err := copilotCycleAbsent(path); err != nil {
			return fmt.Errorf("Copilot absent owned path appeared")
		}
	}
	for root, expected := range c.trees {
		actual, err := clineAttemptTree(root)
		if err != nil || !reflect.DeepEqual(actual, expected) {
			return fmt.Errorf("Copilot original or unaffected tree changed")
		}
	}
	for path, before := range c.held {
		probe := path
		if before.IsDir() {
			probe = filepath.Join(path, "leaf")
		}
		if err := projectNativeRuntimeParents(probe); err != nil {
			return err
		}
		after, err := os.Lstat(path)
		if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Sys().(*syscall.Stat_t).Uid != after.Sys().(*syscall.Stat_t).Uid || before.Sys().(*syscall.Stat_t).Gid != after.Sys().(*syscall.Stat_t).Gid {
			return fmt.Errorf("Copilot original foreign identity changed")
		}
	}
	return nil
}

func (c *copilotProjectCycle) author(t *testing.T, r *copilotCycleRecipe, replace bool) {
	t.Helper()
	manifest, _ := json.Marshal(map[string]any{"$schema": domain.PluginSchemaV1, "name": "native-proof", "version": r.Version, "description": "Disposable Copilot cycle"})
	skill := []byte(fmt.Sprintf("---\nname: %s\ndescription: Disposable Copilot %s %s %s\n---\nUAP_COPILOT_%s_%s\n", r.Skill, r.Key, r.Revision, r.Nonce, r.Revision, r.Nonce))
	authored := map[string][]byte{"plugin.json": manifest, "skills/" + r.Skill + "/SKILL.md": skill}
	if r.Key != "user" {
		probe, err := projectNativeRuntimeRead(c.f.base.Probe, 32<<20)
		if err != nil {
			t.Fatal(err)
		}
		mcp, _ := json.Marshal(map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{r.Skill: map[string]any{
			"type": "stdio", "command": "./bin/probe", "args": []string{"argument with spaces", "${PLUGIN_ROOT}/argument", "${UNKNOWN}"}, "cwd": "${PLUGIN_ROOT}",
			"env": map[string]string{"UAP_TEST_ROOT": c.f.base.Root, "UAP_TEST_EVENTS": c.f.base.Events, "UAP_TEST_NONCE": r.Nonce, "UAP_TEST_REVISION": r.Revision,
				"UAP_TEST_LITERAL": "literal ${UNKNOWN} $HOME", "UAP_TEST_ONCE": "${PLUGIN_ROOT}/${UNKNOWN}"}}}})
		authored["mcp.json"], authored["bin/probe"] = mcp, probe
	}
	for relative, body := range authored {
		mode := os.FileMode(0600)
		if relative == "bin/probe" {
			mode = 0700
		}
		path := filepath.Join(r.Source, filepath.FromSlash(relative))
		if replace {
			if _, err := projectNativeRuntimeRead(path, 32<<20); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, body, mode); err != nil {
				t.Fatal(err)
			}
		} else {
			projectNativeRuntimeWrite(t, path, body, mode)
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
	tree, err := clineAttemptTree(r.Source)
	if err != nil {
		t.Fatal(err)
	}
	c.trees[r.Source] = tree
	c.append(t, "cycle-proof.jsonl", "source-"+r.Key+"-"+r.Revision, map[string]any{"source": r.Source, "digest": r.Digest, "files": tree})
}
