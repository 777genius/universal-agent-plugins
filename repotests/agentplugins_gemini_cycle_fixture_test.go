//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const geminiCycleName = "uap-gemini-cycle"
const geminiCycleMode = "gemini-complete-cycle-first-capture"

var geminiCyclePublic = []string{"git-a", "git-b", "git-user", "version", "add-user", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b", "remove-user"}
var geminiCycleListings = []string{"baseline-a", "baseline-b", "baseline-user", "installed-user", "installed-a", "installed-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "updated-a", "repaired-a", "removed-a", "removed-b", "removed-user"}
var geminiCycleNative = []string{"baseline-a", "installed-user", "installed-a", "installed-b", "restarted-a", "updated-a", "repaired-a", "removed-a", "removed-b", "removed-user"}
var geminiCycleAggregates = []string{"cycle-native-commands.jsonl", "cycle-native-events.jsonl", "cycle-discovery.jsonl", "cycle-provider-requests.jsonl", "cycle-provider-responses.jsonl", "cycle-provider-receipts.jsonl", "cycle-probe-events.jsonl", "cycle-proof.jsonl"}

type geminiCycleRecipe struct {
	Scope, Project, Source, Name, Revision, Nonce, Digest, StateRoot, Profile, Config, SkillPath, Active, Data string
	Authored                                                                                                   map[string][]byte
	Installation                                                                                               domain.Installation
	Binding                                                                                                    domain.ClientBinding
	Removed                                                                                                    bool
}

func (r *geminiCycleRecipe) scopeName() string {
	if r.Scope == "user" {
		return "user"
	}
	return "project"
}

type geminiCycleBytes struct {
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
	Body   []byte `json:"body"`
}
type geminiProjectCycle struct {
	f                         *geminiProjectAttempt
	recipes                   map[string]*geminiCycleRecipe
	frozen                    map[string]map[string]geminiCycleRecord
	fileFrames                map[string]geminiCycleFileFrame
	absent                    map[string]bool
	open                      map[string]bool
	aggregates                map[string][]byte
	public, listings, natives []string
	sessions, probeSessions   map[string]bool
	probePIDs                 map[int64]bool
	settings                  map[string]map[string]any
	profileBefore             map[string]map[string]geminiCycleRecord
	owner                     *geminiCycleProvider
	stage                     string
	observed                  bool
	eventsAccepted            []byte
	openEvents                bool
}

func newGeminiProjectCycle(f *geminiProjectAttempt) *geminiProjectCycle {
	for _, name := range geminiCycleAggregates {
		f.files[name] = []byte{}
	}
	f.evidence["capture_mode"] = geminiCycleMode
	f.evidence["cycle_capture_review_required"] = true
	f.evidence["cycle_observed_complete"] = false
	f.evidence["cycle_layout"] = map[string]string{"root": f.base.Root, "home": f.base.Home, "a": f.base.Project, "b": f.other, "outside": f.outside, "installer": f.installer, "client": f.client, "node": f.node, "git": f.git}
	return &geminiProjectCycle{f: f, recipes: map[string]*geminiCycleRecipe{}, frozen: map[string]map[string]geminiCycleRecord{}, fileFrames: map[string]geminiCycleFileFrame{}, absent: map[string]bool{}, open: map[string]bool{}, aggregates: map[string][]byte{}, public: []string{}, listings: []string{}, natives: []string{}, sessions: map[string]bool{}, probeSessions: map[string]bool{}, probePIDs: map[int64]bool{}, settings: map[string]map[string]any{}, profileBefore: map[string]map[string]geminiCycleRecord{}, stage: "prepare"}
}
func (c *geminiProjectCycle) stream(t *testing.T, raw []byte) geminiCycleBytes {
	t.Helper()
	raw = bytes.ReplaceAll(raw, []byte(c.f.secret), []byte("[redacted]"))
	if geminiCyclePrivateCredential(raw) {
		t.Fatal("Gemini cycle private/resource refusal before encoding")
	}
	return geminiCycleBytes{len(raw), geminiCycleSHA(raw), append([]byte{}, raw...)}
}
func (c *geminiProjectCycle) append(t *testing.T, member, stage string, value any) {
	t.Helper()
	known := false
	for _, name := range geminiCycleAggregates {
		known = known || member == name
	}
	if !known || stage == "" {
		t.Fatal("Gemini cycle registered aggregate/stage required")
	}
	raw, err := json.Marshal(map[string]any{"stage": stage, "value": value})
	if err != nil {
		t.Fatal("Gemini cycle serialization")
	}
	raw = append(raw, '\n')
	total := len(raw)
	for _, body := range c.f.files {
		total += len(body)
	}
	if len(c.aggregates[member])+len(raw) > 1<<20 || total > 16<<20 || geminiCyclePrivateCredential(raw) {
		t.Fatal("Gemini cycle aggregate bound/privacy")
	}
	c.aggregates[member] = append(c.aggregates[member], raw...)
	c.f.files[member] = bytes.Clone(c.aggregates[member])
}
func (c *geminiProjectCycle) freeze(t *testing.T, path string) {
	t.Helper()
	tree, err := geminiCycleTree(c.f.ctx, path)
	if err != nil {
		t.Fatal("Gemini cycle ordinary owned tree", err)
	}
	c.frozen[path] = tree
}
func (c *geminiProjectCycle) check(t *testing.T) {
	t.Helper()
	c.f.check(t)
	if err := c.originalFiles(); err != nil {
		t.Fatal(err)
	}
	for path, want := range c.frozen {
		if c.open[path] {
			continue
		}
		actual, err := geminiCycleTree(c.f.ctx, path)
		if err != nil || !reflect.DeepEqual(actual, want) {
			t.Fatal("Gemini cycle frozen authority changed")
		}
	}
	for path := range c.absent {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("Gemini cycle held absence changed")
		}
	}
	if !c.openEvents {
		body, err := c.eventLog()
		if err != nil || !bytes.Equal(body, c.eventsAccepted) {
			if err == nil {
				c.f.evidence["cycle_unassigned_probe_events"] = map[string]any{"bytes": len(body), "sha256": geminiCycleSHA(body)}
			}
			t.Fatal("Gemini probe events changed outside selected native observation")
		}
	}
	if c.f.ctx.Err() != nil {
		t.Fatal("Gemini cycle original work deadline")
	}
}
func (c *geminiProjectCycle) prepare(t *testing.T) {
	t.Helper()
	f := c.f
	projectNativeRuntimeWrite(t, f.base.Events, []byte{}, 0600)
	for _, scope := range []string{"user", "a", "b"} {
		project := f.outside
		if scope == "a" {
			project = f.base.Project
		}
		if scope == "b" {
			project = f.other
		}
		r := &geminiCycleRecipe{Scope: scope, Project: project, Source: filepath.Join(f.base.Root, "cycle-source-"+scope), Name: "uapgemini" + map[string]string{"user": "u", "a": "a", "b": "b"}[scope], Revision: map[string]string{"user": "U-one", "a": "A-one", "b": "B-one"}[scope], Nonce: geminiCycleSHA([]byte(f.base.Nonce + scope))[:32]}
		r.StateRoot = filepath.Join(project, ".uap")
		r.Profile = filepath.Join(project, ".gemini")
		if scope == "user" {
			r.StateRoot = filepath.Join(f.base.Root, "installer-state")
			r.Profile = filepath.Join(f.base.Home, ".gemini")
		}
		r.Config = filepath.Join(r.Profile, "settings.json")
		r.SkillPath = filepath.Join(r.Profile, "skills", r.Name, "SKILL.md")
		c.recipes[scope] = r
		c.author(t, r, false)
		c.seed(t, r)
	}
	trust, _ := json.Marshal(map[string]string{f.base.Project: "TRUST_FOLDER", f.other: "TRUST_FOLDER", f.outside: "TRUST_FOLDER"})
	policy := []byte("[[rule]]\ntoolName = \"*\"\ndecision = \"deny\"\npriority = 100\n")
	for _, scope := range []string{"user", "a", "b"} {
		policy = append(policy, []byte(fmt.Sprintf("\n[[rule]]\nmcpName = %q\ntoolName = \"inspect_runtime\"\ndecision = \"allow\"\npriority = 200\n", c.recipes[scope].Name))...)
	}
	for path, body := range map[string][]byte{filepath.Join(f.base.Home, ".gemini", "trustedFolders.json"): trust, filepath.Join(f.base.Home, ".gemini", "policies", "uap-project-proof.toml"): policy} {
		projectNativeRuntimeWrite(t, path, append(body, '\n'), 0600)
		f.protected[path] = nativeSHA(t, path)
	}
	for _, r := range c.recipes {
		c.freeze(t, r.Project)
	}
	for path := range f.protected {
		frame, err := geminiCycleFile(f.ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		c.fileFrames[path] = frame
	}
	c.check(t)
}
func (c *geminiProjectCycle) seed(t *testing.T, r *geminiCycleRecipe) {
	t.Helper()
	foreign := "foreign-" + r.Scope
	disabled := []any{"foreign-user", "foreign-a", "foreign-b"}
	doc := map[string]any{"mcpServers": map[string]any{foreign: map[string]any{"httpUrl": "http://127.0.0.1:9/" + foreign}}, "mcp": map[string]any{"excluded": disabled}, "skills": map[string]any{"disabled": disabled}}
	if r.Scope == "user" {
		doc["security"] = map[string]any{"auth": map[string]any{"selectedType": "gemini-api-key"}, "folderTrust": map[string]any{"enabled": true}}
		doc["privacy"] = map[string]any{"usageStatisticsEnabled": false}
		doc["telemetry"] = map[string]any{"enabled": false}
		doc["experimental"] = map[string]any{"adk": map[string]any{"agentSessionNoninteractiveEnabled": false}}
		doc["general"] = map[string]any{"enableAutoUpdate": false, "enableAutoUpdateNotification": false}
	}
	c.settings[r.Scope] = doc
	body, _ := json.Marshal(doc)
	projectNativeRuntimeWrite(t, r.Config, append(body, '\n'), 0600)
	c.f.protected[r.Config] = nativeSHA(t, r.Config)
	path := filepath.Join(r.Profile, "skills", foreign, "SKILL.md")
	projectNativeRuntimeWrite(t, path, geminiCycleForeignSkill(r.Scope), 0600)
	c.f.protected[path] = nativeSHA(t, path)
	projectNativeRuntimeWrite(t, filepath.Join(r.StateRoot, "plugin-data", "caller-owned", "keep.txt"), []byte("CALLER_"+r.Scope+"\n"), 0600)
	c.freeze(t, r.StateRoot)
	c.freeze(t, filepath.Join(r.Profile, "skills"))
}
func geminiCycleForeignSkill(scope string) []byte {
	return []byte("---\nname: foreign-" + scope + "\ndescription: Foreign inert marker\n---\nFOREIGN_" + scope + "\n")
}
