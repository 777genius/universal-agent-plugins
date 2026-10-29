//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type windComponentRecipe struct {
	Scope, Project, Source, Revision, Nonce, StateRoot, Profile, Config, Active, Data string
	Authored                                                                          map[string][]byte
	Digest                                                                            string
	Binding                                                                           domain.ClientBinding
	Installation                                                                      domain.Installation
	proven                                                                            map[string]map[string]windProfileRecord
}

func (r *windComponentRecipe) scopeName() string {
	if r.Scope == "user" {
		return "user"
	}
	return "project"
}

type windComponentFixture struct {
	f         *projectNativeCLIAttempt
	recipes   map[string]*windComponentRecipe
	trees     map[string]map[string]windProfileRecord
	completed []string
	proofs    []map[string]any
	stage     string
}

func newWindComponentFixture(f *projectNativeCLIAttempt) *windComponentFixture {
	f.evidence["capture_mode"] = windComponentMode
	f.evidence["capture_review_required"] = true
	f.evidence["component_layout"] = map[string]any{"root": f.base.Root, "home": f.base.Home, "a": f.base.Project, "b": f.other, "outside": f.outside, "installer": f.installer, "client": f.client, "git": f.git}
	return &windComponentFixture{f: f, recipes: map[string]*windComponentRecipe{}, trees: map[string]map[string]windProfileRecord{}, completed: []string{}, proofs: []map[string]any{}, stage: "prepare"}
}
func windOwnedTree(ctx context.Context, root string) (result map[string]windProfileRecord, err error) {
	edges, err := windOpenDirectory(root)
	if err != nil {
		return nil, fmt.Errorf("owned tree root")
	}
	defer func() {
		if windCloseEdges(edges) != nil {
			result = nil
			err = fmt.Errorf("owned tree close")
		}
	}()
	w := &windFrameWalk{ctx: ctx, records: map[string]windProfileRecord{}}
	if err = w.directory(edges[len(edges)-1].file, "tree\x00.", 0); err != nil {
		return nil, err
	}
	if windCheckEdges(edges) != nil || ctx.Err() != nil {
		return nil, fmt.Errorf("owned tree changed/deadline")
	}
	result = map[string]windProfileRecord{}
	for key, record := range w.records {
		result[strings.TrimPrefix(key, "tree\x00")] = record
	}
	return result, nil
}
func (w *windComponentFixture) freeze(t *testing.T, path string) {
	t.Helper()
	tree, err := windOwnedTree(w.f.ctx, path)
	if err != nil {
		t.Fatal("Windsurf owned tree refusal", err)
	}
	w.trees[path] = tree
}
func (w *windComponentFixture) check(t *testing.T) {
	t.Helper()
	w.f.check(t)
	if err := windFrozenTrees(w.f.ctx, w.trees); err != nil {
		t.Fatal(err)
	}
	if err := w.events(); err != nil {
		t.Fatal("Windsurf bounded intrinsic probe events refused")
	}
}
func windFrozenTrees(ctx context.Context, trees map[string]map[string]windProfileRecord) error {
	paths := make([]string, 0, len(trees))
	for path := range trees {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		want := trees[path]
		actual, err := windOwnedTree(ctx, path)
		if err != nil {
			return fmt.Errorf("Windsurf frozen source/state/native/data/cwd tree changed: root_role=owned-tree root_sha256=%s inventory=refused", windsurfConfigDigest([]byte(path)))
		}
		if !reflect.DeepEqual(actual, want) {
			return windFrozenTreeRefusal(path, want, actual)
		}
	}
	return nil
}

// Failure-only facts from the exact comparison frame. Unknown names and file
// contents never enter the fatal output, and no observed record becomes trusted.
func windFrozenTreeRefusal(path string, want, actual map[string]windProfileRecord) error {
	keys := map[string]bool{}
	for key := range want {
		keys[key] = true
	}
	for key := range actual {
		keys[key] = true
	}
	changed, files := []string{}, []string{}
	for key := range keys {
		old, before := want[key]
		now, after := actual[key]
		if before == after && old == now {
			continue
		}
		changed = append(changed, key)
		if old.Kind == "file" || now.Kind == "file" {
			files = append(files, key)
		}
	}
	if len(changed) == 0 {
		return fmt.Errorf("Windsurf frozen source/state/native/data/cwd tree changed: diagnostic=refused")
	}
	sort.Strings(changed)
	sort.Strings(files)
	key := changed[0]
	if len(files) != 0 {
		key = files[0]
	}
	old, before := want[key]
	now, after := actual[key]
	operation := "changed"
	if !before {
		operation = "added"
	} else if !after {
		operation = "missing"
	}
	oldValue, oldOK := windProjectRecord(old, before)
	nowValue, nowOK := windProjectRecord(now, after)
	if !oldOK || !nowOK {
		return fmt.Errorf("Windsurf frozen source/state/native/data/cwd tree changed: diagnostic=refused")
	}
	message := fmt.Sprintf("Windsurf frozen source/state/native/data/cwd tree changed: root_role=owned-tree root_sha256=%s key_sha256=%s operation=%s remaining_in_root=%d before=%t after=%t before_record_sha256=%s after_record_sha256=%s equal(kind=%t mode=%t device=%t inode=%t uid=%t gid=%t links=%t size=%t modified=%t changed=%t sha256=%t)",
		windsurfConfigDigest([]byte(path)), windsurfConfigDigest([]byte(key)), operation, len(changed)-1, before, after, oldValue.RecordSHA256, nowValue.RecordSHA256,
		old.Kind == now.Kind, old.Mode == now.Mode, old.Device == now.Device, old.Inode == now.Inode, old.UID == now.UID, old.GID == now.GID, old.Links == now.Links, old.Size == now.Size, old.Modified == now.Modified, old.Changed == now.Changed, old.SHA256 == now.SHA256)
	if len(message) > 2<<10 {
		return fmt.Errorf("Windsurf frozen source/state/native/data/cwd tree changed: diagnostic=bound")
	}
	return fmt.Errorf("%s", message)
}
func (w *windComponentFixture) prepare(t *testing.T) {
	t.Helper()
	f := w.f
	w.prepareRuntime(t)
	projectNativeRuntimeWrite(t, f.base.Events, []byte{}, 0600)
	for _, scope := range []string{"user", "a", "b"} {
		r := &windComponentRecipe{Scope: scope, Project: f.outside, Source: filepath.Join(f.base.Root, "component-source-"+scope), Revision: scope + "-one", Nonce: windsurfConfigDigest([]byte(f.base.Nonce + scope))[:32]}
		r.StateRoot = filepath.Join(f.base.Root, "installer-state")
		r.Profile = filepath.Join(f.base.Home, ".codeium", "windsurf")
		if scope != "user" {
			r.Project = f.base.Project
			if scope == "b" {
				r.Project = f.other
			}
			r.StateRoot = filepath.Join(r.Project, ".uap")
			r.Profile = filepath.Join(r.Project, ".devin")
		}
		r.Config = filepath.Join(r.Profile, "mcp_config.json")
		w.recipes[scope] = r
		w.author(t, r)
		w.seed(t, r)
	}
	for _, path := range []string{f.base.Project, f.other, f.outside} {
		w.freeze(t, path)
	}
}
func (w *windComponentFixture) author(t *testing.T, r *windComponentRecipe) {
	t.Helper()
	f := w.f
	probe, err := projectNativeRuntimeRead(f.base.Probe, 32<<20)
	if err != nil {
		t.Fatal(err)
	}
	manifest, _ := json.Marshal(map[string]any{"$schema": domain.PluginSchemaV1, "name": windComponentName, "version": "1.0.0", "description": "Disposable Windsurf component observation"})
	skill := []byte("---\nname: " + windComponentName + "\ndescription: " + r.Revision + "\n---\nUAP_WINDSURF_" + r.Revision + "_" + r.Nonce + "\n")
	env := map[string]string{"UAP_TEST_ROOT": f.base.Root, "UAP_TEST_EVENTS": f.base.Events, "UAP_TEST_NONCE": r.Nonce, "UAP_TEST_REVISION": r.Revision}
	mcp, _ := json.Marshal(map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{windComponentName: map[string]any{"type": "stdio", "command": "./bin/probe", "args": []string{"Owned Windsurf " + r.Scope, "${PLUGIN_ROOT}/argument"}, "cwd": "${PLUGIN_ROOT}", "env": env}}})
	r.Authored = map[string][]byte{"plugin.json": manifest, "skills/" + windComponentName + "/SKILL.md": skill, "mcp.json": mcp, "bin/probe": probe}
	captured := make([]packagedigest.CapturedEntry, 0, len(r.Authored)*3)
	directories := map[string]bool{}
	for name, body := range r.Authored {
		mode := os.FileMode(0600)
		if name == "bin/probe" {
			mode = 0700
		}
		projectNativeRuntimeWrite(t, filepath.Join(r.Source, filepath.FromSlash(name)), body, mode)
		captured = append(captured, packagedigest.CapturedEntry{Path: name, Kind: "file", Executable: mode&0111 != 0, Content: bytes.Clone(body)})
		for dir := filepath.ToSlash(filepath.Dir(name)); dir != "."; dir = filepath.ToSlash(filepath.Dir(dir)) {
			directories[dir] = true
		}
	}
	for dir := range directories {
		captured = append(captured, packagedigest.CapturedEntry{Path: dir, Kind: "directory"})
	}
	r.Digest, err = packagedigest.DigestCaptured(f.ctx, captured)
	if err != nil {
		t.Fatal(err)
	}
	w.freeze(t, r.Source)
}
func windForeign(scope string) map[string]any {
	if scope == "user" {
		return map[string]any{"serverUrl": "http://127.0.0.1:9/foreign-user", "disabled": true}
	}
	return map[string]any{"url": "http://127.0.0.1:9/foreign-" + scope, "transport": "http", "disabled": true}
}
func (w *windComponentFixture) seed(t *testing.T, r *windComponentRecipe) {
	t.Helper()
	doc := map[string]any{"mcpServers": map[string]any{"foreign-" + r.Scope: windForeign(r.Scope)}}
	body, _ := json.Marshal(doc)
	projectNativeRuntimeWrite(t, r.Config, append(body, '\n'), 0600)
	projectNativeRuntimeWrite(t, r.Config+".agentplugins.lock", []byte{}, 0600)
	w.f.protected[r.Config+".agentplugins.lock"] = windsurfConfigDigest([]byte{})
	skill := filepath.Join(r.Profile, "skills", "foreign-"+r.Scope, "SKILL.md")
	projectNativeRuntimeWrite(t, skill, []byte("---\nname: foreign-"+r.Scope+"\ndescription: Harmless foreign marker\n---\nFOREIGN_"+r.Scope+"\n"), 0600)
	w.f.protected[skill] = nativeSHA(t, skill)
	w.f.protected[r.Config] = nativeSHA(t, r.Config)
	caller := filepath.Join(r.StateRoot, "plugin-data", "caller-owned", "keep.txt")
	projectNativeRuntimeWrite(t, caller, []byte("CALLER_"+r.Scope+"\n"), 0600)
	w.f.protected[caller] = nativeSHA(t, caller)
	w.freeze(t, r.StateRoot)
	w.freeze(t, r.Profile)
}
func (w *windComponentFixture) events() error {
	body, err := projectNativeRuntimeRead(w.f.base.Events, 1<<20)
	if err != nil || (len(body) > 0 && body[len(body)-1] != '\n') {
		return fmt.Errorf("probe event read/framing")
	}
	lines := bytes.Split(bytes.TrimSuffix(body, []byte{'\n'}), []byte{'\n'})
	if len(body) == 0 {
		return nil
	}
	if len(lines) > 128 {
		return fmt.Errorf("probe event bound")
	}
	for _, line := range lines {
		value, err := projectNativeRuntimeJSON(line)
		row, ok := value.(map[string]any)
		if err != nil || !ok || projectNativeRuntimeKeys(row, "method pid session nonce timestamp facts", "") != nil || (row["method"] != "initialize" && row["method"] != "tools/list") || row["facts"] != nil {
			return fmt.Errorf("probe event method/shape")
		}
		nonce, ok := row["nonce"].(string)
		owned := false
		for _, r := range w.recipes {
			owned = owned || (r.Active != "" && nonce == r.Nonce)
		}
		if !ok || !owned {
			return fmt.Errorf("probe event source")
		}
	}
	return nil
}
func (w *windComponentFixture) finish(t *testing.T) {
	t.Helper()
	defer w.f.finish(t)
	w.f.evidence["completed_labels"] = append([]string{}, w.completed...)
	w.f.evidence["component_proof_count"] = len(w.proofs)
	w.f.evidence["attempt_stage"] = w.stage
	if len(w.proofs) > 0 {
		var body []byte
		for _, proof := range w.proofs {
			raw, err := json.Marshal(proof)
			if err != nil {
				t.Error("component proof serialization")
				break
			}
			body = append(append(body, raw...), '\n')
		}
		w.f.keep(t, "component-proof.jsonl", body)
	}
}
func windsurfComponentFirstCapture(t *testing.T) {
	t.Helper()
	spec, err := projectNativeCLISpecification("windsurf")
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv(spec.optin) != "1" {
		t.Skip("opt-in genuine installed Windsurf component research")
	}
	f := newProjectNativeCLIAttempt(t, spec)
	w := newWindComponentFixture(f)
	defer w.finish(t)
	w.prepare(t)
	for _, label := range windComponentLabels {
		w.command(t, label)
	}
	w.stage = "capture-review"
	t.Fatal("genuine Windsurf component sequence captured; source-bound raw grammar/profile review required; all qualifications remain false")
}
