//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const clineCycleName = "uap-cline-cycle"

var clineCyclePublic = []string{"git-a", "git-b", "version", "add-user", "add-a", "add-b", "repeat-add-a", "unchanged-update-a", "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a", "remove-b", "remove-user"}
var clineCycleListings = []string{"empty-a", "empty-b", "installed-user", "installed-a", "installed-b", "stable-a", "updated-a", "repaired-a", "removed-a", "remaining-b", "removed-b", "removed-user", "final-a", "final-b"}
var clineCycleACP = []string{"empty-a", "empty-b", "installed-a", "installed-b", "updated-a", "repaired-a", "removed-a", "remaining-b", "final-a", "final-b"}
var clineCycleAggregates = []string{"cycle-native-acp.jsonl", "cycle-native-commands.jsonl", "cycle-discovery.jsonl", "cycle-provider-requests.jsonl", "cycle-provider-responses.jsonl", "cycle-provider-receipts.jsonl", "cycle-probe-events.jsonl", "cycle-proof.jsonl"}

type clineCycleRecipe struct {
	Scope, Project, Source, Skill, Revision, Nonce, Digest string
	Authored                                               map[string][]byte
	Installation                                           domain.Installation
	Binding                                                domain.ClientBinding
	Active, Data, SkillPath, Config                        string
	Removed                                                bool
}
type clineProjectCycle struct {
	f                    *clineProjectAttempt
	owner                *clineCycleProvider
	recipes              map[string]*clineCycleRecipe
	aggregates           map[string][]byte
	listings, sessions   []string
	sessionIDs, probeIDs map[string]bool
}
type clineCycleBytes struct {
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
	Body   []byte `json:"body"`
}

func (c *clineProjectCycle) stream(t *testing.T, raw []byte) clineCycleBytes {
	raw = bytes.ReplaceAll(raw, []byte(c.f.key), []byte("[REDACTED_FIXTURE_KEY]"))
	if clineCyclePrivateCredential(raw) {
		t.Fatal("Cline cycle private credential refused before encoding")
	}
	return clineCycleBytes{Bytes: len(raw), SHA256: clineAttemptSHA(raw), Body: append([]byte{}, raw...)}
}
func (c *clineProjectCycle) append(t *testing.T, member, stage string, value any) {
	t.Helper()
	known := false
	for _, name := range clineCycleAggregates {
		known = known || name == member
	}
	if !known || stage == "" {
		t.Fatal("unregistered Cline cycle aggregate")
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
	if len(c.aggregates[member])+len(raw) > 1<<20 || total > 12<<20 {
		t.Fatal("Cline cycle live aggregate bound")
	}
	c.aggregates[member] = append(c.aggregates[member], raw...)
}
func newClineProjectCycle(t *testing.T, f *clineProjectAttempt) *clineProjectCycle {
	t.Helper()
	f.cycle = true
	f.absent = map[string]bool{}
	f.evidence["capture_mode"] = "cline-complete-cycle"
	f.evidence["cycle_capture_review_required"] = false
	c := &clineProjectCycle{f: f, recipes: map[string]*clineCycleRecipe{}, aggregates: map[string][]byte{}, sessionIDs: map[string]bool{}, probeIDs: map[string]bool{}, listings: []string{}, sessions: []string{}}
	return c
}
func (c *clineProjectCycle) prepare(t *testing.T) {
	t.Helper()
	f := c.f
	for _, scope := range []string{"user", "a", "b"} {
		project := f.outside
		switch scope {
		case "a":
			project = f.base.Project
		case "b":
			project = f.other
		}
		r := &clineCycleRecipe{Scope: scope, Project: project, Source: filepath.Join(f.base.Root, "cycle-source-"+scope), Skill: "uap-cline-" + scope, Revision: scope + "-one", Nonce: clineAttemptSHA([]byte(f.base.Nonce + scope))[:32]}
		c.recipes[scope] = r
		c.author(t, r, false)
		c.seed(t, r)
	}
}
func (c *clineProjectCycle) author(t *testing.T, r *clineCycleRecipe, replace bool) {
	t.Helper()
	manifest, _ := json.Marshal(map[string]any{"$schema": domain.PluginSchemaV1, "name": clineCycleName, "version": "1.0.0", "description": "Disposable Cline cycle"})
	skill := []byte(fmt.Sprintf("---\nname: %s\ndescription: %s\n---\nUAP_CLINE_%s_%s\n", r.Skill, r.Revision, r.Revision, r.Nonce))
	authored := map[string][]byte{"plugin.json": manifest, "skills/" + r.Skill + "/SKILL.md": skill}
	if r.Scope != "user" {
		probe, err := projectNativeRuntimeRead(c.f.base.Probe, 32<<20)
		if err != nil {
			t.Fatal(err)
		}
		mcp, _ := json.Marshal(map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{"uapclinea": map[string]any{"type": "stdio", "command": "./bin/probe", "args": []string{"Owned Cline " + r.Scope, "${PLUGIN_ROOT}/argument"}, "cwd": "${PLUGIN_ROOT}", "env": map[string]string{"UAP_TEST_ROOT": c.f.base.Root, "UAP_TEST_EVENTS": c.f.base.Events, "UAP_TEST_NONCE": r.Nonce, "UAP_TEST_REVISION": r.Revision}}}})
		authored["mcp.json"], authored["bin/probe"] = mcp, probe
	}
	for path, body := range authored {
		mode := os.FileMode(0600)
		if path == "bin/probe" {
			mode = 0700
		}
		target := filepath.Join(r.Source, filepath.FromSlash(path))
		if replace {
			if _, err := projectNativeRuntimeRead(target, 64<<20); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, body, mode); err != nil {
				t.Fatal(err)
			}
		} else {
			projectNativeRuntimeWrite(t, target, body, mode)
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
	c.f.trees[r.Source] = tree
	c.append(t, "cycle-proof.jsonl", "source-"+r.Scope+"-"+r.Revision, map[string]any{"source": r.Source, "digest": r.Digest, "files": tree})
}
func (c *clineProjectCycle) seed(t *testing.T, r *clineCycleRecipe) {
	t.Helper()
	root := r.Project
	if r.Scope == "user" {
		root = c.f.base.Home
	}
	r.SkillPath = filepath.Join(root, ".cline", "skills", r.Skill, "SKILL.md")
	r.Config = filepath.Join(root, ".uap", "clients", "cline", "mcp.json")
	if r.Scope == "user" {
		r.Config = filepath.Join(root, ".cline", "data", "settings", "cline_mcp_settings.json")
	}
	foreign := "foreign-" + r.Scope
	body, _ := json.Marshal(map[string]any{"foreignPreference": "retain-" + r.Scope, "mcpServers": map[string]any{foreign: map[string]any{"transport": map[string]any{"type": "streamableHttp", "url": "http://127.0.0.1:9/" + foreign}, "disabled": true}}})
	projectNativeRuntimeWrite(t, r.Config, body, 0600)
	path := filepath.Join(root, ".cline", "skills", foreign, "SKILL.md")
	projectNativeRuntimeWrite(t, path, []byte("---\nname: "+foreign+"\ndescription: Foreign inert marker\n---\nFOREIGN_"+r.Scope+"\n"), 0600)
	c.f.protected[path] = nativeSHA(t, path)
	c.f.protected[r.Config] = nativeSHA(t, r.Config)
	caller := filepath.Join(filepath.Dir(c.statePath(r)), "plugin-data", "caller-owned")
	projectNativeRuntimeWrite(t, filepath.Join(caller, "keep.txt"), []byte("CALLER_"+r.Scope+"\n"), 0600)
	tree, err := clineAttemptTree(caller)
	if err != nil {
		t.Fatal(err)
	}
	c.f.trees[caller] = tree
	c.skillInventory(t, r)
}
func (c *clineProjectCycle) statePath(r *clineCycleRecipe) string {
	if r.Scope == "user" {
		return filepath.Join(c.f.base.Root, "installer-state", "state-v2.json")
	}
	return filepath.Join(r.Project, ".uap", "state-v2.json")
}
func (c *clineProjectCycle) command(t *testing.T, label, scope string) ports.CommandResult {
	t.Helper()
	f, r := c.f, c.recipes[scope]
	if r == nil || clineCycleScope(label) != scope {
		t.Fatal("Cline public stage scope differs from fixed table")
	}
	index := len(f.used)
	if index >= len(clineCyclePublic) || clineCyclePublic[index] != label || f.used[label] {
		t.Fatal("Cline public stage order")
	}
	cwd := r.Project
	var argv []string
	switch label {
	case "git-a", "git-b":
		argv = []string{f.git, "-c", "core.hooksPath=/dev/null", "init", "--quiet"}
	case "version":
		cwd, argv = f.outside, []string{f.client, "--auto-approve", "false", "--version"}
	default:
		verb := strings.Split(label, "-")[0]
		if label == "repeat-add-a" {
			verb = "add"
		}
		if label == "repeat-remove-a" {
			verb = "remove"
		}
		if label == "unchanged-update-a" {
			verb = "update"
		}
		if label == "intact-repair-a" || label == "missing-repair-a" {
			verb = "repair"
		}
		selector := clineCycleName
		if verb == "add" || label == "update-a" {
			selector = r.Source
		}
		argv = []string{f.installer, verb, selector, "--scope", scopeName(r), "--target", "cline", "--format", "json"}
		if r.Scope != "user" {
			argv = append(argv, "--project-root", r.Project)
		}
		if verb == "add" {
			argv = append(argv, "--no-save")
		}
	}
	f.check(t)
	f.used[label] = true
	ctx, cancel := context.WithTimeout(f.ctx, 30*time.Second)
	defer cancel()
	result, err := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: argv, Env: f.env(false), Dir: cwd, StdoutLimitBytes: 1 << 20}, 5*time.Second)
	kind := "none"
	if err != nil {
		kind = "capture"
		if reflect.TypeOf(err) == reflect.TypeFor[*exec.ExitError]() {
			kind = "exit"
		}
	}
	meta, _ := json.Marshal(map[string]any{"argv": argv, "cwd": cwd, "exit": result.ExitCode, "error": err != nil, "error_kind": kind, "deadline": ctx.Err() != nil})
	f.keep(t, label+".command.json", meta)
	f.keep(t, label+".stdout.log", result.Stdout)
	f.keep(t, label+".stderr.log", result.Stderr)
	f.check(t)
	if ctx.Err() != nil || len(result.Stdout) > 1<<20 || bytes.Contains(result.Stderr, []byte("stderr bytes omitted")) {
		t.Fatal("Cline cycle public capture/cleanup")
	}
	if label == "repeat-remove-a" {
		if !clineCycleAbsentRemoval(result, err) {
			t.Fatal("Cline repeat-removal exact refusal differs")
		}
		return result
	}
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("Cline cycle %s process failed", label)
	}
	return result
}
func clineCycleScope(stage string) string {
	if stage == "version" || strings.HasSuffix(stage, "-user") {
		return "user"
	}
	if strings.HasSuffix(stage, "-a") {
		return "a"
	}
	if strings.HasSuffix(stage, "-b") {
		return "b"
	}
	return ""
}
func clineCycleAbsentRemoval(result ports.CommandResult, err error) bool {
	return err == nil && result.ExitCode == 1 && len(result.Stdout) == 0 && string(result.Stderr) == "agentplugins: plugin is not installed for target \"cline\" in project scope; no target was changed\n"
}
func scopeName(r *clineCycleRecipe) string {
	if r.Scope == "user" {
		return "user"
	}
	return "project"
}
func (c *clineProjectCycle) finish(t *testing.T) {
	t.Helper()
	defer c.f.finish(t)
	if c.owner != nil {
		c.owner.close(t, c)
	}
	for _, member := range clineCycleAggregates {
		c.f.keep(t, member, c.aggregates[member])
	}
	c.f.evidence["cycle_public_stages"] = clineCyclePrefix(clineCyclePublic, c.f.used)
	c.f.evidence["cycle_listing_stages"], c.f.evidence["cycle_acp_stages"] = c.listings, c.sessions
	c.f.check(t)
	for _, raw := range c.f.files {
		if clineCyclePrivateCredential(raw) {
			t.Fatal("Cline cycle final private canary refusal")
		}
	}
}
func clineCyclePrefix(order []string, used map[string]bool) []string {
	result := []string{}
	for _, name := range order {
		if used[name] {
			result = append(result, name)
		}
	}
	return result
}
